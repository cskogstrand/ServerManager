package main

import (
	"os/exec"
	"testing"
)

func newTestInstance(id int) *Instance {
	inst := &Instance{Conf: ServerInstance{Id: &id}, Udp: &UdpPlugin{online: true}}
	inst.Status.Players = 3
	inst.tel.udpOnline = true
	inst.drivers = map[int]*DriverState{1: {CarId: 1, Connected: true}}
	inst.positions = map[int]*CarPositionState{1: {CarId: 1}}
	return inst
}

func TestWaitForExitClearsState(t *testing.T) {
	inst := newTestInstance(1)

	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	inst.cmd = cmd

	inst.waitForExit(cmd) // synchronous in the test

	if inst.cmd != nil {
		t.Fatal("cmd should be cleared after exit")
	}
	if inst.Status.Players != 0 {
		t.Fatalf("players should be 0, got %d", inst.Status.Players)
	}
	if inst.Udp.online || inst.tel.udpOnline {
		t.Fatal("telemetry should be marked offline after exit")
	}
	if len(inst.driversSnapshot()) != 0 {
		t.Fatal("drivers should be cleared after exit")
	}
	if len(inst.positionsSnapshot()) != 0 {
		t.Fatal("positions should be cleared after exit")
	}
}

// When stop() (or a restart) has already swapped inst.cmd, the exiting old
// process must not clobber the new state.
func TestWaitForExitIgnoresSupersededProcess(t *testing.T) {
	inst := newTestInstance(1)

	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Simulate stop()/restart already replacing the current process handle.
	inst.cmd = nil

	inst.waitForExit(cmd)

	if inst.Status.Players != 3 {
		t.Fatalf("superseded exit must not reset state, players=%d", inst.Status.Players)
	}
}
