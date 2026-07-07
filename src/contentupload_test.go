package main

import "testing"

func TestDetectArchiveContentKindFromPaths(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		want    string
		wantErr bool
	}{
		{
			name:  "direct car",
			paths: []string{"ks_car/ui/ui_car.json", "ks_car/data/car.ini"},
			want:  "car",
		},
		{
			name:  "content track",
			paths: []string{"content/tracks/ks_track/ui/ui_track.json", "content/tracks/ks_track/models.ini"},
			want:  "track",
		},
		{
			name:    "ambiguous",
			paths:   []string{"ks_car/ui/ui_car.json", "ks_track/ui/ui_track.json"},
			wantErr: true,
		},
		{
			name:    "unknown",
			paths:   []string{"readme.txt"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectArchiveContentKindFromPaths(tt.paths)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse7zListPathsSkipsArchiveHeader(t *testing.T) {
	output := `Listing archive: /tmp/uploaded-track.7z

--
Path = /tmp/uploaded-track.7z
Type = 7z
Physical Size = 1234

----------
Path = ui/ui_track.json
Size = 42

Path = models.ini
Size = 12
`

	got := parse7zListPaths(output)
	want := []string{"ui/ui_track.json", "models.ini"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	kind, err := detectArchiveContentKindFromPaths(got)
	if err != nil {
		t.Fatal(err)
	}
	if kind != "track" {
		t.Fatalf("got kind %q, want track", kind)
	}
}

func TestFriendly7zArchiveError(t *testing.T) {
	raw := `ERROR: /tmp/upload.rar : Cannot open the file as archive
Errors: 1`
	got := friendly7zArchiveError(raw, "fallback")
	want := "The archive could not be opened. It may be corrupt, incomplete, password-protected, an unsupported RAR variant, or one part of a multi-part RAR."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	if got := friendly7zArchiveError("", "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}

	raw = `7-Zip 26.00
ERROR: /tmp/upload.rar : Unknown archive error
Errors: 1`
	if got := friendly7zArchiveError(raw, "fallback"); got != "fallback" {
		t.Fatalf("got %q, want fallback", got)
	}
}
