package main

// Rolling-buffer stream recorder. Per connected driver with a capture URL, one
// persistent ffmpeg continuously segments the raw stream into a ring of short
// MPEG-TS files (filename = wall-clock start). On a drift spike or a manual
// "Record now", a clip is assembled by concatenating the segments that cover
// the moment, then the screenshot is pulled from that local clip — no
// per-event network connect, so the capture reflects the actual moment instead
// of "a few seconds after ffmpeg woke up".
//
// A reconciler keeps the running recorder set equal to (connected drivers ∩
// armed drivers) ∪ (drivers with an active manual recording); a janitor ages
// out segments past the ring window so disk stays bounded.

import (
	"context"
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
)

const (
	bufSegmentSeconds    = 2                // length of each buffer segment
	bufRingSeconds       = 180              // history kept on disk per driver
	maxBufferRecorders   = 8                // simultaneous persistent ffmpeg recorders
	reconcileInterval    = 15 * time.Second // recorder set reconciliation tick
	janitorInterval      = 12 * time.Second // buffer prune tick
	recorderGrace        = 30 * time.Second // keep recording this long after a driver leaves
	assembleFlushMargin  = 3 * time.Second  // wait past a window's end for ffmpeg to flush
	localFfmpegTimeout   = 30 * time.Second
	recorderStallTimeout = 20 * time.Second // alive but no fresh segments this long → restart
)

// bufferRecorder is one persistent segmenting ffmpeg for a driver's stream.
type bufferRecorder struct {
	guid      string
	url       string
	dir       string // buffer dir (per-driver "buffer" subfolder)
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	startedAt int64

	mu     sync.Mutex
	exited bool
}

func (r *bufferRecorder) dead() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exited
}

func (r *bufferRecorder) stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

type bufSeg struct {
	path    string
	startMs int64
}

func bufferDir(guid string) string {
	return filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid), "buffer")
}

// segStartFromName parses the wall-clock start encoded in a segment filename
// ("seg_20060102-150405.ts" -> unix-ms). Returns 0 on any mismatch.
func segStartFromName(name string) int64 {
	name = strings.TrimSuffix(name, ".ts")
	name = strings.TrimPrefix(name, "seg_")
	t, err := time.ParseInLocation("20060102-150405", name, time.Local)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}

func listBufferSegments(dir string) []bufSeg {
	matches, err := filepath.Glob(filepath.Join(dir, "seg_*.ts"))
	if err != nil {
		return nil
	}
	segs := make([]bufSeg, 0, len(matches))
	for _, p := range matches {
		if st := segStartFromName(filepath.Base(p)); st > 0 {
			segs = append(segs, bufSeg{path: p, startMs: st})
		}
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].startMs < segs[j].startMs })
	return segs
}

// selectSegments returns, in order, the segments whose time span intersects
// [startMs, endMs). segs must be sorted ascending by start.
func selectSegments(segs []bufSeg, startMs, endMs int64) []bufSeg {
	out := make([]bufSeg, 0, len(segs))
	for i, s := range segs {
		segEnd := s.startMs + int64(bufSegmentSeconds)*1000
		if i+1 < len(segs) {
			segEnd = segs[i+1].startMs
		}
		if segEnd > startMs && s.startMs < endMs {
			out = append(out, s)
		}
	}
	return out
}

// inputArgs are ffmpeg input options tuned per protocol so the recorder fails
// fast / reconnects instead of hanging.
func inputArgs(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return []string{"-rw_timeout", "15000000", "-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "4"}
	case "rtsp":
		return []string{"-rtsp_transport", "tcp"}
	}
	return nil
}

func (m *captureManager) startRecorder(guid, srcURL string) *bufferRecorder {
	dir := bufferDir(guid)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Print("driver capture: buffer mkdir: ", err)
		return nil
	}
	args := []string{"-nostdin", "-loglevel", "error"}
	args = append(args, inputArgs(srcURL)...)
	args = append(args,
		"-i", srcURL,
		"-c", "copy",
		"-f", "segment",
		"-segment_time", strconv.Itoa(bufSegmentSeconds),
		"-segment_format", "mpegts",
		"-reset_timestamps", "1",
		"-strftime", "1",
		filepath.Join(dir, "seg_%Y%m%d-%H%M%S.ts"),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.ffmpeg, args...)
	if err := cmd.Start(); err != nil {
		cancel()
		log.Printf("driver capture: recorder start failed for %s: %v", guid, err)
		return nil
	}
	r := &bufferRecorder{guid: guid, url: srcURL, dir: dir, cmd: cmd, cancel: cancel, startedAt: time.Now().UnixMilli()}
	go func() {
		err := cmd.Wait()
		r.mu.Lock()
		r.exited = true
		r.mu.Unlock()
		// ctx.Err() != nil means we cancelled it on purpose — not an error.
		if err != nil && ctx.Err() == nil {
			log.Printf("driver capture: recorder for %s ended: %v", guid, err)
		}
	}()
	log.Printf("driver capture: rolling buffer started for %s", guid)
	return r
}

// ---- recorder lifecycle -----------------------------------------------------

func (m *captureManager) startBackgroundLoops() {
	go m.reconcileLoop()
	go m.janitorLoop()
}

func (m *captureManager) reconcileLoop() {
	t := time.NewTicker(reconcileInterval)
	defer t.Stop()
	for {
		m.reconcile()
		select {
		case <-t.C:
		case <-m.nudgeCh:
		}
	}
}

// nudge asks the reconciler to run before its next tick (driver joined/left, or
// a stream config / manual recording change). Non-blocking, coalesced.
func (m *captureManager) nudge() {
	if m == nil {
		return
	}
	select {
	case m.nudgeCh <- struct{}{}:
	default:
	}
}

func (m *captureManager) janitorLoop() {
	t := time.NewTicker(janitorInterval)
	defer t.Stop()
	for range t.C {
		m.pruneBuffers()
	}
}

// computeDesired is the set of guids that should have a running recorder.
//
// 24/7 model: record every armed stream (enabled + capture URL configured),
// whether or not the driver is connected — so the buffer is always warm before
// a session and streams can be debugged any time from the debug screen.
// Persistence is separate: spike clips are only written while the driver is
// online and drifting (driverDrift), manual clips only on "Record now".
func (m *captureManager) computeDesired() map[string]bool {
	out := map[string]bool{}
	m.mu.Lock()
	for g := range m.armedGuids {
		out[g] = true
	}
	m.mu.Unlock()
	m.manualMu.Lock()
	for g := range m.manual {
		out[g] = true
	}
	m.manualMu.Unlock()
	return out
}

func onlineDriverGuids() map[string]bool {
	out := map[string]bool{}
	if Instances == nil {
		return out
	}
	for _, inst := range Instances.All() {
		if !inst.isRunning() {
			continue
		}
		for _, d := range inst.driversSnapshot() {
			if d.Connected && d.Guid != "" {
				out[d.Guid] = true
			}
		}
	}
	return out
}

func (m *captureManager) reconcile() {
	if m == nil || !m.enabled {
		return
	}
	m.mu.Lock()
	capEnabled := m.cfg.enabled
	m.mu.Unlock()

	desired := map[string]bool{}
	if capEnabled {
		desired = m.computeDesired()
	}
	now := time.Now().UnixMilli()

	m.recMu.Lock()
	defer m.recMu.Unlock()
	for g := range desired {
		m.lastDesiredAt[g] = now
	}
	// Start or restart recorders for desired drivers — restart if the process
	// died (stream blip, ffmpeg crash) or stalled (alive but writing no
	// segments, e.g. an unpullable WebRTC/LL-HLS source). A recorder that keeps
	// failing is retried with growing backoff so a bad source doesn't thrash.
	for g := range desired {
		if r := m.recorders[g]; r != nil {
			dead := r.dead()
			stalled := !dead && m.recorderStalled(g, r.startedAt)
			if !dead && !stalled {
				m.recFails[g] = 0 // healthy — clear backoff
				continue
			}
			if stalled {
				log.Printf("driver capture: recorder for %s stalled (no fresh segments) — stopping", g)
			}
			r.stop()
			delete(m.recorders, g)
			m.recFails[g]++
			m.recNextAttempt[g] = now + recorderBackoffMs(m.recFails[g])
			continue // restart on a later tick once the backoff elapses
		}
		if now < m.recNextAttempt[g] || len(m.recorders) >= maxBufferRecorders {
			continue
		}
		url := m.captureURLFor(g)
		if url == "" {
			continue
		}
		if nr := m.startRecorder(g, url); nr != nil {
			m.recorders[g] = nr
		}
	}
	// Stop recorders no longer desired, after a grace window so a brief drop
	// doesn't discard the buffer.
	graceMs := int64(recorderGrace / time.Millisecond)
	for g, r := range m.recorders {
		if desired[g] {
			continue
		}
		if now-m.lastDesiredAt[g] > graceMs {
			r.stop()
			delete(m.recorders, g)
			delete(m.recFails, g)
			delete(m.recNextAttempt, g)
			log.Printf("driver capture: rolling buffer stopped for %s", g)
		}
	}
}

// recorderBackoffMs grows the retry delay for a repeatedly failing source:
// 15s, 30s, 60s, 120s, then capped at 300s.
func recorderBackoffMs(fails int) int64 {
	steps := []int64{15, 30, 60, 120, 300}
	i := fails - 1
	if i < 0 {
		i = 0
	}
	if i >= len(steps) {
		i = len(steps) - 1
	}
	return steps[i] * 1000
}

// recorderStalled reports whether a live recorder has produced no fresh segment
// for recorderStallTimeout (after a spin-up grace). True for a source ffmpeg
// connects to but can't actually demux (WebRTC/LL-HLS).
func (m *captureManager) recorderStalled(guid string, startedAt int64) bool {
	now := time.Now().UnixMilli()
	stallMs := int64(recorderStallTimeout / time.Millisecond)
	if now-startedAt < stallMs {
		return false // still spinning up
	}
	segs := listBufferSegments(bufferDir(guid))
	if len(segs) == 0 {
		return true
	}
	return now-segs[len(segs)-1].startMs > stallMs
}

func (m *captureManager) pruneBuffers() {
	matches, err := filepath.Glob(filepath.Join(mediaBaseDir(), "drivers", "*", "buffer", "seg_*.ts"))
	if err != nil {
		return
	}
	cutoff := time.Now().UnixMilli() - int64(bufRingSeconds)*1000
	for _, p := range matches {
		if st := segStartFromName(filepath.Base(p)); st > 0 && st < cutoff {
			_ = os.Remove(p)
		}
	}
}

// ---- clip assembly from the buffer ------------------------------------------

// waitForWindow blocks until the recorder has had time to flush the segment(s)
// past endMs to disk.
func (m *captureManager) waitForWindow(endMs int64) {
	target := endMs + int64(assembleFlushMargin/time.Millisecond) + int64(bufSegmentSeconds)*1000
	for {
		now := time.Now().UnixMilli()
		if now >= target {
			return
		}
		d := time.Duration(target-now) * time.Millisecond
		if d > 5*time.Second {
			d = 5 * time.Second
		}
		time.Sleep(d)
	}
}

// assembleClip concatenates the buffered segments covering [startMs, endMs) into
// outPath (mp4) and returns the wall-clock start of the first segment used.
func (m *captureManager) assembleClip(guid string, startMs, endMs int64, outPath string) (int64, bool) {
	sel := selectSegments(listBufferSegments(bufferDir(guid)), startMs, endMs)
	if len(sel) == 0 {
		return 0, false
	}
	listPath := outPath + ".concat.txt"
	var b strings.Builder
	for _, s := range sel {
		// concat demuxer entries: single-quoted, with embedded quotes escaped.
		b.WriteString("file '")
		b.WriteString(strings.ReplaceAll(s.path, "'", `'\''`))
		b.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(b.String()), 0o644); err != nil {
		log.Print("driver capture: write concat list: ", err)
		return 0, false
	}
	defer os.Remove(listPath)

	if m.concatEncode(listPath, outPath, endMs-startMs) {
		return sel[0].startMs, true
	}
	return 0, false
}

// concatEncode stitches the buffered TS segments into outPath, RE-ENCODING
// rather than stream-copying. Copy is faster but the segmenter resets each
// segment's timestamps to ~0, and concat-copying them back into one MP4 leaves
// a non-monotonic DTS at every 2s join. Strict browser decoders (Chrome/FF MSE)
// treat that as fatal and stall on the glitchy frame (QuickTime resyncs past
// it). Re-encoding rebuilds a clean, monotonic, zero-based timeline.
// ponytail: re-encode always — correctness over the copy fast-path that shipped
// unplayable clips. Re-add a copy path only if it produces verified-monotonic ts.
func (m *captureManager) concatEncode(listPath, outPath string, windowMs int64) bool {
	// Budget the re-encode off the clip length, not a fixed cap: a 120s manual
	// clip needs far longer than a 14s auto-clip. CommandContext sends SIGKILL on
	// the deadline, which leaves a moov-less, unplayable MP4 — so a too-short
	// timeout silently produces a broken file. Allow ~3x realtime + headroom,
	// floored at the old short-clip budget. ponytail: 3x is slack for a busy box;
	// veryfast usually beats realtime.
	timeout := time.Duration(windowMs)*3*time.Millisecond + 30*time.Second
	if timeout < 2*localFfmpegTimeout {
		timeout = 2 * localFfmpegTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-nostdin", "-y", "-loglevel", "error",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-movflags", "+faststart",
		outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("driver capture: clip concat encode failed: %v %s", err, strings.TrimSpace(string(out)))
		// A killed/failed encode leaves a partial, moov-less file — drop it so it
		// can't be served or orphaned on disk.
		_ = os.Remove(outPath)
		return false
	}
	return fileNonEmpty(outPath)
}

// grabLatestFrame extracts a still from the freshest fully-written buffer
// segment for the driver. Returns false if nothing is buffered yet.
func (m *captureManager) grabLatestFrame(guid, outPath string) bool {
	segs := listBufferSegments(bufferDir(guid))
	if len(segs) == 0 {
		return false
	}
	// Prefer the second-newest segment — the newest is still being written.
	src := segs[len(segs)-1].path
	if len(segs) >= 2 {
		src = segs[len(segs)-2].path
	}
	return m.grabFrameFromFileEnd(src, outPath)
}

// grabFrameFromFileEnd grabs the frame ~1s before the end of a local file (the
// freshest decodable still).
func (m *captureManager) grabFrameFromFileEnd(src, outPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), localFfmpegTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-nostdin", "-y", "-loglevel", "error",
		"-sseof", "-1",
		"-i", src,
		"-frames:v", "1", "-q:v", "3",
		outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("driver capture: snapshot frame failed: %v %s", err, strings.TrimSpace(string(out)))
		return false
	}
	return fileNonEmpty(outPath)
}

// grabFrameFromFile pulls a single still at offsetSec into a local clip file.
func (m *captureManager) grabFrameFromFile(src string, offsetSec float64, outPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), localFfmpegTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.ffmpeg,
		"-nostdin", "-y", "-loglevel", "error",
		"-ss", strconv.FormatFloat(offsetSec, 'f', 3, 64),
		"-i", src,
		"-frames:v", "1", "-q:v", "3",
		outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("driver capture: frame extract failed: %v %s", err, strings.TrimSpace(string(out)))
		return false
	}
	return fileNonEmpty(outPath)
}
