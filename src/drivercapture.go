package main

// Drift-spike auto-capture. When a driver's live drift score crosses
// captureTriggerScore during a run, driverDrift (drivers.go) submits one
// captureRequest. This engine pulls a screenshot + short clip from the driver's
// configured raw stream (driver_stream.stream_capture_url) via ffmpeg and files
// them as driver_media so they surface in the Driver Detail highlight reel.
//
// Throttling keeps the reel curated rather than endless:
//   - one capture per drift run            (DriverState.driftRunFired)
//   - per-session cap                       (DriverState.captureCount)
//   - per-driver cooldown                   (DriverState.lastCaptureMs)
//   - global concurrency + per-minute rate  (captureManager.sem / recent)
//   - retention: keep the top-by-score + most-recent, prune the rest
//
// Capture is only available where ffmpeg is on PATH and the driver has a raw,
// server-reachable capture URL (HLS/RTMP/SRT/RTSP) — platform embeds can't be
// grabbed. Every ffmpeg failure is logged and swallowed; nothing here blocks
// the UDP event loop.

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ztrue/tracerr"
)

const (
	captureTriggerScore   = 2500 // a run must reach this score to be "big"
	maxCapturesPerSession = 12   // per driver, per session
	captureCooldownMs     = 45_000
	captureClipSeconds    = 14
	maxCaptureConcurrent  = 2 // simultaneous ffmpeg jobs
	maxCapturesPerMinute  = 8 // global ceiling
	retainTopByScore      = 12 // per driver: keep the N highest-scoring captures…
	retainRecent          = 8  // …plus the M most recent; prune the rest
	ffmpegTimeout         = 40 * time.Second
)

type captureRequest struct {
	guid        string
	driverName  string
	trackKey    string
	trackConfig string
	score       int
	delta       int
}

type captureManager struct {
	enabled bool   // ffmpeg present on PATH
	ffmpeg  string // resolved ffmpeg binary

	mu         sync.Mutex
	armedGuids map[string]bool // driver guids with a usable capture URL
	recent     []int64         // unix-ms of recent captures (global rate window)

	sem chan struct{} // concurrency limiter
}

var Captures *captureManager

func newCaptureManager() *captureManager {
	m := &captureManager{
		armedGuids: map[string]bool{},
		sem:        make(chan struct{}, maxCaptureConcurrent),
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		m.enabled = true
		m.ffmpeg = path
		log.Print("driver capture: ffmpeg found, stream highlight capture enabled")
	} else {
		log.Print("driver capture: ffmpeg not found on PATH — stream highlight capture disabled")
	}
	m.refresh()
	return m
}

// refresh reloads the set of driver guids that have a capture-source URL.
// Called at startup and after any driver-stream change.
func (m *captureManager) refresh() {
	if m == nil {
		return
	}
	guids, err := Dba.selectCaptureGuids()
	if err != nil {
		log.Print("driver capture: refresh: ", err)
		return
	}
	m.mu.Lock()
	m.armedGuids = guids
	m.mu.Unlock()
}

// armed reports whether captures should be attempted for a guid: ffmpeg is
// available and the driver has a capture URL configured.
func (m *captureManager) armed(guid string) bool {
	if m == nil || !m.enabled || guid == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.armedGuids[guid]
}

// submit runs a capture job if the global rate + concurrency budget allows,
// otherwise drops it (the per-run/per-session gates upstream keep this rare).
// Never blocks the caller.
func (m *captureManager) submit(req captureRequest) {
	if m == nil || !m.enabled {
		return
	}
	now := time.Now().UnixMilli()

	m.mu.Lock()
	cutoff := now - 60_000
	kept := m.recent[:0]
	for _, t := range m.recent {
		if t >= cutoff {
			kept = append(kept, t)
		}
	}
	m.recent = kept
	if len(m.recent) >= maxCapturesPerMinute {
		m.mu.Unlock()
		log.Printf("driver capture: per-minute ceiling hit, dropping capture for %s", req.guid)
		return
	}
	m.recent = append(m.recent, now)
	m.mu.Unlock()

	select {
	case m.sem <- struct{}{}:
		go func() {
			defer func() { <-m.sem }()
			m.run(req)
		}()
	default:
		log.Printf("driver capture: worker pool full, dropping capture for %s", req.guid)
	}
}

func (m *captureManager) run(req captureRequest) {
	captureURL := m.captureURLFor(req.guid)
	if captureURL == "" {
		return
	}
	dir := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(req.guid))
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Print("driver capture: mkdir: ", err)
		return
	}
	at := time.Now()
	stamp := strconv.FormatInt(at.UnixMilli(), 10)
	trackName := m.trackName(req.trackKey, req.trackConfig)

	// Screenshot: a single frame from the live edge.
	shot := stamp + "_shot.jpg"
	if m.grabFrame(captureURL, filepath.Join(dir, shot)) {
		caption := fmt.Sprintf("Drift spike — %s pts", groupThousands(req.score))
		if err := Dba.insertDriverMedia(req.guid, "screenshot", shot, caption, at.UnixMilli(), 0, req.score, req.delta); err != nil {
			log.Print("driver capture: insert screenshot: ", err)
		}
	}

	// Clip: a short forward window catching the rest of the run.
	clip := stamp + "_clip.mp4"
	if m.grabClip(captureURL, filepath.Join(dir, clip)) {
		caption := fmt.Sprintf("Drift run — %s pts", groupThousands(req.score))
		if trackName != "" {
			caption += " · " + trackName
		}
		if err := Dba.insertDriverMedia(req.guid, "clip", clip, caption, at.UnixMilli(), captureClipSeconds, req.score, req.delta); err != nil {
			log.Print("driver capture: insert clip: ", err)
		}
	}

	m.prune(req.guid, dir)
}

func (m *captureManager) grabFrame(srcURL, outPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-nostdin", "-y", "-loglevel", "error",
		"-i", srcURL,
		"-frames:v", "1", "-q:v", "3",
		outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("driver capture: frame grab failed: %v %s", err, strings.TrimSpace(string(out)))
		return false
	}
	return fileNonEmpty(outPath)
}

func (m *captureManager) grabClip(srcURL, outPath string) bool {
	// Stream copy first (fast, no re-encode). `-t` before `-i` bounds how much
	// of the live input is read.
	ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel()
	copyCmd := exec.CommandContext(ctx, m.ffmpeg,
		"-nostdin", "-y", "-loglevel", "error",
		"-t", strconv.Itoa(captureClipSeconds),
		"-i", srcURL,
		"-c", "copy", "-movflags", "+faststart",
		outPath)
	if out, err := copyCmd.CombinedOutput(); err == nil && fileNonEmpty(outPath) {
		return true
	} else if err != nil {
		log.Printf("driver capture: clip copy failed, re-encoding: %v %s", err, strings.TrimSpace(string(out)))
	}

	// Fall back to H.264 for sources that won't copy cleanly into mp4.
	ctx2, cancel2 := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel2()
	encCmd := exec.CommandContext(ctx2, m.ffmpeg,
		"-nostdin", "-y", "-loglevel", "error",
		"-t", strconv.Itoa(captureClipSeconds),
		"-i", srcURL,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-movflags", "+faststart",
		outPath)
	if out, err := encCmd.CombinedOutput(); err != nil {
		log.Printf("driver capture: clip encode failed: %v %s", err, strings.TrimSpace(string(out)))
		return false
	}
	return fileNonEmpty(outPath)
}

func (m *captureManager) captureURLFor(guid string) string {
	streams, err := Dba.selectDriverStreamsByGuids([]string{guid})
	if err != nil {
		return ""
	}
	ds, ok := streams[guid]
	if !ok {
		return ""
	}
	return strings.TrimSpace(derefOrEmpty(ds.StreamCaptureUrl))
}

func (m *captureManager) trackName(key, config string) string {
	if key == "" {
		return ""
	}
	info, err := Dba.selectTrackInfoMap()
	if err != nil {
		return key
	}
	if t, ok := info[key+"\x00"+config]; ok && t.name != "" {
		return t.name
	}
	if t, ok := info[key]; ok && t.name != "" {
		return t.name
	}
	return key
}

// prune keeps the highest-scoring and most-recent captures for a driver and
// deletes the rest (rows + files), bounding disk use.
func (m *captureManager) prune(guid, dir string) {
	items, err := Dba.listDriverMedia(guid)
	if err != nil {
		log.Print("driver capture: prune list: ", err)
		return
	}
	keep := map[int64]bool{}

	byScore := append([]mediaRow(nil), items...)
	sort.SliceStable(byScore, func(i, j int) bool { return byScore[i].triggerScore > byScore[j].triggerScore })
	for i, it := range byScore {
		if i >= retainTopByScore {
			break
		}
		keep[it.id] = true
	}

	byRecent := append([]mediaRow(nil), items...)
	sort.SliceStable(byRecent, func(i, j int) bool { return byRecent[i].capturedAt > byRecent[j].capturedAt })
	for i, it := range byRecent {
		if i >= retainRecent {
			break
		}
		keep[it.id] = true
	}

	var delIds []int64
	for _, it := range items {
		if keep[it.id] {
			continue
		}
		delIds = append(delIds, it.id)
		if it.path != "" {
			_ = os.Remove(filepath.Join(dir, filepath.Base(it.path)))
		}
	}
	if len(delIds) > 0 {
		if err := Dba.deleteDriverMedia(delIds); err != nil {
			log.Print("driver capture: prune delete: ", err)
		}
	}
}

// validateCaptureURL accepts the stream protocols ffmpeg can pull from.
func validateCaptureURL(label string, raw *string) error {
	raw = trimmedStringPtr(raw)
	if raw == nil {
		return nil
	}
	u, err := url.Parse(*raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s must be a valid URL", label)
	}
	switch u.Scheme {
	case "http", "https", "rtmp", "rtmps", "rtsp", "srt", "udp":
		return nil
	}
	return fmt.Errorf("%s must be an http(s), rtmp(s), rtsp, srt or udp URL", label)
}

func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// groupThousands renders an int with comma group separators (12480 -> "12,480").
func groupThousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(s[i])
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// ---- capture-specific DB access --------------------------------------------

type mediaRow struct {
	id           int64
	path         string
	capturedAt   int64
	triggerScore int
}

func (dba Dbaccess) selectCaptureGuids() (map[string]bool, error) {
	rows, err := dba.db.Query(`
SELECT driver_guid FROM driver_stream
WHERE enabled = 1 AND stream_capture_url IS NOT NULL AND TRIM(stream_capture_url) <> ''`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			return nil, tracerr.Wrap(err)
		}
		if guid != "" {
			out[guid] = true
		}
	}
	return out, rows.Err()
}

func (dba Dbaccess) insertDriverMedia(guid, kind, path, caption string, capturedAt int64, durationS, triggerScore, triggerDelta int) error {
	var dur sql.NullInt64
	if durationS > 0 {
		dur = sql.NullInt64{Int64: int64(durationS), Valid: true}
	}
	_, err := dba.db.Exec(`
INSERT INTO driver_media (driver_guid, kind, path, caption, captured_at, duration_s, trigger_score, trigger_delta)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		guid, kind, path, caption, capturedAt, dur, triggerScore, triggerDelta)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) listDriverMedia(guid string) ([]mediaRow, error) {
	rows, err := dba.db.Query(`
SELECT id, path, captured_at, COALESCE(trigger_score, 0) FROM driver_media WHERE driver_guid = ?`, guid)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := make([]mediaRow, 0)
	for rows.Next() {
		var r mediaRow
		if err := rows.Scan(&r.id, &r.path, &r.capturedAt, &r.triggerScore); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (dba Dbaccess) deleteDriverMedia(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	_, err := dba.db.Exec(`DELETE FROM driver_media WHERE id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}
