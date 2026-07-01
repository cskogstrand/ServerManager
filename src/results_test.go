package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseResultFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "race_result.json")
	body := `{
		"Type": "RACE",
		"TrackName": "Spa",
		"Cars": [
			{ "CarId": 0, "Driver": { "Name": "Ada" }, "BestLap": 121345 },
			{ "CarId": 1, "Driver": { "Name": "Bea" }, "BestLap": 122000 }
		],
		"Result": [
			{ "CarId": 1, "Position": 1 },
			{ "CarId": 0, "Position": 2 }
		]
	}`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, ok := parseResultFile(path, 2)
	if !ok {
		t.Fatal("parse failed")
	}
	if got.Type != "RACE" || got.Track != "Spa" || got.Winner != "Bea" || got.Entries != 2 || got.BestLapMs != 121345 || got.InstanceId != 2 {
		t.Fatalf("unexpected summary: %+v", got)
	}
}
