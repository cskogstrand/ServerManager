package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
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
	Count       int    `json:"count"`
}

// instanceFromRequest resolves the target instance from the :instance path
// param or ?instance= query, falling back to the default instance so the
// pre-multi-server endpoints keep working unchanged.
func instanceFromRequest(c *gin.Context) (*Instance, error) {
	idStr := c.Param("instance")
	if idStr == "" {
		idStr = c.Query("instance")
	}
	id, _ := strconv.Atoi(idStr)
	return instanceById(id)
}

func apiInstanceError(c *gin.Context, err error) {
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": err.Error(),
	})
}

func apiContentJobsActive(c *gin.Context) {
	c.PureJSON(http.StatusOK, gin.H{
		"jobs": ContentJobs.ListVisible(),
	})
}

func apiContentJob(c *gin.Context) {
	job, ok := ContentJobs.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Content job not found.",
		})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{
		"job": job,
	})
}

func runContentURLImportJob(jobID string, archiveURL string, sourceName string, basepath string, kind string, overwrite bool) {
	ContentJobs.Update(jobID, func(job *ContentJob) {
		job.Status = "running"
		job.Phase = "downloading"
		job.Message = "Preparing download..."
		job.Progress = 5
	})

	tempPath, err := newTempArchiveFile(sourceName)
	if err != nil {
		ContentJobs.Update(jobID, func(job *ContentJob) {
			job.Status = "failed"
			job.Phase = "failed"
			job.Message = "Could not prepare the background import."
			job.Progress = 0
			job.FinishedAt = time.Now().Unix()
		})
		return
	}
	defer os.Remove(tempPath)

	resolvedName, err := downloadContentArchive(archiveURL, tempPath, func(downloaded int64, total int64) {
		progress := 20
		if total > 0 {
			progress = 10 + int(float64(downloaded)/float64(total)*55)
			if progress > 65 {
				progress = 65
			}
		}
		ContentJobs.Update(jobID, func(job *ContentJob) {
			job.Status = "running"
			job.Phase = "downloading"
			job.Message = buildContentJobDownloadMessage(downloaded, total)
			job.Progress = progress
			job.Downloaded = downloaded
			job.DownloadTotal = total
		})
	})
	if err != nil {
		msg := err.Error()
		var uploadErr contentUploadError
		if errors.As(err, &uploadErr) {
			msg = uploadErr.Message
		}
		ContentJobs.Update(jobID, func(job *ContentJob) {
			job.Status = "failed"
			job.Phase = "failed"
			job.Message = msg
			job.Progress = 0
			job.FinishedAt = time.Now().Unix()
		})
		return
	}
	if resolvedName != "" {
		sourceName = resolvedName
		ContentJobs.Update(jobID, func(job *ContentJob) {
			job.SourceName = resolvedName
		})
	}

	ContentJobs.Update(jobID, func(job *ContentJob) {
		job.Status = "running"
		job.Phase = "extracting"
		job.Message = "Archive downloaded. Importing content..."
		job.Progress = 72
	})

	result, err := importContentArchive(tempPath, sourceName, basepath, kind, overwrite)
	if err != nil {
		msg := err.Error()
		var uploadErr contentUploadError
		if errors.As(err, &uploadErr) {
			msg = uploadErr.Message
		}
		ContentJobs.Update(jobID, func(job *ContentJob) {
			job.Status = "failed"
			job.Phase = "failed"
			job.Message = msg
			job.Progress = 72
			job.FinishedAt = time.Now().Unix()
		})
		return
	}

	ContentJobs.Update(jobID, func(job *ContentJob) {
		job.Status = "running"
		job.Phase = "recaching"
		job.Message = "Archive imported. Rebuilding the content cache..."
		job.Progress = 90
		job.FilesWritten = result.FilesWritten
		job.ImportedAssets = result.AssetKeys
	})

	counts, err := refreshContentCounts()
	if err != nil {
		ContentJobs.Update(jobID, func(job *ContentJob) {
			job.Status = "failed"
			job.Phase = "failed"
			job.Message = err.Error()
			job.Progress = 90
			job.FinishedAt = time.Now().Unix()
		})
		return
	}

	ContentJobs.Update(jobID, func(job *ContentJob) {
		job.Status = "completed"
		job.Phase = "completed"
		job.Message = fmt.Sprintf("Imported %d %s archive item(s): %s", len(result.AssetKeys), kind, strings.Join(result.AssetKeys, ", "))
		job.Progress = 100
		job.FilesWritten = result.FilesWritten
		job.ImportedAssets = result.AssetKeys
		job.TracksTotal = counts.Tracks
		job.CarsTotal = counts.Cars
		job.WeathersTotal = counts.Weathers
		job.FinishedAt = time.Now().Unix()
	})
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

func serverStatusPayload(inst *Instance) gin.H {
	inst.refresh()
	st := inst.statusSnapshot()

	sessionType := "Unknown"
	switch st.Session.typ {
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
	if inst.Cr.serverEvent.Id != nil {
		currentEvent["id"] = *inst.Cr.serverEvent.Id
	}
	if inst.Cr.serverEvent.UserEvent.CategoryName != nil {
		currentEvent["category"] = *inst.Cr.serverEvent.UserEvent.CategoryName
	}
	if inst.Cr.serverEvent.UserEvent.TrackName != nil {
		currentEvent["track"] = *inst.Cr.serverEvent.UserEvent.TrackName
	}
	if inst.Cr.serverEvent.UserEvent.DifficultyName != nil {
		currentEvent["difficulty"] = *inst.Cr.serverEvent.UserEvent.DifficultyName
	}
	if inst.Cr.serverEvent.UserEvent.CacheTrackKey != nil {
		currentEvent["track_key"] = *inst.Cr.serverEvent.UserEvent.CacheTrackKey
	}
	if inst.Cr.serverEvent.UserEvent.CacheTrackConfig != nil {
		currentEvent["track_config"] = *inst.Cr.serverEvent.UserEvent.CacheTrackConfig
	}
	if inst.Cr.serverEvent.UserEvent.SessionName != nil {
		currentEvent["session"] = *inst.Cr.serverEvent.UserEvent.SessionName
	}
	if inst.Cr.serverEvent.UserEvent.ClassName != nil {
		currentEvent["class"] = *inst.Cr.serverEvent.UserEvent.ClassName
	}
	if inst.Cr.serverEvent.UserEvent.ClassId != nil {
		currentEvent["class_id"] = *inst.Cr.serverEvent.UserEvent.ClassId
	}
	if inst.Cr.serverEvent.UserEvent.TimeName != nil {
		currentEvent["time"] = *inst.Cr.serverEvent.UserEvent.TimeName
	}
	if inst.Cr.serverEvent.UserEvent.TimeId != nil {
		currentEvent["time_id"] = *inst.Cr.serverEvent.UserEvent.TimeId
	}
	if inst.Cr.serverEvent.StartedAt != nil {
		currentEvent["started_at"] = *inst.Cr.serverEvent.StartedAt
	}
	if inst.Cr.serverEvent.Finished != nil {
		currentEvent["finished"] = *inst.Cr.serverEvent.Finished
	}
	if inst.Cr.serverEvent.UserEvent.Id != nil {
		event, err := Dba.selectEvent(*inst.Cr.serverEvent.UserEvent.Id)
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

	dir := inst.Dir()
	return gin.H{
		"instance_id":   inst.Id(),
		"instance_name": inst.Name(),
		"is_running":    st.Status,
		"text":          inst.logContent(),
		"players":       st.Players,
		"public_ip":     st.PublicIp,
		"tmp_loc":       dir,
		"cfg_path":      filepath.Join(dir, "cfg", "server_cfg.ini"),
		"entry_path":    filepath.Join(dir, "cfg", "entry_list.ini"),
		"session": gin.H{
			"name":                 st.Session.name,
			"type":                 sessionType,
			"index":                st.Session.sessionIndex,
			"current_session_index": st.Session.currentSessionIndex,
			"session_count":        st.Session.sessionCount,
			"track":                st.Session.track,
			"track_config":         st.Session.trackConfig,
			"server_name":          st.Session.serverName,
			"time":                 st.Session.time,
			"laps":                 st.Session.laps,
			"wait_time":            st.Session.waitTime,
			"ambient_temp":         st.Session.ambientTemp,
			"road_temp":            st.Session.roadTemp,
			"weather_graphics":     st.Session.weatherGraphics,
			"elapsed_ms":           st.Session.elapsedMs,
		},
		"current_event": currentEvent,
		"current_cars": func() []DashboardClassEntryUpdate {
			entries, err := parseEntryListFile(filepath.Join(dir, "cfg", "entry_list.ini"))
			if err != nil {
				return []DashboardClassEntryUpdate{}
			}
			return entries
		}(),
	}
}

func applyServerEvent(inst *Instance, serverEvent ServerEvent) bool {
	if serverEvent.UserEvent.Id == nil {
		log.Print("Cannot apply server event without event id")
		return false
	}

	dir := inst.Dir()
	contentDir := filepath.Join(dir, "content")

	inst.Cr.serverEvent = serverEvent
	inst.Cr.renderIni(*serverEvent.UserEvent.Id, inst.Conf)
	inst.Cr.writeIni(dir)

	tm := time.Now().Unix()
	serverEvent.StartedAt = &tm
	serverEvent.ServerCfg = &inst.Cr.serverCfgResult
	serverEvent.EntryList = &inst.Cr.entryListResult

	if _, err := Dba.updateServerEvent(serverEvent); err != nil {
		log.Print("Could not update server event: ", err)
	}

	exec := "acServer"
	if runtime.GOOS == "windows" {
		exec = "acServer.exe"
	}

	var zf ZipFile
	zf.ExtractFile(zf.FindZipFile(exec), dir)

	extractedCars := make(map[string]bool)
	for _, e := range inst.Cr.class.Entries {
		if extractedCars[*e.CacheCarKey] {
			continue
		}
		extractedCars[*e.CacheCarKey] = true
		zf.ExtractFiles(zf.FindZipFiles("cars/"+*e.CacheCarKey+"/"), contentDir)
	}

	track := inst.Cr.track
	if *track.Config == "" {
		if inst.Cr.cspRequired {
			inst.ensureCspTrackAliases()
			zf.ExtractFileToSubfolder(zf.FindZipFile("tracks/"+*track.Key+"/models.ini"), inst.cspTrackFolder())
			zf.ExtractFileToSubfolder(zf.FindZipFile("tracks/"+*track.Key+"/data/drs_zones.ini"), filepath.Join(inst.cspTrackFolder(), "data"))
			zf.ExtractFileToSubfolder(zf.FindZipFile("tracks/"+*track.Key+"/data/surfaces.ini"), filepath.Join(inst.cspTrackFolder(), "data"))
		} else {
			zf.ExtractFile(zf.FindZipFile("tracks/"+*track.Key+"/models.ini"), contentDir)
			zf.ExtractFile(zf.FindZipFile("tracks/"+*track.Key+"/data/drs_zones.ini"), contentDir)
			zf.ExtractFile(zf.FindZipFile("tracks/"+*track.Key+"/data/surfaces.ini"), contentDir)
		}
	} else {
		if inst.Cr.cspRequired {
			inst.ensureCspTrackAliases()
			zf.ExtractFileToSubfolder(zf.FindZipFile("tracks/"+*track.Key+"/models_"+*track.Config+".ini"), inst.cspTrackFolder())
			zf.ExtractFileToSubfolder(zf.FindZipFile("tracks/"+*track.Key+"/"+*track.Config+"/data/drs_zones.ini"), filepath.Join(inst.cspTrackFolder(), "data"))
			zf.ExtractFileToSubfolder(zf.FindZipFile("tracks/"+*track.Key+"/"+*track.Config+"/data/surfaces.ini"), filepath.Join(inst.cspTrackFolder(), "data"))
		} else {
			zf.ExtractFile(zf.FindZipFile("tracks/"+*track.Key+"/models_"+*track.Config+".ini"), contentDir)
			zf.ExtractFile(zf.FindZipFile("tracks/"+*track.Key+"/"+*track.Config+"/data/drs_zones.ini"), contentDir)
			zf.ExtractFile(zf.FindZipFile("tracks/"+*track.Key+"/"+*track.Config+"/data/surfaces.ini"), contentDir)
		}
	}

	zf.ExtractFile(zf.FindZipFile("system/data/surfaces.ini"), dir)
	zf.Close()

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
			"message": "Could not read the upload. Use a supported archive smaller than 2 GB.",
		})
		return
	}

	kind := strings.TrimSpace(c.PostForm("kind"))
	overwrite := parseBoolFormValue(c.PostForm("overwrite"))
	archiveURL := strings.TrimSpace(c.PostForm("archive_url"))
	var header *multipart.FileHeader

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

	sourceName := archiveURL
	if archiveURL == "" {
		header, err = c.FormFile("archive")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Choose an archive file or enter an archive URL.",
			})
			return
		}
		sourceName = header.Filename
	} else {
		parsedURL, err := validateArchiveURL(archiveURL)
		if err != nil {
			var uploadErr contentUploadError
			if errors.As(err, &uploadErr) {
				c.JSON(uploadErr.Status, gin.H{
					"success": false,
					"message": uploadErr.Message,
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "The archive URL is not valid.",
			})
			return
		}
		sourceName = parsedURL.Path
	}

	source := "upload"
	if archiveURL != "" {
		source = "url"
		job := ContentJobs.Create(kind, source, sourceName, archiveURL)
		go runContentURLImportJob(job.ID, archiveURL, sourceName, basepath, kind, overwrite)
		c.PureJSON(http.StatusAccepted, gin.H{
			"success": true,
			"async":   true,
			"job":     job,
			"message": "Background import started.",
		})
		return
	}

	if _, err := ensureSupportedArchiveName(header.Filename); err != nil {
		var uploadErr contentUploadError
		if errors.As(err, &uploadErr) {
			c.JSON(uploadErr.Status, gin.H{
				"success": false,
				"message": uploadErr.Message,
			})
			return
		}
		return
	}

	tempPath, err := newTempArchiveFile(sourceName)
	if err != nil {
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

	result, err := importContentArchive(tempPath, sourceName, basepath, kind, overwrite)
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
		"async":          false,
		"kind":           kind,
		"source":         source,
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
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	if !inst.isRunning() {
		if inst.serverApplyTrack() {
			inst.start()
			// Hang the request until the UDP Server becomes online
			for start := time.Now(); time.Since(start) < time.Minute; {
				if inst.Udp.online {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	c.PureJSON(http.StatusOK, serverStatusPayload(inst))
}

func apiServerStop(c *gin.Context) {
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	inst.stop()
	c.PureJSON(http.StatusOK, serverStatusPayload(inst))
}

func apiServerStatus(c *gin.Context) {
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	c.PureJSON(http.StatusOK, serverStatusPayload(inst))
}

func apiServerUpdateCurrentEvent(c *gin.Context) {
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

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
			count := ent.Count
			if count < 1 {
				count = 1
			}
			entries = append(entries, UserClassEntry{
				CacheCarKey: &carKey,
				SkinKey:     &skinKey,
				Count:       &count,
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
	if payload.RestartNow && inst.isRunning() && inst.Cr.serverEvent.UserEvent.Id != nil && *inst.Cr.serverEvent.UserEvent.Id == payload.EventID {
		serverEvent := inst.Cr.serverEvent
		if serverEvent.Id != nil {
			updatedServerEvent, err := Dba.selectServerEvent(*serverEvent.Id)
			if err == nil {
				serverEvent = updatedServerEvent
			}
		}
		inst.stop()
		if applyServerEvent(inst, serverEvent) {
			inst.start()
			restarted = true
		}
	}

	response := serverStatusPayload(inst)
	response["success"] = true
	response["restart_required"] = !restarted && inst.isRunning()
	response["restarted"] = restarted
	c.PureJSON(http.StatusOK, response)
}

// apiServerReadiness aggregates the setup health the Server Setup page needs:
// install path, content cache, presets, events and per-instance port/queue
// state. The frontend turns these facts into pass/fail checks with links.
func apiServerReadiness(c *gin.Context) {
	cfg, err := Dba.selectConfig()
	if err != nil {
		apiDbError(c, err)
		return
	}

	installPath := ""
	if cfg.InstallPath != nil {
		installPath = *cfg.InstallPath
	}
	binary := "acServer"
	if runtime.GOOS == "windows" {
		binary = "acServer.exe"
	}
	acserverFound := false
	if strings.TrimSpace(installPath) != "" {
		if _, statErr := os.Stat(filepath.Join(installPath, "server", binary)); statErr == nil {
			acserverFound = true
		}
	}

	count := func(fn func() (int, error)) int {
		n, _ := fn()
		return n
	}
	tracks := count(func() (int, error) { l, e := Dba.selectCacheTracks(); return len(l), e })
	cars := count(func() (int, error) { l, e := Dba.selectCacheCars(); return len(l), e })
	weathers := count(func() (int, error) { l, e := Dba.selectCacheWeathers(); return len(l), e })

	filled := func(table string) int {
		l, _ := Dba.selectDropDownList(true, table)
		return len(l)
	}
	events, _ := Dba.selectEventList()

	// Detect a port reused across instances.
	portConflict := ""
	seen := map[int]string{}
	instances := make([]gin.H, 0)
	for _, inst := range Instances.All() {
		for _, p := range []*int{inst.Conf.UdpPort, inst.Conf.TcpPort, inst.Conf.HttpPort, inst.Conf.PluginPort, inst.Conf.PluginListenPort} {
			if p == nil {
				continue
			}
			// TCP and UDP game ports legitimately share a value within one instance.
			if owner, ok := seen[*p]; ok && owner != inst.Name() {
				portConflict = fmt.Sprintf("Port %d is used by both %q and %q", *p, owner, inst.Name())
			}
			seen[*p] = inst.Name()
		}
		pending, _ := Dba.selectServerEvents(true, inst.Id())
		runMode := runModeManualQueue
		if inst.Conf.RunMode != nil && *inst.Conf.RunMode != "" {
			runMode = *inst.Conf.RunMode
		}
		instances = append(instances, gin.H{
			"id":            inst.Id(),
			"name":          inst.Name(),
			"run_mode":      runMode,
			"queue_pending": len(pending),
			"is_running":    inst.isRunning(),
		})
	}

	cfgFilled := cfg.CfgFilled != nil && *cfg.CfgFilled == 1
	modFilled := cfg.ModFilled != nil && *cfg.ModFilled == 1

	c.PureJSON(http.StatusOK, gin.H{
		"install_path":    installPath,
		"acserver_found":  acserverFound,
		"cfg_filled":      cfgFilled,
		"mod_filled":      modFilled,
		"content":         gin.H{"tracks": tracks, "cars": cars, "weathers": weathers},
		"presets":         gin.H{"difficulties": filled("user_difficulty"), "sessions": filled("user_session"), "classes": filled("user_class"), "times": filled("user_time")},
		"events":          len(events),
		"instances":       instances,
		"port_conflict":   portConflict,
	})
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
// Previews render with a throwaway renderer so they never clobber the state
// of a running instance.
func apiEntryList(c *gin.Context) {
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	id := c.Query("id")
	idInt, _ := strconv.Atoi(id)

	cr := ConfigRenderer{}
	cr.renderIni(idInt, inst.Conf)
	c.String(http.StatusOK, cr.entryListResult)
}

func apiServerCfg(c *gin.Context) {
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	id := c.Query("id")
	idInt, _ := strconv.Atoi(id)

	cr := ConfigRenderer{}
	cr.renderIni(idInt, inst.Conf)
	c.String(http.StatusOK, cr.serverCfgResult)
}

// queueRowRepeatLocked reports whether the queue row's instance is in repeat
// mode, in which case its manual queue is frozen.
func queueRowRepeatLocked(rowId int) bool {
	se, err := Dba.selectServerEvent(rowId)
	if err != nil || se.InstanceId == nil {
		return false
	}
	return instanceInRepeatMode(*se.InstanceId)
}

func apiQueueMoveUp(c *gin.Context) {
	id := c.Param("id")
	idInt, _ := strconv.Atoi(id)

	if queueRowRepeatLocked(idInt) {
		apiError(c, http.StatusConflict, "repeat_locked", "This instance is in repeat mode. Switch it back to manual queue first.")
		return
	}

	Dba.updateServerEventMoveUp(idInt)

	c.String(http.StatusOK, "ok")
}

func apiQueueMoveDown(c *gin.Context) {
	id := c.Param("id")
	idInt, _ := strconv.Atoi(id)

	if queueRowRepeatLocked(idInt) {
		apiError(c, http.StatusConflict, "repeat_locked", "This instance is in repeat mode. Switch it back to manual queue first.")
		return
	}

	Dba.updateServerEventMoveDown(idInt)

	c.String(http.StatusOK, "ok")
}

func apiQueueSkipEvent(c *gin.Context) {
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	inst.serverChangeTrack()
	c.String(http.StatusOK, "ok")
}

// apiQueueReorder applies a full ordered id list for one instance's queue.
func apiQueueReorder(c *gin.Context) {
	var body struct {
		Instance int   `json:"instance"`
		Ids      []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiBadRequest(c, "Invalid request payload")
		return
	}
	if instanceInRepeatMode(body.Instance) {
		apiError(c, http.StatusConflict, "repeat_locked", "This instance is in repeat mode. Switch it back to manual queue first.")
		return
	}
	if err := Dba.updateServerEventOrder(body.Ids); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"ids": body.Ids})
}

func apiQueueClearCompleted(c *gin.Context) {
	Dba.deleteServerEventsCompleted()
	c.String(http.StatusOK, "ok")
}

func apiQueueAddEvent(c *gin.Context) {
	eventId, err := strconv.Atoi(c.Param("id"))
	if err != nil || eventId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid event id"})
		return
	}

	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	if _, repeat := inst.repeatEventId(); repeat {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "This instance is in repeat mode. Switch it back to manual queue first."})
		return
	}

	if _, err := Dba.insertServerEvent(eventId, inst.Id()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

func apiQueueAddCategory(c *gin.Context) {
	categoryId, err := strconv.Atoi(c.Param("id"))
	if err != nil || categoryId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid category id"})
		return
	}

	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		return
	}

	if _, repeat := inst.repeatEventId(); repeat {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "This instance is in repeat mode. Switch it back to manual queue first."})
		return
	}

	if _, err := Dba.insertServerEventCategory(categoryId, inst.Id()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

func apiInstances(c *gin.Context) {
	list := make([]gin.H, 0)
	for _, inst := range Instances.All() {
		inst.refresh()
		st := inst.statusSnapshot()

		runMode := runModeManualQueue
		if inst.Conf.RunMode != nil && *inst.Conf.RunMode != "" {
			runMode = *inst.Conf.RunMode
		}

		var repeatEvent gin.H
		if eventId, repeat := inst.repeatEventId(); repeat {
			repeatEvent = gin.H{"id": eventId}
			if se, err := Dba.selectServerEventForEvent(eventId); err == nil {
				repeatEvent["track"] = se.UserEvent.TrackName
				repeatEvent["category"] = se.UserEvent.CategoryName
				repeatEvent["class"] = se.UserEvent.ClassName
			}
		}

		list = append(list, gin.H{
			"id":                 inst.Id(),
			"name":               inst.Name(),
			"udp_port":           inst.Conf.UdpPort,
			"tcp_port":           inst.Conf.TcpPort,
			"http_port":          inst.Conf.HttpPort,
			"plugin_port":        inst.Conf.PluginPort,
			"plugin_listen_port": inst.Conf.PluginListenPort,
			"is_running":         st.Status,
			"players":            st.Players,
			"run_mode":           runMode,
			"repeat_event_id":    inst.Conf.RepeatEventId,
			"repeat_event":       repeatEvent,
		})
	}

	c.PureJSON(http.StatusOK, gin.H{"instances": list})
}

// instanceInRepeatMode reports whether the given queue mutation should be
// blocked because the instance is auto-repeating one event.
func instanceInRepeatMode(instanceId int) bool {
	if instanceId <= 0 {
		return false
	}
	inst := Instances.Get(instanceId)
	if inst == nil {
		return false
	}
	_, repeat := inst.repeatEventId()
	return repeat
}

// apiInstanceRunMode switches an instance between manual_queue and
// repeat_event. Switching to repeat pins repeat_event_id and (when running)
// applies it immediately; switching back to manual leaves the queue intact.
func apiInstanceRunMode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		apiBadRequest(c, "Invalid instance id")
		return
	}

	inst := Instances.Get(id)
	if inst == nil {
		apiNotFound(c)
		return
	}

	var body struct {
		RunMode       string `json:"run_mode"`
		RepeatEventId *int   `json:"repeat_event_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiBadRequest(c, "Invalid request payload")
		return
	}

	switch body.RunMode {
	case runModeManualQueue:
		if _, err := Dba.updateServerInstanceRunMode(id, runModeManualQueue, nil); err != nil {
			apiDbError(c, err)
			return
		}
	case runModeRepeatEvent:
		if body.RepeatEventId == nil || *body.RepeatEventId <= 0 {
			apiBadRequest(c, "repeat_event_id is required for repeat mode")
			return
		}
		if _, err := Dba.selectServerEventForEvent(*body.RepeatEventId); err != nil {
			apiBadRequest(c, "That event does not exist or is missing a preset")
			return
		}
		if _, err := Dba.updateServerInstanceRunMode(id, runModeRepeatEvent, body.RepeatEventId); err != nil {
			apiDbError(c, err)
			return
		}
	default:
		apiBadRequest(c, "run_mode must be manual_queue or repeat_event")
		return
	}

	if err := Instances.LoadFromDb(); err != nil {
		apiDbError(c, err)
		return
	}

	// If the instance is already running, apply the new mode on the next
	// rotation; restarting here would kick players unexpectedly. The dashboard
	// surfaces that a restart is needed.
	c.PureJSON(http.StatusOK, gin.H{"run_mode": body.RunMode, "repeat_event_id": body.RepeatEventId})
}

func apiInstanceCreate(c *gin.Context) {
	var si ServerInstance
	if err := c.ShouldBindJSON(&si); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	si.Id = nil

	if si.Name == nil || strings.TrimSpace(*si.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Instance name is required"})
		return
	}
	if si.UdpPort == nil || si.TcpPort == nil || si.HttpPort == nil || si.PluginPort == nil || si.PluginListenPort == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "All ports are required: udp_port, tcp_port, http_port, plugin_port, plugin_listen_port"})
		return
	}

	if err := Instances.validatePorts(si); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	id, err := Dba.insertServerInstance(si)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := Instances.LoadFromDb(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{"success": true, "id": id})
}

func apiInstanceUpdate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid instance id"})
		return
	}

	inst := Instances.Get(id)
	if inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Instance not found"})
		return
	}

	if inst.isRunning() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Stop the server before changing instance settings"})
		return
	}

	var si ServerInstance
	if err := c.ShouldBindJSON(&si); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	si.Id = &id

	if err := Instances.validatePorts(si); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if _, err := Dba.updateServerInstance(si); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := Instances.LoadFromDb(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

func apiInstanceDelete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid instance id"})
		return
	}

	inst := Instances.Get(id)
	if inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Instance not found"})
		return
	}

	if inst.isRunning() {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Stop the server before deleting the instance"})
		return
	}

	if len(Instances.All()) <= 1 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "Cannot delete the last remaining instance"})
		return
	}

	if _, err := Dba.deleteServerInstance(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := Instances.LoadFromDb(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.PureJSON(http.StatusOK, gin.H{"success": true})
}
