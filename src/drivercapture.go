package main

// Drift-run auto-capture. A rolling buffer (driverbuffer.go) continuously
// records each armed stream; when a drift run ENDS whose peak score cleared the
// trigger, driverDrift (drivers.go) submits one captureRequest spanning the
// run. This engine cuts the clip from the buffer and pulls the peak-moment
// screenshot — no per-event network connect — filing them as driver_media so
// they surface in the Driver Detail highlight reel.
//
// Throttling keeps the reel curated rather than endless:
//   - one capture per drift run            (fired once at run end)
//   - per-session cap                       (DriverState.captureCount)
//   - per-driver cooldown                   (DriverState.lastCaptureMs)
//   - global concurrency + per-minute rate  (captureManager.sem / recent)
//   - retention: keep the top-by-score + most-recent, prune the rest
//
// Capture needs ffmpeg on PATH and a raw, server-reachable capture URL
// (HLS/RTMP/SRT/RTSP) — WebRTC/WHEP and Low-Latency HLS can't be pulled. Every
// ffmpeg failure is logged and swallowed; nothing here blocks the UDP loop.

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ztrue/tracerr"
)

const (
	// Defaults for the user-tunable trigger settings; overridden per-install by
	// user_config via the configuration page (see loadCaptureSettings).
	defaultCaptureTriggerScore  = 2500
	defaultCaptureClipSeconds   = 14
	defaultCaptureCooldownSec   = 45
	defaultCaptureMaxPerSession = 12
	maxCaptureClipSeconds       = 120 // clamp on the configured clip length

	// Internal safety limits (not user-tunable).
	maxCaptureConcurrent = 2 // simultaneous spike-assembly jobs
	maxCapturesPerMinute = 8 // global ceiling
	retainTopByScore     = 12
	retainRecent         = 8

	manualMaxDuration   = 120 * time.Second // safety cap for a manual recording
	maxManualConcurrent = 4
)

type captureRequest struct {
	guid        string
	driverName  string
	trackKey    string
	trackConfig string
	score       int   // peak score of the run
	delta       int   // peak - baseline
	runStartMs  int64 // when the run's live score first rose from zero
	runEndMs    int64 // when the run ended
	peakMs      int64 // when the peak score occurred (screenshot moment)
	driftRunId  int64 // driver_drift_run row this capture belongs to (0 = unknown)
	connectionId int64 // driver_connection (session) this capture belongs to (0 = unknown)
}

type captureManager struct {
	enabled bool   // ffmpeg present on PATH
	ffmpeg  string // resolved ffmpeg binary

	mu         sync.Mutex
	armedGuids map[string]bool // driver guids with a usable capture URL
	cfg        captureSettings // tunables from user_config
	recent     []int64         // unix-ms of recent captures (global rate window)

	sem chan struct{} // spike-assembly concurrency limiter

	// Rolling-buffer recorders (driverbuffer.go): one persistent ffmpeg per
	// connected+armed driver, reconciled by a background loop.
	recMu          sync.Mutex
	recorders      map[string]*bufferRecorder // guid -> running recorder
	lastDesiredAt  map[string]int64           // guid -> last time it was wanted (grace)
	recFails       map[string]int             // guid -> consecutive start failures (backoff)
	recNextAttempt map[string]int64           // guid -> earliest next start (unix-ms)
	nudgeCh        chan struct{}              // wake the reconciler early

	manualMu sync.Mutex
	manual   map[string]*manualMark // guid -> in-progress manual recording
}

var Captures *captureManager

func newCaptureManager() *captureManager {
	m := &captureManager{
		armedGuids:     map[string]bool{},
		recorders:      map[string]*bufferRecorder{},
		lastDesiredAt:  map[string]int64{},
		recFails:       map[string]int{},
		recNextAttempt: map[string]int64{},
		manual:         map[string]*manualMark{},
		nudgeCh:        make(chan struct{}, 1),
		sem:            make(chan struct{}, maxCaptureConcurrent),
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		m.enabled = true
		m.ffmpeg = path
		log.Print("driver capture: ffmpeg found, rolling-buffer highlight capture enabled")
	} else {
		log.Print("driver capture: ffmpeg not found on PATH — stream highlight capture disabled")
	}
	m.refresh()
	if m.enabled {
		m.startBackgroundLoops()
	}
	return m
}

// refresh reloads the set of driver guids that have a capture-source URL.
// Called at startup and after any driver-stream change.
func (m *captureManager) refresh() {
	if m == nil {
		return
	}
	cfg := loadCaptureSettings()
	guids, err := Dba.selectCaptureGuids()
	m.mu.Lock()
	m.cfg = cfg
	if err == nil {
		m.armedGuids = guids
	}
	m.mu.Unlock()
	if err != nil {
		log.Print("driver capture: refresh guids: ", err)
	}
	// A stream may have been (dis)armed or its URL changed — re-evaluate which
	// recorders should run.
	m.nudge()
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

// run assembles a finished drift run's clip + screenshot from the rolling
// buffer. No network connect: the recorder is already capturing, so we cut the
// segments spanning the whole run (a little lead-in/trail), and grab the
// screenshot at the run's peak moment. Clip length tracks the run length.
func (m *captureManager) run(req captureRequest) {
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	if !cfg.clips && !cfg.screenshots {
		return
	}

	guid := req.guid
	dir := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid))
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Print("driver capture: mkdir: ", err)
		return
	}

	const preMs, postMs = 3000, 3000
	startMs := req.runStartMs - preMs
	endMs := req.runEndMs + postMs
	// Never ask for more than the buffer can hold.
	if maxSpan := int64(bufRingSeconds-5) * 1000; endMs-startMs > maxSpan {
		startMs = endMs - maxSpan
	}

	m.waitForWindow(endMs)

	stamp := strconv.FormatInt(req.runEndMs, 10)
	clipFile := stamp + "_clip.mp4"
	clipPath := filepath.Join(dir, clipFile)
	clipStartMs, ok := m.assembleClip(guid, startMs, endMs, clipPath)
	if !ok {
		log.Printf("driver capture: no buffered video for %s run — recorder may have just started", guid)
		return
	}
	at := time.Now()
	trackName := m.trackName(req.trackKey, req.trackConfig)
	durS := int((endMs - startMs) / 1000)

	// Screenshot: pull the peak frame straight from the local clip.
	if cfg.screenshots {
		shot := stamp + "_shot.jpg"
		offset := float64(req.peakMs-clipStartMs) / 1000.0
		if offset < 0 {
			offset = 0
		}
		if m.grabFrameFromFile(clipPath, offset, filepath.Join(dir, shot)) {
			caption := fmt.Sprintf("Drift spike — %s pts", groupThousands(req.score))
			if err := Dba.insertDriverMedia(guid, "screenshot", shot, caption, at.UnixMilli(), 0, req.score, req.delta, req.driftRunId, req.connectionId); err != nil {
				log.Print("driver capture: insert screenshot: ", err)
			}
		}
	}

	if cfg.clips {
		caption := fmt.Sprintf("Drift run — %s pts", groupThousands(req.score))
		if trackName != "" {
			caption += " · " + trackName
		}
		if err := Dba.insertDriverMedia(guid, "clip", clipFile, caption, at.UnixMilli(), durS, req.score, req.delta, req.driftRunId, req.connectionId); err != nil {
			log.Print("driver capture: insert clip: ", err)
		}
	} else {
		_ = os.Remove(clipPath) // built only to source the screenshot
	}

	m.prune(guid, dir)
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

func (dba Dbaccess) insertDriverMedia(guid, kind, path, caption string, capturedAt int64, durationS, triggerScore, triggerDelta int, driftRunId, connectionId int64) error {
	var dur, ts, td, runId, connId sql.NullInt64
	if durationS > 0 {
		dur = sql.NullInt64{Int64: int64(durationS), Valid: true}
	}
	// Manual recordings pass score 0 -> store NULL so they get no spike badge.
	if triggerScore > 0 {
		ts = sql.NullInt64{Int64: int64(triggerScore), Valid: true}
		td = sql.NullInt64{Int64: int64(triggerDelta), Valid: true}
	}
	// Manual/snapshot media pass 0 -> store NULL (not tied to a scored run).
	if driftRunId > 0 {
		runId = sql.NullInt64{Int64: driftRunId, Valid: true}
	}
	// 0 -> NULL: capture happened outside any tracked connection (legacy/edge).
	if connectionId > 0 {
		connId = sql.NullInt64{Int64: connectionId, Valid: true}
	}
	_, err := dba.db.Exec(`
INSERT INTO driver_media (driver_guid, kind, path, caption, captured_at, duration_s, trigger_score, trigger_delta, drift_run_id, connection_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		guid, kind, path, caption, capturedAt, dur, ts, td, runId, connId)
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

// ---- settings ---------------------------------------------------------------

type captureSettings struct {
	enabled       bool
	screenshots   bool
	clips         bool
	triggerScore  int
	clipSeconds   int
	cooldownMs    int
	maxPerSession int
}

// loadCaptureSettings reads the auto-capture tunables from user_config, applying
// defaults for any unset/invalid value.
func loadCaptureSettings() captureSettings {
	s := captureSettings{
		enabled:       true,
		screenshots:   true,
		clips:         true,
		triggerScore:  defaultCaptureTriggerScore,
		clipSeconds:   defaultCaptureClipSeconds,
		cooldownMs:    defaultCaptureCooldownSec * 1000,
		maxPerSession: defaultCaptureMaxPerSession,
	}
	cfg, err := Dba.selectConfig()
	if err != nil {
		return s
	}
	if cfg.CaptureEnabled != nil {
		s.enabled = *cfg.CaptureEnabled != 0
	}
	if cfg.CaptureScreenshots != nil {
		s.screenshots = *cfg.CaptureScreenshots != 0
	}
	if cfg.CaptureClips != nil {
		s.clips = *cfg.CaptureClips != 0
	}
	if cfg.CaptureTriggerScore != nil && *cfg.CaptureTriggerScore > 0 {
		s.triggerScore = *cfg.CaptureTriggerScore
	}
	if cfg.CaptureClipSeconds != nil && *cfg.CaptureClipSeconds > 0 {
		s.clipSeconds = *cfg.CaptureClipSeconds
	}
	if s.clipSeconds > maxCaptureClipSeconds {
		s.clipSeconds = maxCaptureClipSeconds
	}
	if cfg.CaptureCooldownSeconds != nil && *cfg.CaptureCooldownSeconds >= 0 {
		s.cooldownMs = *cfg.CaptureCooldownSeconds * 1000
	}
	if cfg.CaptureMaxPerSession != nil && *cfg.CaptureMaxPerSession > 0 {
		s.maxPerSession = *cfg.CaptureMaxPerSession
	}
	return s
}

// shouldTrigger applies the configured auto-capture gates (enable, score
// threshold, per-session cap, cooldown) plus the armed check. The one-capture-
// per-run guard lives on DriverState and is checked by the caller.
func (m *captureManager) shouldTrigger(guid string, score, count int, lastMs, now int64) bool {
	if m == nil || !m.enabled || guid == "" {
		return false
	}
	m.mu.Lock()
	armed := m.armedGuids[guid]
	cfg := m.cfg
	m.mu.Unlock()
	if !armed || !cfg.enabled || (!cfg.screenshots && !cfg.clips) {
		return false
	}
	if score < cfg.triggerScore || count >= cfg.maxPerSession || now-lastMs < int64(cfg.cooldownMs) {
		return false
	}
	return true
}

// ---- manual "Record now" ----------------------------------------------------

// manualMark is an in-progress operator-triggered recording. The rolling buffer
// keeps capturing regardless; we just remember when "Record now" was pressed
// and assemble a clip spanning from then until the driver's next drift run ends
// (or manualMaxDuration as a safety cap). It also pins the recorder on so the
// buffer keeps filling even if the driver isn't connected to a server.
type manualMark struct {
	guid    string
	startMs int64
	timer   *time.Timer
}

// startManualRecording arms a manual recording: it marks the start, ensures a
// recorder is running so the buffer fills from now forward, and finalizes when
// the driver's next drift run ends. Returns a user-facing error when capture
// isn't possible or one is already running.
func (m *captureManager) startManualRecording(guid string) error {
	if m == nil || !m.enabled {
		return errors.New("capture is unavailable: ffmpeg is not installed on the server")
	}
	if m.captureURLFor(guid) == "" {
		return errors.New("no capture URL is configured for this driver")
	}

	m.manualMu.Lock()
	if _, busy := m.manual[guid]; busy {
		m.manualMu.Unlock()
		return errors.New("a recording is already in progress for this driver")
	}
	if len(m.manual) >= maxManualConcurrent {
		m.manualMu.Unlock()
		return errors.New("too many recordings in progress — try again shortly")
	}
	mark := &manualMark{guid: guid, startMs: time.Now().UnixMilli()}
	mark.timer = time.AfterFunc(manualMaxDuration, func() { m.finalizeManual(guid) })
	m.manual[guid] = mark
	m.manualMu.Unlock()

	m.nudge() // make sure a recorder is (or comes) up to fill the buffer
	log.Printf("driver capture: manual recording armed for %s", guid)
	return nil
}

// onDriftRunEnd finalizes a manual recording when the driver's current/next
// drift run ends. No-op if none is armed.
func (m *captureManager) onDriftRunEnd(guid string) {
	if m == nil || guid == "" {
		return
	}
	m.finalizeManual(guid)
}

// stopManualRecording finalizes an in-progress manual recording immediately
// (assembling the clip up to now). Errors when none is armed.
func (m *captureManager) stopManualRecording(guid string) error {
	if m == nil || !m.enabled {
		return errors.New("capture is unavailable")
	}
	m.manualMu.Lock()
	_, ok := m.manual[guid]
	m.manualMu.Unlock()
	if !ok {
		return errors.New("no recording in progress for this driver")
	}
	m.finalizeManual(guid)
	return nil
}

func (m *captureManager) finalizeManual(guid string) {
	m.manualMu.Lock()
	mark := m.manual[guid]
	if mark == nil {
		m.manualMu.Unlock()
		return
	}
	delete(m.manual, guid)
	m.manualMu.Unlock()
	if mark.timer != nil {
		mark.timer.Stop()
	}
	go m.assembleManual(mark)
}

func (m *captureManager) assembleManual(mark *manualMark) {
	guid := mark.guid
	dir := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid))
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Print("driver capture: mkdir: ", err)
		return
	}
	// A little lead-in before the press, a little trail after the run end.
	startMs := mark.startMs - 4000
	endMs := time.Now().UnixMilli() + 2000
	m.waitForWindow(endMs)

	stamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	file := stamp + "_manual.mp4"
	if _, ok := m.assembleClip(guid, startMs, endMs, filepath.Join(dir, file)); !ok {
		log.Printf("driver capture: manual recording for %s produced no file", guid)
		return
	}
	// duration 0: the mm:ss badge is for short auto-clips; manual clips can run
	// long, so leave it off (matches the prior manual-recording behaviour).
	if err := Dba.insertDriverMedia(guid, "clip", file, "Manual recording", time.Now().UnixMilli(), 0, 0, 0, 0, liveConnectionIdForGuid(guid)); err != nil {
		log.Print("driver capture: insert manual clip: ", err)
	}
	m.prune(guid, dir)
	log.Printf("driver capture: manual recording finished for %s", guid)
}

// takeSnapshot grabs a still from the freshest buffered video and files it as a
// screenshot. Returns the media filename, or an error when capture is
// unavailable or the buffer is empty (stream down / just started).
func (m *captureManager) takeSnapshot(guid string) (string, error) {
	if m == nil || !m.enabled {
		return "", errors.New("capture is unavailable: ffmpeg is not installed on the server")
	}
	if m.captureURLFor(guid) == "" {
		return "", errors.New("no capture URL is configured for this driver")
	}
	dir := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid))
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", err
	}
	stamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	file := stamp + "_snap.jpg"
	if !m.grabLatestFrame(guid, filepath.Join(dir, file)) {
		return "", errors.New("no buffered video yet — the stream may be down or still starting")
	}
	// trigger 0 -> NULL -> no spike badge (this is an operator snapshot).
	if err := Dba.insertDriverMedia(guid, "screenshot", file, "Snapshot", time.Now().UnixMilli(), 0, 0, 0, 0, liveConnectionIdForGuid(guid)); err != nil {
		return "", err
	}
	m.prune(guid, dir)
	return file, nil
}

// apiDriverRecord (POST /api/drivers/:guid/record) starts a manual recording.
func apiDriverRecord(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	if guid == "" {
		apiBadRequest(c, "Invalid driver")
		return
	}
	if Captures == nil {
		apiError(c, http.StatusServiceUnavailable, "capture_unavailable", "Capture is not available.")
		return
	}
	if err := Captures.startManualRecording(guid); err != nil {
		apiError(c, http.StatusConflict, "capture_error", err.Error())
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"status": "recording"})
}

// apiDriverRecordStop (POST /api/drivers/:guid/record/stop) finalizes an
// in-progress manual recording now.
func apiDriverRecordStop(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	if guid == "" {
		apiBadRequest(c, "Invalid driver")
		return
	}
	if Captures == nil {
		apiError(c, http.StatusServiceUnavailable, "capture_unavailable", "Capture is not available.")
		return
	}
	if err := Captures.stopManualRecording(guid); err != nil {
		apiError(c, http.StatusConflict, "capture_error", err.Error())
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"status": "stopped"})
}

// apiDriverSnapshot (POST /api/drivers/:guid/snapshot) grabs a still from the
// driver's live stream now.
func apiDriverSnapshot(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	if guid == "" {
		apiBadRequest(c, "Invalid driver")
		return
	}
	if Captures == nil {
		apiError(c, http.StatusServiceUnavailable, "capture_unavailable", "Capture is not available.")
		return
	}
	file, err := Captures.takeSnapshot(guid)
	if err != nil {
		apiError(c, http.StatusConflict, "capture_error", err.Error())
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"status": "captured", "url": "/api/drivers/" + url.PathEscape(guid) + "/media/" + file})
}
