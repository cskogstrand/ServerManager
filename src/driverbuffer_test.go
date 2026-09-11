package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// clipEncodeTimeout must always give ffmpeg at least as long as the clip itself,
// so a realtime-speed re-encode of the whole window is never SIGKILLed mid-write
// (the bug that produced moov-less, unplayable manual clips). Windows under ~10s
// keep the old fixed floor.
func TestClipEncodeTimeout(t *testing.T) {
	floor := 2 * localFfmpegTimeout // 60s
	cases := []struct {
		windowMs int64
		want     time.Duration
	}{
		{0, floor},                  // 0+30s -> floored
		{9000, floor},               // 27s+30s=57s -> floored
		{10000, floor},              // 30s+30s=60s == floor
		{14000, 72 * time.Second},   // 14s auto-clip: 42s+30s
		{120000, 390 * time.Second}, // 120s: 360s+30s
		{126000, 408 * time.Second}, // 120s footage + 6s lead/trail: 378s+30s
		{300000, 930 * time.Second}, // pathological long clip: 900s+30s
	}
	for _, c := range cases {
		got := clipEncodeTimeout(c.windowMs)
		if got != c.want {
			t.Errorf("window %dms: got %v, want %v", c.windowMs, got, c.want)
		}
		// Hard guarantee: never kill before the footage could play through in realtime.
		if got < time.Duration(c.windowMs)*time.Millisecond {
			t.Errorf("window %dms: timeout %v is shorter than the clip itself", c.windowMs, got)
		}
		if got < floor {
			t.Errorf("window %dms: timeout %v below floor %v", c.windowMs, got, floor)
		}
	}
	// The max manual clip (120s footage) must comfortably exceed the record cap,
	// not sit on the floor.
	if got := clipEncodeTimeout(126000); got <= manualMaxDuration {
		t.Errorf("manual-max clip budget %v must exceed manualMaxDuration %v", got, manualMaxDuration)
	}
}

// selectSegments picks every 2s buffer segment whose span intersects the window,
// in order, and excludes the ones that only touch the edges.
func TestSelectSegments(t *testing.T) {
	segs := []bufSeg{
		{path: "a", startMs: 0},
		{path: "b", startMs: 2000},
		{path: "c", startMs: 4000},
		{path: "d", startMs: 6000},
		{path: "e", startMs: 8000},
	}
	got := selectSegments(segs, 3000, 7000)
	want := []int64{2000, 4000, 6000} // b covers 2-4 (end>3), c 4-6, d 6-8 (start<7)
	if len(got) != len(want) {
		t.Fatalf("got %d segments, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].startMs != want[i] {
			t.Errorf("segment %d: got start %d, want %d", i, got[i].startMs, want[i])
		}
	}

	if len(selectSegments(nil, 0, 1000)) != 0 {
		t.Error("empty buffer must select nothing")
	}
}

// A long-GOP MPEG-TS segment starts at a nonzero timestamp. EOF/input seeking
// can exit successfully with no frame; exercise the real decoder and clip trim.
func TestMPEGTSFrameAndClipWindow(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "segment.ts")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=olive:s=160x90:r=25", "-t", "8", "-c:v", "libx264", "-g", "200", "-pix_fmt", "yuv420p", "-f", "mpegts", src).CombinedOutput(); err != nil {
		t.Fatalf("fixture encoding: %v %s", err, out)
	}
	manager := captureManager{ffmpeg: ffmpeg}
	jpg := filepath.Join(dir, "still.jpg")
	if !manager.grabFrameFromFile(src, 7, jpg) {
		t.Fatal("No JPEG from completed MPEG-TS segment")
	}
	image, err := os.ReadFile(jpg)
	if err != nil || len(image) < 2 || image[0] != 0xff || image[1] != 0xd8 {
		t.Fatal("Invalid JPEG", err)
	}
	list := filepath.Join(dir, "segments.txt")
	os.WriteFile(list, []byte("file '"+src+"'\n"), 0600)
	clip := filepath.Join(dir, "clip.mp4")
	if !manager.concatEncode(list, clip, 2000, 3000) {
		t.Fatal("clip encode failed")
	}
	out, err := exec.Command(probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", clip).Output()
	if err != nil {
		t.Fatal(err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration < 1.95 || duration > 2.05 {
		t.Fatalf("Clip escaped requested window: %s %v", out, err)
	}
}
