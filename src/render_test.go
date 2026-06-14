package main

import "testing"

func cnt(n int) *int { return &n }

func TestGridCount(t *testing.T) {
	key := "car"
	entry := func(c int) UserClassEntry { return UserClassEntry{CacheCarKey: &key, Count: cnt(c)} }

	if got := gridCount(UserClass{}); got != 0 {
		t.Fatalf("empty class should be 0, got %d", got)
	}
	// 3 + 2 + (nil/0 → 1) = 6
	got := gridCount(UserClass{Entries: []UserClassEntry{entry(3), entry(2), {CacheCarKey: &key}}})
	if got != 6 {
		t.Fatalf("expected 6, got %d", got)
	}
}

func TestEventDurationMinutes(t *testing.T) {
	on := cnt(1)

	// Nothing enabled → 0.
	if got := eventDurationMinutes(UserEvent{}); got != 0 {
		t.Fatalf("empty event should be 0, got %d", got)
	}

	// Practice 20 + qualify 15 + timed race 30 = 65; booking disabled is ignored.
	full := UserEvent{
		BookingEnabled:  cnt(0), BookingTime: cnt(99),
		PracticeEnabled: on, PracticeTime: cnt(20),
		QualifyEnabled:  on, QualifyTime: cnt(15),
		RaceEnabled:     on, RaceTime: cnt(30), RaceLaps: cnt(0),
	}
	if got := eventDurationMinutes(full); got != 65 {
		t.Fatalf("expected 65, got %d", got)
	}

	// A lap race contributes 0 minutes even with a race_time set.
	lap := UserEvent{RaceEnabled: on, RaceTime: cnt(30), RaceLaps: cnt(10)}
	if got := eventDurationMinutes(lap); got != 0 {
		t.Fatalf("lap race should be 0, got %d", got)
	}
}
