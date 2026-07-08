package main

import (
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

func writeTrackJSON(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"name":"Test","length":"1000m","pitboxes":"12"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}
