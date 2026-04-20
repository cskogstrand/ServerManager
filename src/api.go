package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type DashboardEventUpdate struct {
	EventID      int                         `json:"event_id"`
	Track        string                      `json:"track"`
	ClassID      int                         `json:"class_id"`
	TimeID       int                         `json:"time_id"`
	WeatherKey   string                      `json:"weather_key"`
	CspTime      string                      `json:"csp_time"`       // "HH:MM"
	CspTimeMul   int                         `json:"csp_time_multi"` // time-of-day multiplier
	ClassEntries []DashboardClassEntryUpdate `json:"class_entries"`
	RestartNow   bool                        `json:"restart_now"`
}

type DashboardClassEntryUpdate struct {
	CacheCarKey string `json:"cache_car_key"`
	SkinKey     string `json:"skin_key"`
}

func parseEntryListFile(path string) ([]DashboardClassEntryUpdate, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := make([]DashboardClassEntryUpdate, 0)
	current := DashboardClassEntryUpdate{}
	inCarSection := false

	flush := func() {
		if current.CacheCarKey != "" {
			entries = append(entries, current)
		}
		current = DashboardClassEntryUpdate{}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if inCarSection {
				flush()
			}
			section := strings.Trim(line, "[]")
			inCarSection = strings.HasPrefix(section, "CAR_")
			continue
		}
		if !inCarSection {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "MODEL":
			current.CacheCarKey = val
		case "SKIN":
			current.SkinKey = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if inCarSection {
		flush()
	}
	return entries, nil
}

func serveDemoSvg(c *gin.Context, label string, subtitle string) {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 360">
<rect width="640" height="360" fill="#0f172a"/>
<rect x="18" y="18" width="604" height="324" rx="18" fill="#111827" stroke="#334155" stroke-width="2"/>
<text x="36" y="86" fill="#e2e8f0" font-family="Arial, sans-serif" font-size="30" font-weight="700">%s</text>
<text x="36" y="122" fill="#94a3b8" font-family="Arial, sans-serif" font-size="18">%s</text>
<rect x="36" y="258" width="180" height="14" rx="7" fill="#1e293b"/>
<rect x="36" y="286" width="280" height="14" rx="7" fill="#1e293b"/>
</svg>`, label, subtitle)
	c.Data(http.StatusOK, "image/svg+xml", []byte(svg))
}

func serverStatusPayload() gin.H {
	Status.refresh()

	sessionType := "Unknown"
	switch Status.Session.typ {
	case 0:
		sessionType = "Booking"
	case 1:
		sessionType = "Practice"
	case 2:
		sessionType = "Qualify"
	case 3:
		sessionType = "Race"
	}

	currentEvent := gin.H{
		"id":         0,
		"category":   "",
		"track":      "",
		"track_key":  "",
		"track_config": "",
		"difficulty": "",
		"session":    "",
		"class":      "",
		"class_id":   0,
		"time":       "",
		"time_id":    0,
		"weather":    "",
		"weather_key": "",
		"started_at": int64(0),
		"finished":   0,
	}
	if Cr.serverEvent.Id != nil {
		currentEvent["id"] = *Cr.serverEvent.Id
	}
	if Cr.serverEvent.UserEvent.CategoryName != nil {
		currentEvent["category"] = *Cr.serverEvent.UserEvent.CategoryName
	}
	if Cr.serverEvent.UserEvent.TrackName != nil {
		currentEvent["track"] = *Cr.serverEvent.UserEvent.TrackName
	}
	if Cr.serverEvent.UserEvent.DifficultyName != nil {
		currentEvent["difficulty"] = *Cr.serverEvent.UserEvent.DifficultyName
	}
	if Cr.serverEvent.UserEvent.CacheTrackKey != nil {
		currentEvent["track_key"] = *Cr.serverEvent.UserEvent.CacheTrackKey
	}
	if Cr.serverEvent.UserEvent.CacheTrackConfig != nil {
		currentEvent["track_config"] = *Cr.serverEvent.UserEvent.CacheTrackConfig
	}
	if Cr.serverEvent.UserEvent.SessionName != nil {
		currentEvent["session"] = *Cr.serverEvent.UserEvent.SessionName
	}
	if Cr.serverEvent.UserEvent.ClassName != nil {
		currentEvent["class"] = *Cr.serverEvent.UserEvent.ClassName
	}
	if Cr.serverEvent.UserEvent.ClassId != nil {
		currentEvent["class_id"] = *Cr.serverEvent.UserEvent.ClassId
	}
	if Cr.serverEvent.UserEvent.TimeName != nil {
		currentEvent["time"] = *Cr.serverEvent.UserEvent.TimeName
	}
	if Cr.serverEvent.UserEvent.TimeId != nil {
		currentEvent["time_id"] = *Cr.serverEvent.UserEvent.TimeId
	}
	if Cr.serverEvent.StartedAt != nil {
		currentEvent["started_at"] = *Cr.serverEvent.StartedAt
	}
	if Cr.serverEvent.Finished != nil {
		currentEvent["finished"] = *Cr.serverEvent.Finished
	}
	if Cr.serverEvent.UserEvent.Id != nil {
		event, err := Dba.selectEvent(*Cr.serverEvent.UserEvent.Id)
		if err == nil && event.TimeId != nil {
			currentEvent["time_id"] = *event.TimeId
			tim, err := Dba.selectTimeWeather(*event.TimeId)
			if err == nil && len(tim.Weathers) > 0 {
				weather := tim.Weathers[0]
				if weather.Name != nil {
					currentEvent["weather"] = *weather.Name
				}
				if weather.Graphics != nil {
					currentEvent["weather_key"] = *weather.Graphics
				}
			}
		}
	}

	return gin.H{
		"is_running": isRunning(),
		"text":       getContent(),
		"players":    Status.Players,
		"public_ip":  Status.PublicIp,
		"tmp_loc":    TempFolder,
		"cfg_path":   filepath.Join(TempFolder, "cfg", "server_cfg.ini"),
		"entry_path": filepath.Join(TempFolder, "cfg", "entry_list.ini"),
		"session": gin.H{
			"name":                 Status.Session.name,
			"type":                 sessionType,
			"index":                Status.Session.sessionIndex,
			"current_session_index": Status.Session.currentSessionIndex,
			"session_count":        Status.Session.sessionCount,
			"track":                Status.Session.track,
			"track_config":         Status.Session.trackConfig,
			"server_name":          Status.Session.serverName,
			"time":                 Status.Session.time,
			"laps":                 Status.Session.laps,
			"wait_time":            Status.Session.waitTime,
			"ambient_temp":         Status.Session.ambientTemp,
			"road_temp":            Status.Session.roadTemp,
			"weather_graphics":     Status.Session.weatherGraphics,
			"elapsed_ms":           Status.Session.elapsedMs,
		},
		"current_event": currentEvent,
		"current_cars": func() []DashboardClassEntryUpdate {
			entries, err := parseEntryListFile(filepath.Join(TempFolder, "cfg", "entry_list.ini"))
			if err != nil {
				return []DashboardClassEntryUpdate{}
			}
			return entries
		}(),
	}
}

func applyServerEvent(serverEvent ServerEvent) bool {
	if serverEvent.UserEvent.Id == nil {
		log.Print("Cannot apply server event without event id")
		return false
	}

	Cr.serverEvent = serverEvent
	Cr.renderIni(*serverEvent.UserEvent.Id)
	Cr.writeIni()

	tm := time.Now().Unix()
	serverEvent.StartedAt = &tm
	serverEvent.ServerCfg = &Cr.serverCfgResult
	serverEvent.EntryList = &Cr.entryListResult

	if _, err := Dba.updateServerEvent(serverEvent); err != nil {
		log.Print("Could not update server event: ", err)
	}

	exec := "acServer"
	if runtime.GOOS == "windows" {
		exec = "acServer.exe"
	}
	Zf.ExtractFile(Zf.FindZipFile(exec), TempFolder)

	for _, e := range Cr.class.Entries {
		Zf.ExtractFiles(Zf.FindZipFiles("cars/"+*e.CacheCarKey+"/"), filepath.Join(TempFolder, "content"))
	}

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

	Zf.ExtractFile(Zf.FindZipFile("system/data/surfaces.ini"), filepath.Join(TempFolder))
	Zf.Close()

	return true
}

func noRoute(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"code":    "PAGE_NOT_FOUND",
		"message": "Page not found",
	})
}

func apiCarImage(c *gin.Context) {
	car := c.Param("car")
	skin := c.Param("skin")

	if strings.HasPrefix(car, "demo_") {
		serveDemoSvg(c, car, skin)
		return
	}

	var zf ZipFile
	zi := zf.FindZipFile("cars/" + car + "/skins/" + skin + "/preview.jpg")
	if zi != nil {
		r, err := zi.Open()
		if err != nil {
			log.Print("Cannot open car preview skin file in zipfile: ", err)
		}
		defer r.Close()
		c.DataFromReader(http.StatusOK, int64(zi.UncompressedSize64), "image/jpg", r, nil)
	} else {
		noRoute(c)
	}
	zf.Close()
}

func apiCar(c *gin.Context) {
	key := c.Param("key")

	cardata, err := Dba.selectCacheCar(key)

	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var power []any
	var torque []any
	var labels []any

	for _, l := range cardata.Power {
		labels = append(labels, l[0])
	}
	for _, p := range cardata.Power {
		power = append(power, p[1])
	}
	for _, t := range cardata.Torque {
		torque = append(torque, t[1])
	}

	c.PureJSON(http.StatusOK, gin.H{
		"key":    key,
		"desc":   cardata.Desc,
		"power":  power,
		"torque": torque,
		"labels": labels,
	})
}

func apiTrackPreviewImage(c *gin.Context) {
	track := c.Param("track")
	config := c.Param("config")

	if strings.HasPrefix(track, "demo_") {
		subtitle := "Track preview"
		if config != "" {
			subtitle = "Layout: " + config
		}
		serveDemoSvg(c, track, subtitle)
		return
	}

	filePath := "tracks/" + track + "/preview.png"
	if config != "" {
		filePath = "tracks/" + track + "/" + config + "/preview.png"
	}

	var zf ZipFile
	zi := zf.FindZipFile(filePath)
	if zi != nil {
		r, err := zi.Open()
		if err != nil {
			log.Print("Could not open track preview from zipfile", err)
		}
		defer r.Close()
		c.DataFromReader(http.StatusOK, int64(zi.UncompressedSize64), "image/png", r, nil)
	} else {
		noRoute(c)
	}
	zf.Close()
}

func apiTrackOutlineImage(c *gin.Context) {
	track := c.Param("track")
	config := c.Param("config")

	if strings.HasPrefix(track, "demo_") {
		serveDemoSvg(c, track, "Outline")
		return
	}

	filePath := "tracks/" + track + "/outline.png"
	if config != "" {
		filePath = "tracks/" + track + "/" + config + "/outline.png"
	}

	var zf ZipFile
	zi := zf.FindZipFile(filePath)
	if zi != nil {
		r, err := zi.Open()
		if err != nil {
			log.Print("Could not open track outline from zipfile", err)
		}
		defer r.Close()
		c.DataFromReader(http.StatusOK, int64(zi.UncompressedSize64), "image/png", r, nil)
	} else {
		noRoute(c)
	}
	zf.Close()
}

func apiWeatherPreviewImage(c *gin.Context) {
	weather := c.Param("weather")

	if strings.HasPrefix(weather, "demo_") {
		serveDemoSvg(c, weather, "Weather preview")
		return
	}

	var zf ZipFile
	zi := zf.FindZipFile("weather/" + weather + "/preview.jpg")
	if zi != nil {
		r, err := zi.Open()
		if err != nil {
			log.Print("Could not open weather preview from zipfile", err)
		}
		defer r.Close()
		c.DataFromReader(http.StatusOK, int64(zi.UncompressedSize64), "image/jpg", r, nil)
	} else {
		noRoute(c)
	}
	zf.Close()
}

func apiDifficulty(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		noRoute(c)
		return
	}

	data, err := Dba.selectDifficulty(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if data.Id == nil {
		noRoute(c)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{
		"data": data,
	})
}

func apiSession(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		noRoute(c)
		return
	}

	data, err := Dba.selectSession(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if data.Id == nil {
		noRoute(c)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{
		"data": data,
	})
}

func apiClass(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		noRoute(c)
		return
	}

	data, err := Dba.selectClassEntries(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if data.Id == nil {
		noRoute(c)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{
		"data": data,
	})
}

func apiTime(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		noRoute(c)
		return
	}

	data, err := Dba.selectTimeWeather(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if data.Id == nil {
		noRoute(c)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{
		"data": data,
	})
}

func apiRecacheContent(c *gin.Context) {
	counts, err := refreshContentCounts()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errInstallPathNotConfigured) {
			status = http.StatusBadRequest
		}
		c.JSON(status, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.PureJSON(http.StatusOK, gin.H{
		"result":         "ok",
		"tracks_total":   counts.Tracks,
		"cars_total":     counts.Cars,
		"weathers_total": counts.Weathers,
	})
}

func apiContentUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxContentUploadSize)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Could not read the upload. Use a .zip archive smaller than 2 GB.",
		})
		return
	}

	kind := strings.TrimSpace(c.PostForm("kind"))
	overwrite := parseBoolFormValue(c.PostForm("overwrite"))

	basepath, err := Dba.basepath()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errInstallPathNotConfigured) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if strings.TrimSpace(basepath) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Set a valid Assetto Corsa installation path before uploading content.",
		})
		return
	}

	header, err := c.FormFile("archive")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Choose a .zip archive to upload.",
		})
		return
	}
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Only .zip archives are supported.",
		})
		return
	}

	tempFile, err := os.CreateTemp(TempFolder, "sm-upload-*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Could not prepare the upload.",
		})
		return
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Could not prepare the upload.",
		})
		return
	}
	defer os.Remove(tempPath)

	if err := c.SaveUploadedFile(header, tempPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Could not save the uploaded archive.",
		})
		return
	}

	result, err := importContentArchive(tempPath, basepath, kind, overwrite)
	if err != nil {
		var uploadErr contentUploadError
		if errors.As(err, &uploadErr) {
			c.JSON(uploadErr.Status, gin.H{
				"success": false,
				"message": uploadErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	counts, err := refreshContentCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{
		"success":        true,
		"kind":           kind,
		"imported_assets": result.AssetKeys,
		"imported_count": len(result.AssetKeys),
		"files_written":  result.FilesWritten,
		"tracks_total":   counts.Tracks,
		"cars_total":     counts.Cars,
		"weathers_total": counts.Weathers,
		"message":        fmt.Sprintf("Imported %d %s archive item(s): %s", len(result.AssetKeys), kind, strings.Join(result.AssetKeys, ", ")),
	})
}

func apiValidateInstallpath(c *gin.Context) {
	binary := "acServer"
	if runtime.GOOS == "windows" {
		binary = "acServer.exe"
	}
	path := filepath.Join(c.PostForm("path"), "server", binary)
	exists := true
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		exists = false
	}

	c.PureJSON(http.StatusOK, gin.H{
		"result": exists,
	})
}

func apiServerStart(c *gin.Context) {
	if !isRunning() {
		if Status.serverApplyTrack() {
			start()
			// Hang the request until the UDP Server becomes online
			for start := time.Now(); time.Since(start) < time.Minute; {
				if Udp.online {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	c.PureJSON(http.StatusOK, serverStatusPayload())
}

func apiServerStop(c *gin.Context) {
	stop()
	c.PureJSON(http.StatusOK, serverStatusPayload())
}

func apiServerStatus(c *gin.Context) {
	c.PureJSON(http.StatusOK, serverStatusPayload())
}

func apiServerUpdateCurrentEvent(c *gin.Context) {
	var payload DashboardEventUpdate
	if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	event, err := Dba.selectEvent(payload.EventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if payload.Track != "" {
		track := strings.SplitN(payload.Track, ":", 2)
		trackKey := track[0]
		trackConfig := ""
		if len(track) > 1 {
			trackConfig = track[1]
		}
		event.CacheTrackKey = &trackKey
		event.CacheTrackConfig = &trackConfig
	}
	if payload.ClassID > 0 {
		event.ClassId = &payload.ClassID
	}
	if payload.TimeID > 0 {
		event.TimeId = &payload.TimeID
	}

	if _, err := Dba.updateEvent(event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if len(payload.ClassEntries) > 0 && event.ClassId != nil {
		cls, err := Dba.selectClassEntries(*event.ClassId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		entries := make([]UserClassEntry, 0, len(payload.ClassEntries))
		for _, ent := range payload.ClassEntries {
			if ent.CacheCarKey == "" || ent.SkinKey == "" {
				continue
			}
			carKey := ent.CacheCarKey
			skinKey := ent.SkinKey
			entries = append(entries, UserClassEntry{
				CacheCarKey: &carKey,
				SkinKey:     &skinKey,
			})
		}
		cls.Entries = entries

		if _, err := Dba.updateClass(cls); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	if payload.WeatherKey != "" && event.TimeId != nil {
		tim, err := Dba.selectTimeWeather(*event.TimeId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		if len(tim.Weathers) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Current time preset does not contain any weather panels",
			})
			return
		}

		tim.Weathers[0].Graphics = &payload.WeatherKey
		if payload.CspTime != "" {
			t := payload.CspTime
			tim.Weathers[0].CspTime = &t
		}
		if payload.CspTimeMul > 0 {
			m := payload.CspTimeMul
			tim.Weathers[0].CspTimeOfDayMulti = &m
		}
		if _, err := Dba.updateTime(tim); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	restarted := false
	if payload.RestartNow && isRunning() && Cr.serverEvent.UserEvent.Id != nil && *Cr.serverEvent.UserEvent.Id == payload.EventID {
		serverEvent := Cr.serverEvent
		if serverEvent.Id != nil {
			updatedServerEvent, err := Dba.selectServerEvent(*serverEvent.Id)
			if err == nil {
				serverEvent = updatedServerEvent
			}
		}
		stop()
		if applyServerEvent(serverEvent) {
			start()
			restarted = true
		}
	}

	response := serverStatusPayload()
	response["success"] = true
	response["restart_required"] = !restarted && isRunning()
	response["restarted"] = restarted
	c.PureJSON(http.StatusOK, response)
}

func apiServerLogfile(c *gin.Context) {
	logpath := filepath.Join(ConfigFolder, "logfile.log")
	c.FileAttachment(logpath, "logfile.log")
}

func apiServerSmdata(c *gin.Context) {
	smdata := filepath.Join(ConfigFolder, "smdata.db")
	c.FileAttachment(smdata, "smdata.db")
}

func apiServerSmcontent(c *gin.Context) {
	smcontent := filepath.Join(ConfigFolder, "smcontent.zip")
	c.FileAttachment(smcontent, "smcontent.zip")
}

// Warning: non determistic as some of the items can be randomized/shuffled.
func apiEntryList(c *gin.Context) {
	id := c.Query("id")
	idInt, _ := strconv.Atoi(id)

	Cr.renderIni(idInt)
	c.String(http.StatusOK, Cr.entryListResult)
}

func apiServerCfg(c *gin.Context) {
	id := c.Query("id")
	idInt, _ := strconv.Atoi(id)

	Cr.renderIni(idInt)
	c.String(http.StatusOK, Cr.serverCfgResult)
}

func apiQueueMoveUp(c *gin.Context) {
	id := c.Param("id")
	idInt, _ := strconv.Atoi(id)

	Dba.updateServerEventMoveUp(idInt)

	c.String(http.StatusOK, "ok")
}

func apiQueueMoveDown(c *gin.Context) {
	id := c.Param("id")
	idInt, _ := strconv.Atoi(id)

	Dba.updateServerEventMoveDown(idInt)

	c.String(http.StatusOK, "ok")
}

func apiQueueSkipEvent(c *gin.Context) {
	Status.serverChangeTrack()
	c.String(http.StatusOK, "ok")
}

func apiQueueClearCompleted(c *gin.Context) {
	Dba.deleteServerEventsCompleted()
	c.String(http.StatusOK, "ok")
}
