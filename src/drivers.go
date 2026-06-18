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
	// DriftLive/DriftLast/DriftBest are populated from the client-side drift
	// HUD's chat lines. DriftLive is the score of the run currently in
	// progress ([DRIFT] live=..), updated ~1x/sec and reset to 0 when the run
	// ends; DriftLast is the final score of the last completed run ([DRIFT]
	// last=..); DriftBest is the best score seen this session. All zero when
	// the drift feature is off or the driver has not drifted yet.
	DriftLive int `json:"drift_live"`
	DriftLast int `json:"drift_last"`
	DriftBest int `json:"drift_best"`

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
	inst.tel.lastDriverAt = time.Now()
	inst.mu.Unlock()
	if row != nil {
		persistFinishedSessions(inst.Id(), []dsSessionRow{*row})
	}
	inst.removeCarPosition(carId)
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
	}
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

// driverDrift records a drift report from the client-side HUD. A live report
// updates the in-progress score; an end-of-run report finalizes it (DriftLast)
// and clears the live score. best is monotonic per the script; we keep the max
// seen in case lines arrive out of order.
func (inst *Instance) driverDrift(carId int, live bool, score, best int) {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	var run *dsDriftInsert
	if d := inst.drivers[carId]; d != nil {
		if live {
			d.DriftLive = score
		} else {
			d.DriftLast = score
			d.DriftLive = 0
			// A completed run: persist it so the driver's drift history and
			// best-ever score survive the session reset.
			if d.Guid != "" && score > 0 {
				run = &dsDriftInsert{
					guid:        d.Guid,
					trackKey:    d.sessTrack,
					trackConfig: d.sessConfig,
					carKey:      d.Car,
					score:       score,
					endedAt:     now,
				}
			}
		}
		if best > d.DriftBest {
			d.DriftBest = best
		}
	}
	inst.mu.Unlock()
	if run != nil {
		if err := Dba.insertDriftRun(inst.Id(), *run); err != nil {
			log.Print("driverstats: insert drift run: ", err)
		}
	}
	inst.publishDrivers()
}

func (inst *Instance) clearDrivers() {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	// Fallback: record any session not already captured by end-of-session, so a
	// stop/track-change without a clean ACSP_END_SESSION still leaves history.
	rows := inst.collectFinishedSessionsLocked(now)
	inst.drivers = make(map[int]*DriverState)
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
