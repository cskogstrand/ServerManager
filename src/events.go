package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// EventBroker fans out server events to SSE subscribers. Slow consumers are
// never waited on: a full subscriber channel just drops the message.
type EventBroker struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

var Events = &EventBroker{subs: make(map[chan []byte]struct{})}

func (b *EventBroker) Subscribe() chan []byte {
	ch := make(chan []byte, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *EventBroker) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

// Publish broadcasts one event. instanceId 0 means "not instance-specific"
// (e.g. content jobs).
func (b *EventBroker) Publish(eventType string, instanceId int, payload any) {
	msg, err := json.Marshal(map[string]any{
		"type":        eventType,
		"instance_id": instanceId,
		"ts":          time.Now().Unix(),
		"data":        payload,
	})
	if err != nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- msg:
		default:
		}
	}
}

func sessionEventPayload(s SessionInfo) map[string]any {
	return map[string]any{
		"name":                  s.name,
		"type":                  s.typ,
		"index":                 s.sessionIndex,
		"current_session_index": s.currentSessionIndex,
		"session_count":         s.sessionCount,
		"track":                 s.track,
		"track_config":          s.trackConfig,
		"server_name":           s.serverName,
		"time":                  s.time,
		"laps":                  s.laps,
		"wait_time":             s.waitTime,
		"ambient_temp":          s.ambientTemp,
		"road_temp":             s.roadTemp,
		"weather_graphics":      s.weatherGraphics,
		"elapsed_ms":            s.elapsedMs,
	}
}

func (inst *Instance) publishRunning(running bool) {
	Events.Publish("server", inst.Id(), map[string]any{"running": running})
}

func (inst *Instance) publishPlayers() {
	st := inst.statusSnapshot()
	Events.Publish("players", inst.Id(), map[string]any{"players": st.Players})
}

// apiServerEventsSSE streams broker events as Server-Sent Events. The old UI
// keeps polling; the SPA subscribes here instead.
func apiServerEventsSSE(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		apiError(c, http.StatusInternalServerError, "streaming_unsupported", "Streaming not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	ch := Events.Subscribe()
	defer Events.Unsubscribe(ch)

	// Initial snapshot so a client does not have to wait for the next event
	for _, inst := range Instances.All() {
		inst.refresh()
		st := inst.statusSnapshot()
		snapshot, err := json.Marshal(map[string]any{
			"type":        "snapshot",
			"instance_id": inst.Id(),
			"ts":          time.Now().Unix(),
			"data": map[string]any{
				"running": st.Status,
				"players": st.Players,
				"session": sessionEventPayload(st.Session),
				"positions": inst.positionsSnapshot(),
			},
		})
		if err == nil {
			fmt.Fprintf(c.Writer, "data: %s\n\n", snapshot)
		}
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case msg := <-ch:
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}
