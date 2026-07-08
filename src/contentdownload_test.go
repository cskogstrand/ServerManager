package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteContentManifestIncludesFilesAndVersions(t *testing.T) {
	dir := t.TempDir()
	writeContentManifest(
		dir,
		map[string]string{
			"ks_mazda_mx5_cup": "https://example.test/ks_mazda_mx5_cup.zip",
		},
		map[string]string{
			"ks_mazda_mx5_cup": "2.4.1",
		},
		"rudskogen",
		"https://example.test/rudskogen.zip",
		"1.2.0",
	)

	data, err := os.ReadFile(filepath.Join(dir, "cfg", "cm_content", "content.json"))
	if err != nil {
		t.Fatal(err)
	}

	var got cmContent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	car := got.Cars["ks_mazda_mx5_cup"]
	if car.File != "ks_mazda_mx5_cup.zip" || car.URL != "https://example.test/ks_mazda_mx5_cup.zip" || car.Version != "2.4.1" {
		t.Fatalf("car manifest entry not populated: %+v", car)
	}
	if got.Track == nil {
		t.Fatal("missing track manifest entry")
	}
	if got.Track.File != "rudskogen.zip" || got.Track.URL != "https://example.test/rudskogen.zip" || got.Track.Version != "1.2.0" {
		t.Fatalf("track manifest entry not populated: %+v", *got.Track)
	}
}

