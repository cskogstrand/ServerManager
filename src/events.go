package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// EventBroker fans out server events to SSE subscribers. Slow consumers are
// never waited on: a full subscriber channel just drops the message.
type EventBroker struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

var Events = &EventBroker{subs: make(map[chan []byte]struct{})}

func (b *EventBroker) Subscribe() chan []byte {
	ch := make(chan []byte, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// Domain event types worth persisting to the activity feed. Live-state churn
// (telemetry, players, positions, snapshots) is deliberately excluded.
var feedPersistTypes = map[string]bool{
	"server": true, "session_start": true, "session_end": true, "lap": true,
	"drift_run": true, "media": true, "recording": true,
}

// Publish broadcasts one event. instanceId 0 means "not instance-specific"
// (e.g. content jobs).
func (b *EventBroker) Publish(eventType string, instanceId int, payload any) {
	ts := time.Now().Unix()
	msg, err := json.Marshal(map[string]any{
		"type":        eventType,
		"instance_id": instanceId,
		"ts":          ts,
		"data":        payload,
	})
	if err != nil {
		return
	}

	if feedPersistTypes[eventType] {
		persistFeedEvent(eventType, instanceId, ts, payload)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

// feedEventRow is one persisted feed event, shaped to match the SSE ServerEvent
// the SPA already knows ({type, instance_id, ts, data}) so the frontend reuses
// its existing event→feed-item mapping for fetched history.
type feedEventRow struct {
	Type       string          `json:"type"`
	InstanceId int             `json:"instance_id"`
	Ts         int64           `json:"ts"`
	Data       json.RawMessage `json:"data"`
}

// persistFeedEvent stores a feed-worthy event. ponytail: synchronous insert on
// the publish path; local SQLite writes are sub-ms. Move to a buffered channel
// only if it ever shows up as UDP-handler latency.
func persistFeedEvent(eventType string, instanceId int, ts int64, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	guid := ""
	if m, ok := payload.(map[string]any); ok {
		guid, _ = m["guid"].(string)
	}
	_ = Dba.insertFeedEvent(eventType, instanceId, guid, ts, string(data))
}

// apiFeedList (GET /api/feed) returns persisted feed events newest-first with
// optional type (comma-separated), guid and instance_id filters plus limit/
// offset pagination. Shape: {events: ServerEvent[], total: int}.
func apiFeedList(c *gin.Context) {
	var types []string
	if t := strings.TrimSpace(c.Query("type")); t != "" {
		types = strings.Split(t, ",")
	}
	guid := strings.TrimSpace(c.Query("guid"))
	instanceId, _ := strconv.Atoi(c.Query("instance_id"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	events, total, err := Dba.queryFeedEvents(types, guid, instanceId, limit, offset)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"events": events, "total": total})
}

func sessionEventPayload(s SessionInfo) map[string]any {
	return map[string]any{
		"name":                  s.name,
		"type":                  s.typ,
		"index":                 s.sessionIndex,
		"current_session_index": s.currentSessionIndex,
		"session_count":         s.sessionCount,
		"track":                 s.track,
		"track_config":          s.trackConfig,
		"server_name":           s.serverName,
		"time":                  s.time,
		"laps":                  s.laps,
		"wait_time":             s.waitTime,
		"ambient_temp":          s.ambientTemp,
		"road_temp":             s.roadTemp,
		"weather_graphics":      s.weatherGraphics,
		"elapsed_ms":            s.elapsedMs,
	}
}

func (inst *Instance) publishRunning(running bool) {
	// name travels with the event so the persisted-feed history renders the same
	// "<server> started/stopped" line the live client builds, with no instance
	// lookup needed when replayed after a refresh.
	Events.Publish("server", inst.Id(), map[string]any{"running": running, "name": inst.Name()})
}

func (inst *Instance) publishPlayers() {
	st := inst.statusSnapshot()
	Events.Publish("players", inst.Id(), map[string]any{"players": st.Players})
}

func (inst *Instance) publishTelemetry() {
	Events.Publish("telemetry", inst.Id(), map[string]any{"telemetry": inst.telemetrySnapshot()})
}

// Domain events: persisted changes pushed to the SPA so any open page (driver
// detail, dashboard live feed, notifications) updates without a refresh. All
// carry the driver guid so views can match by-driver. Values are captured under
// inst.mu by the caller and passed in here, never read off live state.

func (inst *Instance) publishSessionStart(guid, name, car, track, trackConfig string, connId int64) {
	Events.Publish("session_start", inst.Id(), map[string]any{
		"guid": guid, "name": name, "car": car,
		"track": track, "track_config": trackConfig, "connection_id": connId,
	})
}

func (inst *Instance) publishLap(name string, lap dsLapInsert) {
	Events.Publish("lap", inst.Id(), map[string]any{
		"guid": lap.guid, "name": name, "lap": lap.lapNumber,
		"laptime_ms": lap.laptimeMs, "cuts": lap.cuts,
		"track": lap.trackKey, "car": lap.carKey,
	})
}

func (inst *Instance) publishDriftRun(name string, runId int64, run dsDriftInsert) {
	Events.Publish("drift_run", inst.Id(), map[string]any{
		"run_id": runId, "guid": run.guid, "name": name, "score": run.score,
		"track": run.trackKey, "track_config": run.trackConfig, "car": run.carKey,
	})
}

func sessionEndPayload(r dsSessionRow) map[string]any {
	pos, entrants := 0, 0
	if r.finishPos.Valid {
		pos = int(r.finishPos.Int64)
	}
	if r.entrants.Valid {
		entrants = int(r.entrants.Int64)
	}
	return map[string]any{
		"guid": r.guid, "name": r.name, "session_type": r.sessionType,
		"laps": r.laps, "best_lap_ms": r.bestLapMs, "drift_best": r.driftBest,
		"finish_pos": pos, "entrants": entrants,
		"track": r.trackKey, "track_config": r.trackConfig, "connection_id": r.connectionId,
	}
}

// apiServerEventsSSE streams broker events as Server-Sent Events. The old UI
// keeps polling; the SPA subscribes here instead.
func apiServerEventsSSE(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		apiError(c, http.StatusInternalServerError, "streaming_unsupported", "Streaming not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	ch := Events.Subscribe()
	defer Events.Unsubscribe(ch)

	// Initial snapshot so a client does not have to wait for the next event
	for _, inst := range Instances.All() {
		inst.refresh()
		st := inst.statusSnapshot()
		snapshot, err := json.Marshal(map[string]any{
			"type":        "snapshot",
			"instance_id": inst.Id(),
			"ts":          time.Now().Unix(),
			"data": map[string]any{
				"running":   st.Status,
				"players":   st.Players,
				"session":   sessionEventPayload(st.Session),
				"drivers":   inst.driversSnapshot(),
				"positions": inst.positionsSnapshot(),
				"telemetry": inst.telemetrySnapshot(),
			},
		})
		if err == nil {
			fmt.Fprintf(c.Writer, "data: %s\n\n", snapshot)
		}
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}
