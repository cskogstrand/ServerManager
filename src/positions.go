package main

import (
	"sort"
	"time"
)

type CarPositionState struct {
	CarId               int     `json:"car_id"`
	X                   float32 `json:"x"`
	Y                   float32 `json:"y"`
	Z                   float32 `json:"z"`
	VelocityX           float32 `json:"velocity_x"`
	VelocityY           float32 `json:"velocity_y"`
	VelocityZ           float32 `json:"velocity_z"`
	Gear                int     `json:"gear"`
	EngineRpm           int     `json:"engine_rpm"`
	NormalizedSplinePos float32 `json:"normalized_spline_pos"`
	UpdatedAt           int64   `json:"updated_at"`
}

func positionStateFromUpdate(cu CarUpdate) CarPositionState {
	return CarPositionState{
		CarId:               cu.carId,
		X:                   cu.position.x,
		Y:                   cu.position.y,
		Z:                   cu.position.z,
		VelocityX:           cu.velocity.x,
		VelocityY:           cu.velocity.y,
		VelocityZ:           cu.velocity.z,
		Gear:                cu.gear,
		EngineRpm:           cu.engineRpm,
		NormalizedSplinePos: cu.normalizedSplinePos,
		UpdatedAt:           time.Now().UnixMilli(),
	}
}

func (inst *Instance) updateCarPosition(cu CarUpdate) {
	inst.mu.Lock()
	if inst.positions == nil {
		inst.positions = make(map[int]*CarPositionState)
	}
	pos := positionStateFromUpdate(cu)
	inst.positions[cu.carId] = &pos
	shouldPublish := time.Since(inst.lastPositionPublish) >= 200*time.Millisecond
	if shouldPublish {
		inst.lastPositionPublish = time.Now()
	}
	inst.mu.Unlock()

	if shouldPublish {
		inst.publishPositions()
	}
}

func (inst *Instance) removeCarPosition(carId int) {
	inst.mu.Lock()
	delete(inst.positions, carId)
	inst.mu.Unlock()
	inst.publishPositions()
}

func (inst *Instance) clearPositions() {
	inst.mu.Lock()
	inst.positions = make(map[int]*CarPositionState)
	inst.mu.Unlock()
	inst.publishPositions()
}

func (inst *Instance) positionsSnapshot() []CarPositionState {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	list := make([]CarPositionState, 0, len(inst.positions))
	for _, p := range inst.positions {
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CarId < list[j].CarId })
	return list
}

func (inst *Instance) publishPositions() {
	Events.Publish("positions", inst.Id(), map[string]any{"positions": inst.positionsSnapshot()})
}
