package main

import "testing"

func TestDriftScorerModeWeightsAndReset(t *testing.T) {
	mode := driftModeDefaults()
	resetAfter := 0.1
	mode.ResetScoreSeconds = &resetAfter
	speedWeight := 0.01
	mode.SpeedWeight = &speedWeight

	sc := newDriftScorer(mode)
	live, _, ended, _ := sc.step(5, 20, 72, 1.0/60.0, 0)
	if ended {
		t.Fatal("run ended while drifting")
	}
	if live <= 0 {
		t.Fatalf("expected weighted drift score, got %d", live)
	}

	for range 4 {
		live, _, ended, _ = sc.step(0, 2, 7.2, 0.05, 0)
	}
	if !ended {
		t.Fatal("expected slow non-drift timer to end the run")
	}
	if live != 0 {
		t.Fatalf("expected live score reset, got %d", live)
	}
}
