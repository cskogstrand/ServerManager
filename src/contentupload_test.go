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
