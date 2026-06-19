package main

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
)

// driftScoreLuaSource is the HUD/feeder CSP script, served (with an SM_INGEST
// line prepended) to joining clients by apiDriftLuaScript. It is also embedded
// raw into the StaticFS via embedded.go; this second embed lets us prepend the
// per-process ingest URL+token without templating the file on disk.
//
//go:embed embed/lua/driftscore.lua
var driftScoreLuaSource string

// driftIngestToken authenticates the telemetry WebSocket. It is baked into the
// script body the server hands each client (apiDriftLuaScript), so only clients
// that fetched the script from this process can stream. Regenerated each start:
// a process restart rejects already-connected clients' reconnects until they
// rejoin the AC server (which refetches the script); new joiners are unaffected.
var driftIngestToken string

func initDriftIngestToken() {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Deterministic fallback is still better than an empty token (which
		// disables the endpoint entirely).
		driftIngestToken = "sm-drift-ingest-fallback"
		return
	}
	driftIngestToken = hex.EncodeToString(b)
}

// driftScorer holds the per-car running state for server-side drift scoring. It
// is a faithful port of the vendored client HUD's scoring math (see
// embed/lua/driftscore.lua), with one deliberate change: the upstream script
// accumulates points once per rendered frame (so the score scaled with the
// player's FPS). We normalize each accumulation by dt*fpsRef so the score is
// rate-independent and fair across clients regardless of their frame rate or
// our telemetry sample rate. At a true 60 fps this is identical to upstream.
type driftScorer struct {
	totalScore    float64 // current run score; floor() is the live score
	comboProgress float64 // fractional combo; floor() is comboMeter
	comboMeter    int
	highestCombo  int
	highestScore  int // best completed run this session (drives DriftBest)
	lastScore     int
	slowFor       float64 // seconds spent slow & not sliding (run-end timer)
}

func newDriftScorer() *driftScorer {
	// Match the upstream initial state: combo starts at 1, not 0.
	return &driftScorer{comboProgress: 1, comboMeter: 1}
}

const (
	driftRequiredSpeed = 40.0 // km/h, from the upstream script
	driftFpsRef        = 60.0 // reference frame rate for accumulation normalization
)

// step folds one telemetry sample into the run. lvx is car-local lateral
// velocity (m/s), speedMs/speedKmh the car speed, dt the seconds elapsed since
// the previous sample. It returns the live score, the best (max of completed PB
// and the live run), whether a run just ended, and that ended run's final
// score.
func (s *driftScorer) step(lvx, speedMs, speedKmh, dt float64) (live, best int, runEnded bool, last int) {
	frames := dt * driftFpsRef

	sliding := lvx / math.Max(3, speedMs)
	slidingMult := math.Abs(sliding) * 10

	if speedKmh > driftRequiredSpeed && slidingMult > 1 {
		driftPoints := slidingMult * 0.05
		s.totalScore += driftPoints * float64(s.comboMeter) * frames
		s.comboProgress += speedKmh * 0.00005 * frames
		s.comboMeter = int(math.Floor(s.comboProgress))
		if s.comboMeter > s.highestCombo {
			s.highestCombo = s.comboMeter
		}
	}

	if speedKmh < driftRequiredSpeed && slidingMult < 1 {
		// Slow and not sliding: after a 2s grace period the run ends.
		if s.slowFor > 2 {
			cur := int(math.Floor(s.totalScore))
			if cur > s.highestScore {
				s.highestScore = cur
			}
			if s.totalScore > 0 {
				s.lastScore = cur
				last = cur
				runEnded = true
			}
			s.totalScore = 0
			s.comboMeter = 1
			s.comboProgress = 1
		}
		s.slowFor += dt
	} else {
		s.slowFor = 0
	}

	live = int(math.Floor(s.totalScore))
	best = s.highestScore
	if live > best {
		best = live
	}
	return live, best, runEnded, last
}

// driftFrame is one telemetry sample from a client's CSP feeder script.
type driftFrame struct {
	I   int     `json:"i"`   // session car id (ACSP carId), from car.index
	Lvx float64 `json:"lvx"` // car-local lateral velocity, m/s
	Kmh float64 `json:"kmh"` // speed, km/h
	Dt  float64 `json:"dt"`  // seconds since this client's previous sample
}

// apiTelemetryIngest is the public WebSocket endpoint each client's CSP script
// streams slip telemetry to. It is intentionally outside the JWT-authenticated
// /api group (game clients have no cookie); the per-process token gates it.
// Game clients are not browsers and send no Origin — x/net/websocket's server
// handshake does not require one, so accepting the upgrade is fine.
func apiTelemetryIngest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("instance"))
	if driftIngestToken == "" || c.Query("token") != driftIngestToken {
		log.Printf("drift ingest: rejected bad token instance=%d remote=%s", id, c.ClientIP())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	inst, err := instanceById(id)
	if err != nil {
		log.Printf("drift ingest: rejected unknown instance=%d remote=%s", id, c.ClientIP())
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		ws.MaxPayloadBytes = 512
		log.Printf("drift ingest: connected instance=%d remote=%s", id, ws.Request().RemoteAddr)
		defer log.Printf("drift ingest: disconnected instance=%d", id)
		first := true
		for {
			// A continuously-streaming client refreshes this on every sample; a
			// silent/dead one is reaped after the deadline.
			_ = ws.SetReadDeadline(time.Now().Add(30 * time.Second))
			var msg string
			if err := websocket.Message.Receive(ws, &msg); err != nil {
				return
			}
			var f driftFrame
			if json.Unmarshal([]byte(msg), &f) != nil {
				continue
			}
			// One diagnostic line per connection: did frames arrive, what carId
			// did the client report, and does it map to a connected driver? A
			// "known=false" here is the carId-mapping bug (player.index vs ACSP
			// carId).
			if first {
				first = false
				inst.mu.Lock()
				_, known := inst.drivers[f.I]
				cars := make([]int, 0, len(inst.drivers))
				for k := range inst.drivers {
					cars = append(cars, k)
				}
				inst.mu.Unlock()
				log.Printf("drift ingest: first frame instance=%d carId=%d known=%v connectedCars=%v lvx=%.2f kmh=%.1f dt=%.3f",
					id, f.I, known, cars, f.Lvx, f.Kmh, f.Dt)
			}
			inst.applyDriftTelemetry(f.I, f.Lvx, f.Kmh, f.Dt)
		}
	}).ServeHTTP(c.Writer, c.Request)
}

// apiDriftLuaScript serves the CSP HUD/feeder script with the ingest URL+token
// for this process prepended, so the script knows where to stream. The ingest
// host is taken from the request the client used to reach us, so it matches the
// reachability the existing drift feature already requires.
func apiDriftLuaScript(c *gin.Context) {
	scheme := "ws"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "wss"
	}
	ingest := fmt.Sprintf("%s://%s/telemetry/ingest?instance=%s&token=%s",
		scheme, c.Request.Host,
		url.QueryEscape(c.Query("instance")),
		url.QueryEscape(driftIngestToken))

	header := "local SM_INGEST = " + strconv.Quote(ingest) + "\n"
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, header+driftScoreLuaSource)
}
