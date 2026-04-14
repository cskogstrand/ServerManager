package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	nextevent := nextevents[0]

	Cr.serverEvent = nextevent
	Cr.renderIni(*nextevent.UserEvent.Id)
	Cr.writeIni()

	tm := time.Now().Unix()
	nextevent.StartedAt = &tm

	nextevent.ServerCfg = &Cr.serverCfgResult
	nextevent.EntryList = &Cr.entryListResult

	Dba.updateServerEvent(nextevent)

	// Populate TempFolder
	exec := "acServer"
	if runtime.GOOS == "windows" {
		exec = "acServer.exe"
	}
	Zf.ExtractFile(Zf.FindZipFile(exec), TempFolder)

	// Extract cars
	for _, e := range Cr.class.Entries {
		Zf.ExtractFiles(Zf.FindZipFiles("cars/"+*e.CacheCarKey+"/"), filepath.Join(TempFolder, "content"))
	}

	// Extract track
	if *Cr.track.Config == "" {
		if Cr.cspRequired {
			ensureCspTrackAliases()
			Zf.ExtractFileToSubfolder(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/models.ini"), cspTrackFolder())
			Zf.ExtractFileToSubfolder(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/data/drs_zones.ini"), filepath.Join(cspTrackFolder(), "data"))
			Zf.ExtractFileToSubfolder(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/data/surfaces.ini"), filepath.Join(cspTrackFolder(), "data"))
		} else {
			Zf.ExtractFile(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/models.ini"), filepath.Join(TempFolder, "content"))
			Zf.ExtractFile(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/data/drs_zones.ini"), filepath.Join(TempFolder, "content"))
			Zf.ExtractFile(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/data/surfaces.ini"), filepath.Join(TempFolder, "content"))
		}
	} else {
		if Cr.cspRequired {
			ensureCspTrackAliases()
			Zf.ExtractFileToSubfolder(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/models_"+*Cr.track.Config+".ini"), cspTrackFolder())
			Zf.ExtractFileToSubfolder(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/"+*Cr.track.Config+"/data/drs_zones.ini"), filepath.Join(cspTrackFolder(), "data"))
			Zf.ExtractFileToSubfolder(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/"+*Cr.track.Config+"/data/surfaces.ini"), filepath.Join(cspTrackFolder(), "data"))
		} else {
			Zf.ExtractFile(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/models_"+*Cr.track.Config+".ini"), filepath.Join(TempFolder, "content"))
			Zf.ExtractFile(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/"+*Cr.track.Config+"/data/drs_zones.ini"), filepath.Join(TempFolder, "content"))
			Zf.ExtractFile(Zf.FindZipFile("tracks/"+*Cr.track.Key+"/"+*Cr.track.Config+"/data/surfaces.ini"), filepath.Join(TempFolder, "content"))
		}
	}

	// Extract surfaces.ini
	Zf.ExtractFile(Zf.FindZipFile("system/data/surfaces.ini"), filepath.Join(TempFolder))

	Zf.Close()

	return true
}
