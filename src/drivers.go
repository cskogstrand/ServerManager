package main

import "sort"

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
	inst.mu.Unlock()
	inst.publishDrivers()
}

func (inst *Instance) driverLeave(carId int) {
	inst.mu.Lock()
	delete(inst.drivers, carId)
	inst.mu.Unlock()
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
	}
	inst.mu.Unlock()
	inst.publishDrivers()
}

func (inst *Instance) clearDrivers() {
	inst.mu.Lock()
	inst.drivers = make(map[int]*DriverState)
	inst.mu.Unlock()
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
