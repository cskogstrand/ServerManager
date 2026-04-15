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

func cspTrackBase() string {
	return filepath.Join(TempFolder, "content", "tracks", "csp")
}

func cspTrackFolder() string {
	return filepath.Join(cspTrackBase(), *Cr.track.Key, *Cr.track.Config)
}

func ensureCspTrackAliases() {
	if !Cr.cspRequired {
		return
	}

	if err := os.MkdirAll(cspTrackFolder(), os.ModePerm); err != nil {
		log.Print("Could not create CSP track folder: ", err)
	}

	if Cr.cspVersion != "" {
		if err := os.MkdirAll(filepath.Join(cspTrackBase(), Cr.cspVersion), os.ModePerm); err != nil {
			log.Print("Could not create CSP version folder: ", err)
		}
	}

	if Cr.cspLetter != "" {
		if err := os.MkdirAll(filepath.Join(cspTrackBase(), Cr.cspLetter), os.ModePerm); err != nil {
			log.Print("Could not create CSP alias folder: ", err)
		}
	}
}

func (stats ServerStatus) refresh() {
	Status.Status = isRunning()
	Status.PublicIp = publicIp
}

func (status ServerStatus) updatePublicIp() {
	ticker := time.NewTicker(5 * time.Minute)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				res, err := http.Get("https://api.ipify.org")
				if err != nil {
					log.Print("Failed to query IP: ", err)
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

func (status ServerStatus) serverChangeTrack() {
	log.Print("Kicking players for track change")
	// Haven't found a cleaner way to notify the users the track is about to change but to kick them
	for i := range Cr.maxClients {
		Udp.WriteKickUser(i)
	}
	time.Sleep(3 * time.Second)

	stop()
	val := 1
	Cr.serverEvent.Finished = &val
	Dba.updateServerEvent(Cr.serverEvent)
	if Status.serverApplyTrack() {
		start()
	} else {
		log.Print("End")
	}
}

func (status ServerStatus) serverApplyTrack() bool {
	nextevents, err := Dba.selectServerEvents(true)

	if err != nil {
		log.Print("Database error: ", err)
	}

	if len(nextevents) == 0 {
		log.Print("No events in queue")
		return false
	}
	return applyServerEvent(nextevents[0])
}
