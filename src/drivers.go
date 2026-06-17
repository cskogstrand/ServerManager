package main

import (
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
	// DriftLast/DriftBest are populated from the client-side drift HUD's
	// end-of-run chat line ([DRIFT] last=.. best=..). Zero when the drift
	// feature is off or the driver has not completed a run.
	DriftLast int `json:"drift_last"`
	DriftBest int `json:"drift_best"`
}

func (inst *Instance) driverJoin(nc NewConnection) {
	inst.mu.Lock()
	if inst.drivers == nil {
		inst.drivers = make(map[int]*DriverState)
	}
	inst.drivers[nc.carId] = &DriverState{
		CarId:     nc.carId,
		Name:      nc.driverName,
		Car:       nc.carModel,
		Skin:      nc.carSkin,
		Guid:      nc.driverGuid,
		Connected: true,
	}
	inst.tel.lastDriverAt = time.Now()
	inst.mu.Unlock()
	inst.publishDrivers()
}

func (inst *Instance) driverLeave(carId int) {
	inst.mu.Lock()
	delete(inst.drivers, carId)
	inst.tel.lastDriverAt = time.Now()
	inst.mu.Unlock()
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
// roster of who is connected.
func (inst *Instance) resetDriverLaps() {
	inst.mu.Lock()
	for _, d := range inst.drivers {
		d.Laps = 0
		d.LastLapMs = 0
		d.BestLapMs = 0
		d.DriftLast = 0
		d.DriftBest = 0
	}
	inst.mu.Unlock()
	inst.publishDrivers()
}

var driftChatRe = regexp.MustCompile(`\[DRIFT\] last=(\d+) best=(\d+)`)

// parseDriftChat extracts (lastRunScore, bestScore) from the machine-readable
// line the client-side drift HUD posts to chat at the end of each run. ok is
// false for any other chat message.
func parseDriftChat(msg string) (last int, best int, ok bool) {
	m := driftChatRe.FindStringSubmatch(msg)
	if m == nil {
		return 0, 0, false
	}
	last, _ = strconv.Atoi(m[1])
	best, _ = strconv.Atoi(m[2])
	return last, best, true
}

// driverDrift records a drift run reported over chat by the client-side HUD.
// best is monotonic per the script; we keep the max seen in case lines arrive
// out of order.
func (inst *Instance) driverDrift(carId, last, best int) {
	inst.mu.Lock()
	if d := inst.drivers[carId]; d != nil {
		d.DriftLast = last
		if best > d.DriftBest {
			d.DriftBest = best
		}
	}
	inst.mu.Unlock()
	inst.publishDrivers()
}

func (inst *Instance) clearDrivers() {
	inst.mu.Lock()
	inst.drivers = make(map[int]*DriverState)
	inst.mu.Unlock()
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
