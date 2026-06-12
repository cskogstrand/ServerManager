package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type ServerStatus struct {
	Status   bool   `json:"status"`
	PublicIp string `json:"public_ip"`
	Players  int    `json:"players"`
	Session  SessionInfo
}

var publicIp string

func (inst *Instance) cspTrackBase() string {
	return filepath.Join(inst.Dir(), "content", "tracks", "csp")
}

func (inst *Instance) cspTrackFolder() string {
	return filepath.Join(inst.cspTrackBase(), *inst.Cr.track.Key, *inst.Cr.track.Config)
}

func (inst *Instance) ensureCspTrackAliases() {
	if !inst.Cr.cspRequired {
		return
	}

	if err := os.MkdirAll(inst.cspTrackFolder(), os.ModePerm); err != nil {
		log.Print("Could not create CSP track folder: ", err)
	}

	if inst.Cr.cspVersion != "" {
		if err := os.MkdirAll(filepath.Join(inst.cspTrackBase(), inst.Cr.cspVersion), os.ModePerm); err != nil {
			log.Print("Could not create CSP version folder: ", err)
		}
	}

	if inst.Cr.cspLetter != "" {
		if err := os.MkdirAll(filepath.Join(inst.cspTrackBase(), inst.Cr.cspLetter), os.ModePerm); err != nil {
			log.Print("Could not create CSP alias folder: ", err)
		}
	}
}

func (inst *Instance) refresh() {
	running := inst.isRunning()
	inst.mu.Lock()
	inst.Status.Status = running
	inst.Status.PublicIp = publicIp
	inst.mu.Unlock()
}

func updatePublicIp() {
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				res, err := http.Get("https://api.ipify.org")
				if err != nil {
					log.Print("Failed to query IP: ", err)
					continue
				}
				ip, err := io.ReadAll(res.Body)
				if err != nil {
					log.Print("Can not read response body: ", err)
				}
				publicIp = string(ip)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func (inst *Instance) serverChangeTrack() {
	log.Print("Kicking players for track change")
	// Haven't found a cleaner way to notify the users the track is about to change but to kick them
	for i := range inst.Cr.maxClients {
		inst.Udp.WriteKickUser(i)
	}
	time.Sleep(3 * time.Second)

	inst.stop()
	// Repeat mode re-runs the same event, so the queue is never consumed and
	// nothing is marked finished.
	if _, repeat := inst.repeatEventId(); !repeat {
		val := 1
		inst.Cr.serverEvent.Finished = &val
		Dba.updateServerEvent(inst.Cr.serverEvent)
	}
	if inst.serverApplyTrack() {
		inst.start()
	} else {
		log.Print("End")
	}
}

func (inst *Instance) serverApplyTrack() bool {
	// Repeat mode: re-apply the pinned event regardless of the manual queue.
	if eventId, repeat := inst.repeatEventId(); repeat {
		se, err := Dba.selectServerEventForEvent(eventId)
		if err != nil {
			log.Print("Repeat event unavailable for instance ", inst.Name(), ": ", err)
			return false
		}
		return applyServerEvent(inst, se)
	}

	nextevents, err := Dba.selectServerEvents(true, inst.Id())

	if err != nil {
		log.Print("Database error: ", err)
	}

	if len(nextevents) == 0 {
		log.Print("No events in queue for instance ", inst.Name())
		return false
	}
	return applyServerEvent(inst, nextevents[0])
}
