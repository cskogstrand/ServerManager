package main

import (
	"bytes"
	"strings"
	"sync"
)

// lineRing is a thread-safe, fixed-size ring of the most recent log lines. It is
// added to the logging MultiWriter so any log line is also retained in memory
// for in-app log views (e.g. the streams debug screen) without re-reading the
// log file or racing on the unguarded LogBuffer.
type lineRing struct {
	mu      sync.Mutex
	pending []byte // bytes of a not-yet-terminated line
	lines   []string
	max     int
}

func newLineRing(max int) *lineRing {
	return &lineRing{max: max}
}

// Write satisfies io.Writer. It never retains p (it copies into pending).
func (r *lineRing) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = append(r.pending, p...)
	for {
		i := bytes.IndexByte(r.pending, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(r.pending[:i]), "\r")
		// Drop the consumed bytes, reusing the backing array.
		r.pending = append(r.pending[:0], r.pending[i+1:]...)
		if line == "" {
			continue
		}
		r.lines = append(r.lines, line)
		if len(r.lines) > r.max {
			r.lines = r.lines[len(r.lines)-r.max:]
		}
	}
	return len(p), nil
}

// filtered returns up to max recent lines (oldest→newest) containing any of the
// lowercase keywords. With no keywords, returns the last max lines.
func (r *lineRing) filtered(keywords []string, max int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.lines))
	for _, ln := range r.lines {
		if len(keywords) == 0 {
			out = append(out, ln)
			continue
		}
		low := strings.ToLower(ln)
		for _, k := range keywords {
			if strings.Contains(low, k) {
				out = append(out, ln)
				break
			}
		}
	}
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// LogLines retains recent log lines in memory for in-app log views.
var LogLines = newLineRing(2000)
