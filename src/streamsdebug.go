package main

// Streams debug endpoint. Surfaces everything SM knows about each driver stream
// — capture subsystem state, the per-stream rolling-buffer recorder + on-disk
// segment buffer, recent capture logs — plus a live ffmpeg probe so an operator
// can test a capture URL straight from the server (the definitive "is this
// source pullable" check).

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// streamLogKeywords selects the log lines relevant to streaming/capture.
var streamLogKeywords = []string{"capture", "rolling buffer", "recorder", "buffer", "drift ingest", "stream", "ffmpeg"}

type streamDebugCapture struct {
	FfmpegAvailable bool   `json:"ffmpeg_available"`
	FfmpegPath      string `json:"ffmpeg_path"`
	Enabled         bool   `json:"enabled"`
	Screenshots     bool   `json:"screenshots"`
	Clips           bool   `json:"clips"`
	TriggerScore    int    `json:"trigger_score"`
	ClipSeconds     int    `json:"clip_seconds"`
	CooldownSeconds int    `json:"cooldown_seconds"`
	MaxPerSession   int    `json:"max_per_session"`
	RingSeconds     int    `json:"ring_seconds"`
	SegmentSeconds  int    `json:"segment_seconds"`
	MaxRecorders    int    `json:"max_recorders"`
	ActiveRecorders int    `json:"active_recorders"`
}

type streamDebugRow struct {
	Id                *int   `json:"id"`
	Guid              string `json:"guid"`
	DisplayName       string `json:"display_name"`
	Enabled           bool   `json:"enabled"`
	EmbedUrl          string `json:"embed_url"`
	StatusUrl         string `json:"status_url"`
	CaptureUrl        string `json:"capture_url"`
	CaptureScheme     string `json:"capture_scheme"`
	CaptureSupported  bool   `json:"capture_supported"`
	Armed             bool   `json:"armed"`
	Online            bool   `json:"online"`
	RecorderRunning   bool   `json:"recorder_running"`
	RecorderDead      bool   `json:"recorder_dead"`
	RecorderStartedMs int64  `json:"recorder_started_ms"`
	SegmentCount      int    `json:"segment_count"`
	OldestSegmentMs   int64  `json:"oldest_segment_ms"`
	NewestSegmentMs   int64  `json:"newest_segment_ms"`
	BufferBytes       int64  `json:"buffer_bytes"`
	ManualActive      bool   `json:"manual_active"`
	MediaCount        int    `json:"media_count"`
}

type streamDebugResponse struct {
	NowMs   int64              `json:"now_ms"`
	Capture streamDebugCapture `json:"capture"`
	Streams []streamDebugRow   `json:"streams"`
	Logs    []string           `json:"logs"`
}

// schemeInfo returns a capture URL's scheme and whether ffmpeg can pull it.
func schemeInfo(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "rtmp", "rtmps", "rtsp", "srt", "udp":
		return scheme, true
	}
	return scheme, false
}

func apiStreamsDebug(c *gin.Context) {
	streams, err := Dba.selectDriverStreams()
	if err != nil {
		apiDbError(c, err)
		return
	}
	online := onlineDriverGuids()

	resp := streamDebugResponse{
		NowMs:   time.Now().UnixMilli(),
		Capture: Captures.debugCapture(),
		Streams: make([]streamDebugRow, 0, len(streams)),
		Logs:    LogLines.filtered(streamLogKeywords, 300),
	}
	for _, ds := range streams {
		guid := strings.TrimSpace(derefOrEmpty(ds.DriverGuid))
		row := streamDebugRow{
			Id:          ds.Id,
			Guid:        guid,
			DisplayName: derefOrEmpty(ds.DisplayName),
			Enabled:     ds.Enabled != nil && *ds.Enabled != 0,
			EmbedUrl:    derefOrEmpty(ds.StreamEmbedUrl),
			StatusUrl:   derefOrEmpty(ds.StreamStatusUrl),
			CaptureUrl:  strings.TrimSpace(derefOrEmpty(ds.StreamCaptureUrl)),
			Online:      online[guid],
		}
		row.CaptureScheme, row.CaptureSupported = schemeInfo(row.CaptureUrl)
		Captures.fillDebugRow(&row, guid)
		if guid != "" {
			if media, err := Dba.listDriverMedia(guid); err == nil {
				row.MediaCount = len(media)
			}
		}
		resp.Streams = append(resp.Streams, row)
	}
	c.PureJSON(http.StatusOK, resp)
}

func (m *captureManager) debugCapture() streamDebugCapture {
	out := streamDebugCapture{
		RingSeconds:    bufRingSeconds,
		SegmentSeconds: bufSegmentSeconds,
		MaxRecorders:   maxBufferRecorders,
	}
	if m == nil {
		return out
	}
	out.FfmpegAvailable = m.enabled
	out.FfmpegPath = m.ffmpeg
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	out.Enabled = cfg.enabled
	out.Screenshots = cfg.screenshots
	out.Clips = cfg.clips
	out.TriggerScore = cfg.triggerScore
	out.ClipSeconds = cfg.clipSeconds
	out.CooldownSeconds = cfg.cooldownMs / 1000
	out.MaxPerSession = cfg.maxPerSession
	m.recMu.Lock()
	out.ActiveRecorders = len(m.recorders)
	m.recMu.Unlock()
	return out
}

func (m *captureManager) fillDebugRow(row *streamDebugRow, guid string) {
	if m == nil || guid == "" {
		return
	}
	m.mu.Lock()
	row.Armed = m.armedGuids[guid]
	m.mu.Unlock()

	m.recMu.Lock()
	if r := m.recorders[guid]; r != nil {
		dead := r.dead()
		row.RecorderRunning = !dead
		row.RecorderDead = dead
		row.RecorderStartedMs = r.startedAt
	}
	m.recMu.Unlock()

	m.manualMu.Lock()
	_, row.ManualActive = m.manual[guid]
	m.manualMu.Unlock()

	segs := listBufferSegments(bufferDir(guid))
	row.SegmentCount = len(segs)
	if len(segs) > 0 {
		row.OldestSegmentMs = segs[0].startMs
		row.NewestSegmentMs = segs[len(segs)-1].startMs
	}
	var total int64
	for _, s := range segs {
		if fi, err := os.Stat(s.path); err == nil {
			total += fi.Size()
		}
	}
	row.BufferBytes = total
}

// ---- live probe -------------------------------------------------------------

type probeRequest struct {
	Url string `json:"url"`
}

type probeResult struct {
	Ok        bool     `json:"ok"`
	ElapsedMs int64    `json:"elapsed_ms"`
	TimedOut  bool     `json:"timed_out"`
	Error     string   `json:"error"`
	Streams   []string `json:"streams"`
	LogTail   string   `json:"log_tail"`
}

func apiStreamsProbe(c *gin.Context) {
	if Captures == nil || !Captures.enabled {
		apiError(c, http.StatusServiceUnavailable, "capture_unavailable", "ffmpeg is not available on the server.")
		return
	}
	var req probeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid request payload")
		return
	}
	target := strings.TrimSpace(req.Url)
	if target == "" {
		apiBadRequest(c, "No URL provided")
		return
	}
	if err := validateCaptureURL("Capture URL", &target); err != nil {
		apiBadRequest(c, err.Error())
		return
	}
	c.PureJSON(http.StatusOK, Captures.probe(target))
}

// probe runs a short ffmpeg read against the URL and reports what it found. A
// pullable source returns fast with detected streams; a WebRTC/LL-HLS source
// hangs until the timeout (TimedOut) or errors immediately — the diagnostic.
func (m *captureManager) probe(target string) probeResult {
	// Short enough to return before a typical reverse-proxy read timeout; long
	// enough that a working source (which outputs in a few seconds) succeeds and
	// a stalling one (WebRTC/LL-HLS) is caught as TimedOut.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "info"}
	args = append(args, inputArgs(target)...)
	args = append(args, "-i", target, "-t", "2", "-f", "null", "-")
	cmd := exec.CommandContext(ctx, m.ffmpeg, args...)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	res := probeResult{ElapsedMs: time.Since(start).Milliseconds(), Streams: []string{}}
	text := string(out)
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "Stream #") {
			res.Streams = append(res.Streams, t)
		}
	}
	res.LogTail = tailString(text, 4000)
	if ctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}
	res.Ok = err == nil && !res.TimedOut
	if err != nil {
		res.Error = err.Error()
	}
	return res
}

func tailString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}
