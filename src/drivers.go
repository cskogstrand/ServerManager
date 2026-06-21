package main

import (
	"log"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// DriverState is one connected (or recently connected) car on an instance,
// built from ACSP connection and lap-completed events. It powers the live
// timing / driver roster and kick-by-car-id.
type DriverState struct {
	CarId     int    `json:"car_id"`
	Name      string `json:"name"`
	Car       string `json:"car"`
	Skin      string `json:"skin"`
	Guid      string `json:"guid"`
	Laps      int    `json:"laps"`
	LastLapMs uint32 `json:"last_lap_ms"`
	BestLapMs uint32 `json:"best_lap_ms"`
	Connected bool   `json:"connected"`
	// DriftLive/DriftLast/DriftBest are the int snapshot of this car's drift
	// score, computed server-side in driftScorer from slip telemetry streamed by
	// the client's CSP feeder (applyDriftTelemetry; legacy chat path still feeds
	// driverDrift). DriftLive is the score of the run currently in progress,
	// updated in real time and reset to 0 when the run ends; DriftLast is the
	// final score of the last completed run; DriftBest is the best score seen
	// this session. All zero when the drift feature is off or the driver has not
	// drifted yet.
	DriftLive int `json:"drift_live"`
	DriftLast int `json:"drift_last"`
	DriftBest int `json:"drift_best"`

	// ExtraDriverId attributes this car to a roster "extra driver" (a real person
	// sharing the account GUID) so completed runs/sessions are stored under that
	// person's name in leaderboards. 0 = none (use the GUID's own name). Set live
	// by an operator and snapshotted onto each run/session as it is recorded.
	ExtraDriverId int `json:"extra_driver_id"`

	// Persistence bookkeeping (not serialized): the session context captured
	// when the driver joined or the session rolled over, used to write a
	// driver_session row when the session ends. recorded guards against writing
	// the same session twice (end-of-session vs. the disconnect that follows a
	// track-change kick). See driverstats.go.
	joinedAt   int64
	sessType   int
	sessTrack  string
	sessConfig string
	recorded   bool

	// Drift-run capture bookkeeping (not serialized). A clip is captured at the
	// END of a drift run if its peak score cleared the trigger, subject to a
	// per-session cap (captureCount) and per-driver cooldown (lastCaptureMs).
	// The run's start time, baseline and peak (value + when) define the clip
	// window and the screenshot moment. See drivercapture.go.
	driftRunBaseline int
	driftRunStartMs  int64
	driftRunPeak     int
	driftRunPeakMs   int64
	captureCount     int
	lastCaptureMs    int64
}

func (inst *Instance) driverJoin(nc NewConnection) {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	if inst.drivers == nil {
		inst.drivers = make(map[int]*DriverState)
	}
	inst.drivers[nc.carId] = &DriverState{
		CarId:      nc.carId,
		Name:       nc.driverName,
		Car:        nc.carModel,
		Skin:       nc.carSkin,
		Guid:       nc.driverGuid,
		Connected:  true,
		joinedAt:   now,
		sessType:   inst.Status.Session.typ,
		sessTrack:  inst.Status.Session.track,
		sessConfig: inst.Status.Session.trackConfig,
	}
	inst.tel.lastDriverAt = time.Now()
	inst.mu.Unlock()
	if nc.driverGuid != "" {
		if err := Dba.recordDriverSeen(nc.driverGuid, nc.driverName, now); err != nil {
			log.Print("driverstats: record driver: ", err)
		}
	}
	Captures.nudge() // a connect may arm a rolling-buffer recorder
	inst.publishDrivers()
}

func (inst *Instance) driverLeave(carId int) {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	var row *dsSessionRow
	if d := inst.drivers[carId]; d != nil {
		if !d.recorded && d.Guid != "" && (d.Laps > 0 || d.DriftBest > 0) {
			r := sessionRowFromDriverLocked(d, now)
			d.recorded = true
			row = &r
		}
	}
	delete(inst.drivers, carId)
	delete(inst.driftScorers, carId)
	inst.tel.lastDriverAt = time.Now()
	inst.mu.Unlock()
	if row != nil {
		persistFinishedSessions(inst.Id(), []dsSessionRow{*row})
	}
	inst.removeCarPosition(carId)
	Captures.nudge() // a disconnect may retire a rolling-buffer recorder
	inst.publishDrivers()
}

func (inst *Instance) driverLap(lc LapCompleted) {
	inst.mu.Lock()
	if d := inst.drivers[lc.carId]; d != nil {
		d.Laps++
		d.LastLapMs = lc.laptime
		if lc.laptime > 0 && (d.BestLapMs == 0 || lc.laptime < d.BestLapMs) {
			d.BestLapMs = lc.laptime
		}
	}
	inst.tel.lastDriverAt = time.Now()
	inst.mu.Unlock()
	inst.publishDrivers()
}

// resetDriverLaps clears lap stats on a new session without dropping the
// roster of who is connected, and re-captures the new session context for each
// driver. The previous session is persisted on ACSP_END_SESSION
// (finalizeCurrentSession), which fires before this, so we don't record here.
func (inst *Instance) resetDriverLaps() {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	newType := inst.Status.Session.typ
	newTrack := inst.Status.Session.track
	newConfig := inst.Status.Session.trackConfig
	for _, d := range inst.drivers {
		d.Laps = 0
		d.LastLapMs = 0
		d.BestLapMs = 0
		d.DriftLive = 0
		d.DriftLast = 0
		d.DriftBest = 0
		d.recorded = false
		d.joinedAt = now
		d.sessType = newType
		d.sessTrack = newTrack
		d.sessConfig = newConfig
		d.driftRunBaseline = 0
		d.driftRunStartMs = 0
		d.driftRunPeak = 0
		d.driftRunPeakMs = 0
		d.captureCount = 0
		d.lastCaptureMs = 0
	}
	inst.driftScorers = make(map[int]*driftScorer)
	inst.mu.Unlock()
	inst.publishDrivers()
}

var driftChatRe = regexp.MustCompile(`\[DRIFT\] (live|last)=(\d+) best=(\d+)`)

// parseDriftChat extracts a drift report from the machine-readable line the
// client-side drift HUD posts to chat: "live=" mid-run (~1x/sec) or "last=" at
// the end of a run. live reports the run in progress, !live the final score.
// ok is false for any other chat message.
func parseDriftChat(msg string) (live bool, score int, best int, ok bool) {
	m := driftChatRe.FindStringSubmatch(msg)
	if m == nil {
		return false, 0, 0, false
	}
	live = m[1] == "live"
	score, _ = strconv.Atoi(m[2])
	best, _ = strconv.Atoi(m[3])
	return live, score, best, true
}

// driverDrift records a drift report on the legacy chat path (clients running
// an older cached HUD that still posts "[DRIFT]" lines). It always publishes
// immediately. The current path is applyDriftTelemetry, which scores
// server-side and throttles publishing.
func (inst *Instance) driverDrift(carId int, live bool, score, best int) {
	inst.recordDrift(carId, live, score, best, true)
}

// recordDrift applies one drift report to a driver. A live report updates the
// in-progress score; an end-of-run report finalizes it (DriftLast) and clears
// the live score. best is the max of the completed PB and the live run. When
// publishNow is false the SSE push is skipped (the high-rate telemetry path
// throttles it in applyDriftTelemetry), but capture and persistence still run.
func (inst *Instance) recordDrift(carId int, live bool, score, best int, publishNow bool) {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	var run *dsDriftInsert
	var capReq *captureRequest
	var endedGuid string
	if d := inst.drivers[carId]; d != nil {
		if live {
			// A run starts when the live score first rises from zero; remember
			// the baseline, start time and (rolling) peak so the end-of-run clip
			// can span the whole run and the screenshot can land on the peak.
			if d.DriftLive == 0 && score > 0 {
				d.driftRunBaseline = score
				d.driftRunStartMs = now
				d.driftRunPeak = score
				d.driftRunPeakMs = now
			}
			d.DriftLive = score
			if score > d.driftRunPeak {
				d.driftRunPeak = score
				d.driftRunPeakMs = now
			}
		} else {
			// Run ended. Capture the whole run if its peak cleared the trigger,
			// subject to the per-session cap, cooldown and a configured source.
			if d.Guid != "" && d.driftRunStartMs > 0 &&
				Captures.shouldTrigger(d.Guid, d.driftRunPeak, d.captureCount, d.lastCaptureMs, now) {
				d.captureCount++
				d.lastCaptureMs = now
				capReq = &captureRequest{
					guid:        d.Guid,
					driverName:  d.Name,
					trackKey:    d.sessTrack,
					trackConfig: d.sessConfig,
					score:       d.driftRunPeak,
					delta:       d.driftRunPeak - d.driftRunBaseline,
					runStartMs:  d.driftRunStartMs,
					runEndMs:    now,
					peakMs:      d.driftRunPeakMs,
				}
			}
			d.DriftLast = score
			d.DriftLive = 0
			d.driftRunBaseline = 0
			d.driftRunStartMs = 0
			d.driftRunPeak = 0
			d.driftRunPeakMs = 0
			// A run just ended: stop any manual recording for this driver.
			if d.Guid != "" {
				endedGuid = d.Guid
			}
			// A completed run: persist it so the driver's drift history and
			// best-ever score survive the session reset.
			if d.Guid != "" && score > 0 {
				run = &dsDriftInsert{
					guid:          d.Guid,
					trackKey:      d.sessTrack,
					trackConfig:   d.sessConfig,
					carKey:        d.Car,
					score:         score,
					endedAt:       now,
					extraDriverId: d.ExtraDriverId,
				}
			}
		}
		if best > d.DriftBest {
			d.DriftBest = best
		}
	}
	inst.mu.Unlock()
	if run != nil {
		runId, err := Dba.insertDriftRun(inst.Id(), *run)
		if err != nil {
			log.Print("driverstats: insert drift run: ", err)
		} else if capReq != nil {
			// Same end-of-run block built both — tag the capture's media with the
			// run id so the leaderboard links score → video exactly.
			capReq.driftRunId = runId
		}
	}
	if capReq != nil {
		Captures.submit(*capReq)
	}
	if endedGuid != "" {
		Captures.onDriftRunEnd(endedGuid)
	}
	if publishNow {
		inst.publishDrivers()
	}
}

// applyDriftTelemetry folds one raw slip sample (from a client's CSP feeder)
// into that car's server-side drift score, then routes the result through the
// shared recordDrift path so capture and persistence behave exactly as before.
// Live updates publish to SSE at ~10Hz; run ends always publish immediately so
// the final score and DriftLast land without delay. It returns the authoritative
// display values (live run score, last completed run, session best, combo) so
// the caller can echo them back to the client — the HUD then shows the server's
// numbers instead of its own, keeping the two in sync. ok is false when no
// driver maps to carId (frame dropped).
func (inst *Instance) applyDriftTelemetry(carId int, lvx, kmh, dt float64) (live, last, best, combo int, ok bool) {
	// Clamp dt: a reconnect or stall can produce a huge gap that would dump a
	// burst of points in one step; an absent/zero dt falls back to the feeder's
	// nominal interval.
	if dt <= 0 || dt > 1 {
		dt = 0.05
	}

	inst.mu.Lock()
	if _, exists := inst.drivers[carId]; !exists {
		inst.mu.Unlock()
		return 0, 0, 0, 0, false
	}
	if inst.driftScorers == nil {
		inst.driftScorers = make(map[int]*driftScorer)
	}
	sc := inst.driftScorers[carId]
	if sc == nil {
		sc = newDriftScorer()
		inst.driftScorers[carId] = sc
	}
	var ended bool
	var lastRun int
	live, best, ended, lastRun = sc.step(lvx, kmh/3.6, kmh, dt)
	last = sc.lastScore
	combo = sc.comboMeter
	publishNow := ended || time.Since(inst.lastDriftPublish) >= 100*time.Millisecond
	if publishNow {
		inst.lastDriftPublish = time.Now()
	}
	inst.mu.Unlock()

	if ended {
		inst.recordDrift(carId, false, lastRun, best, true)
	} else {
		inst.recordDrift(carId, true, live, best, publishNow)
	}
	return live, last, best, combo, true
}

func (inst *Instance) clearDrivers() {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	// Fallback: record any session not already captured by end-of-session, so a
	// stop/track-change without a clean ACSP_END_SESSION still leaves history.
	rows := inst.collectFinishedSessionsLocked(now)
	inst.drivers = make(map[int]*DriverState)
	inst.driftScorers = make(map[int]*driftScorer)
	inst.mu.Unlock()
	persistFinishedSessions(inst.Id(), rows)
	inst.clearPositions()
	inst.publishDrivers()
}

func (inst *Instance) driversSnapshot() []DriverState {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	list := make([]DriverState, 0, len(inst.drivers))
	for _, d := range inst.drivers {
		list = append(list, *d)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CarId < list[j].CarId })
	return list
}

func (inst *Instance) publishDrivers() {
	Events.Publish("drivers", inst.Id(), map[string]any{"drivers": inst.driversSnapshot()})
}

// setExtraDriverForCar assigns (edid > 0) or clears (edid == 0) the extra-driver
// attribution for a connected car. Subsequent runs/sessions for that car are
// recorded under the extra driver's name. Returns false if no such car.
func (inst *Instance) setExtraDriverForCar(carId, edid int) bool {
	inst.mu.Lock()
	d := inst.drivers[carId]
	if d == nil {
		inst.mu.Unlock()
		return false
	}
	d.ExtraDriverId = edid
	inst.mu.Unlock()
	inst.publishDrivers()
	return true
}

// clearExtraDriverAssignment drops a now-deleted extra driver from any live car
// so the roster stops pointing at a missing id.
func (inst *Instance) clearExtraDriverAssignment(edid int) {
	inst.mu.Lock()
	changed := false
	for _, d := range inst.drivers {
		if d.ExtraDriverId == edid {
			d.ExtraDriverId = 0
			changed = true
		}
	}
	inst.mu.Unlock()
	if changed {
		inst.publishDrivers()
	}
}
