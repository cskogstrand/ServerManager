package main

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNextFreePort(t *testing.T) {
	if got := nextFreePort(nil, 9600); got != 9600 {
		t.Fatalf("empty should return fallback, got %d", got)
	}
	if got := nextFreePort([]int{}, 8081); got != 8081 {
		t.Fatalf("empty slice should return fallback, got %d", got)
	}
	if got := nextFreePort([]int{9600, 9601, 9603}, 9600); got != 9604 {
		t.Fatalf("should be one above max, got %d", got)
	}
	if got := nextFreePort([]int{5000, 5001}, 5000); got != 5002 {
		t.Fatalf("plugin base should be 5002, got %d", got)
	}
}

func readyMap() gin.H {
	return gin.H{
		"acserver_found": true,
		"cfg_filled":     true,
		"events":         2,
		"port_conflict":  "",
		"content":        gin.H{"tracks": 5, "cars": 10, "weathers": 3},
		"presets":        gin.H{"difficulties": 1, "sessions": 1, "classes": 1, "times": 1},
	}
}

func TestSetupBlockingIssues(t *testing.T) {
	t.Run("fully ready has no blockers and can start", func(t *testing.T) {
		blocking, canStart := setupBlockingIssues(readyMap())
		if !canStart || len(blocking) != 0 {
			t.Fatalf("expected ready, got canStart=%v blocking=%v", canStart, blocking)
		}
	})

	t.Run("fresh install blocks every gate keyed to its step", func(t *testing.T) {
		d := gin.H{
			"acserver_found": false,
			"cfg_filled":     false,
			"events":         0,
			"port_conflict":  "",
			"content":        gin.H{"tracks": 0, "cars": 0, "weathers": 0},
			"presets":        gin.H{"difficulties": 0, "sessions": 0, "classes": 0, "times": 0},
		}
		blocking, canStart := setupBlockingIssues(d)
		if canStart {
			t.Fatal("fresh install must not be startable")
		}
		steps := map[string]bool{}
		for _, b := range blocking {
			steps[b["step"].(string)] = true
		}
		for _, want := range []string{"install", "content", "server", "race"} {
			if !steps[want] {
				t.Fatalf("expected a blocker for step %q, got %v", want, steps)
			}
		}
	})

	t.Run("port conflict blocks the instance step", func(t *testing.T) {
		d := readyMap()
		d["port_conflict"] = "Port 9600 is used by both \"A\" and \"B\""
		blocking, canStart := setupBlockingIssues(d)
		if canStart {
			t.Fatal("port conflict must block start")
		}
		found := false
		for _, b := range blocking {
			if b["step"] == "instance" {
				found = true
			}
		}
		if !found {
			t.Fatal("expected an instance-step blocker for the port conflict")
		}
	})

	t.Run("no events blocks the race step even when everything else is ready", func(t *testing.T) {
		d := readyMap()
		d["events"] = 0
		blocking, canStart := setupBlockingIssues(d)
		if canStart {
			t.Fatal("no events must block start")
		}
		if len(blocking) != 1 || blocking[0]["step"] != "race" {
			t.Fatalf("expected a single race blocker, got %v", blocking)
		}
	})
}
