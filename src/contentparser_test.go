package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTrackJSONCandidatesIncludesBaseAndLayouts(t *testing.T) {
	trackPath := t.TempDir()
	writeTrackJSON(t, filepath.Join(trackPath, "ui", "ui_track.json"))
	writeTrackJSON(t, filepath.Join(trackPath, "ui", "sprint", "ui_track.json"))
	writeTrackJSON(t, filepath.Join(trackPath, "ui", "reverse", "dlc_ui_track.json"))
	writeTrackJSON(t, filepath.Join(trackPath, "gatebil", "ui", "ui_track.json"))
	if err := os.MkdirAll(filepath.Join(trackPath, "ui", "missing"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := trackJSONCandidates(trackPath)
	gotConfigs := map[string]bool{}
	for _, candidate := range got {
		gotConfigs[candidate.config] = true
	}

	for _, want := range []string{"", "gatebil", "reverse", "sprint"} {
		if !gotConfigs[want] {
			t.Fatalf("missing config %q in %v", want, gotConfigs)
		}
	}
	if len(gotConfigs) != 4 {
		t.Fatalf("got configs %v, want exactly 4 configs", gotConfigs)
	}
}

func TestParseTrackJSONKeepsVersion(t *testing.T) {
	trackPath := t.TempDir()
	jsonPath := filepath.Join(trackPath, "ui", "ui_track.json")
	writeTrackJSONWithVersion(t, jsonPath, "1.2.0")

	got, err := parseTrackJSON(jsonPath, "rudskogen", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version == nil || *got.Version != "1.2.0" {
		t.Fatalf("got version %v, want 1.2.0", got.Version)
	}
}

func TestCacheCarJSONKeepsVersion(t *testing.T) {
	var got CacheCar
	if err := json.Unmarshal([]byte(`{"name":"Test Car","version":"2.4.1"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version == nil || *got.Version != "2.4.1" {
		t.Fatalf("got version %v, want 2.4.1", got.Version)
	}
}

func writeTrackJSON(t *testing.T, path string) {
	t.Helper()
	writeTrackJSONWithVersion(t, path, "")
}

func writeTrackJSONWithVersion(t *testing.T, path string, version string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"name":"Test","length":"1000m","pitboxes":"12"}`
	if version != "" {
		data = `{"name":"Test","version":"` + version + `","length":"1000m","pitboxes":"12"}`
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
