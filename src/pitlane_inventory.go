package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RigGear struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name"`
	Notes string `json:"notes"`
}
type Rig struct {
	ID              int       `json:"id"`
	Revision        int       `json:"revision"`
	Name            string    `json:"name"`
	Location        string    `json:"location"`
	Display         string    `json:"display"`
	Notes           string    `json:"notes"`
	UsualDriverGUID string    `json:"usual_driver_guid"`
	UsualGuestID    *int      `json:"usual_guest_id"`
	Gear            []RigGear `json:"gear"`
}

func (r *Rig) validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || len(r.Name) > 160 {
		return errors.New("Give the rig a name of 1–160 characters")
	}
	switch r.Display {
	case "triple", "single", "wide", "ultrawide", "vr", "custom":
	default:
		return errors.New("Choose one of the six display arrangements")
	}
	if len(r.Notes) > 10000 || len(r.Location) > 500 || len(r.Gear) > 100 {
		return errors.New("Rig notes, location or gear list is too long")
	}
	seen := map[string]bool{}
	for _, g := range r.Gear {
		if g.ID == "" || len(g.ID) > 100 || seen[g.ID] || strings.TrimSpace(g.Name) == "" || len(g.Name) > 300 || len(g.Notes) > 2000 {
			return errors.New("Each gear item needs a unique ID and a name; keep notes under 2,000 characters")
		}
		seen[g.ID] = true
	}
	if r.UsualGuestID != nil && r.UsualDriverGUID != "" {
		return errors.New("Choose either a usual guest or account driver")
	}
	if r.Gear == nil {
		r.Gear = []RigGear{}
	}
	return nil
}
func (dba Dbaccess) rigs() ([]Rig, error) {
	rows, err := dba.db.Query("SELECT id,revision,name,location,display,notes,usual_driver_guid,usual_guest_id,gear FROM rig ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rigs := []Rig{}
	for rows.Next() {
		var r Rig
		var gear string
		if err = rows.Scan(&r.ID, &r.Revision, &r.Name, &r.Location, &r.Display, &r.Notes, &r.UsualDriverGUID, &r.UsualGuestID, &gear); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(gear), &r.Gear); err != nil {
			return nil, err
		}
		rigs = append(rigs, r)
	}
	return rigs, rows.Err()
}
func apiRigs(c *gin.Context) {
	rigs, err := Dba.rigs()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"rigs": rigs})
}
func apiRigSave(c *gin.Context) {
	var r Rig
	if c.ShouldBindJSON(&r) != nil {
		apiBadRequest(c, "Invalid rig")
		return
	}
	if err := r.validate(); err != nil {
		apiBadRequest(c, err.Error())
		return
	}
	if r.UsualGuestID != nil {
		var n int
		if err := Dba.db.QueryRow("SELECT count(*) FROM guest_driver WHERE id=?", *r.UsualGuestID).Scan(&n); err != nil {
			apiDbError(c, err)
			return
		}
		if n != 1 {
			apiBadRequest(c, "That guest no longer exists")
			return
		}
	}
	if r.UsualDriverGUID != "" {
		var n int
		if err := Dba.db.QueryRow("SELECT count(*) FROM driver WHERE guid=?", r.UsualDriverGUID).Scan(&n); err != nil {
			apiDbError(c, err)
			return
		}
		if n != 1 {
			apiBadRequest(c, "Choose an existing account driver")
			return
		}
	}
	gear, _ := json.Marshal(r.Gear)
	var result sql.Result
	var err error
	if c.Param("id") == "" {
		result, err = Dba.db.Exec("INSERT INTO rig(name,location,display,notes,usual_driver_guid,usual_guest_id,gear) VALUES(?,?,?,?,?,?,?)", r.Name, r.Location, r.Display, r.Notes, r.UsualDriverGUID, r.UsualGuestID, string(gear))
		if err == nil {
			id, _ := result.LastInsertId()
			r.ID = int(id)
			r.Revision = 1
		}
	} else {
		id, ok := pathId(c)
		if !ok {
			return
		}
		r.ID = id
		result, err = Dba.db.Exec("UPDATE rig SET name=?,location=?,display=?,notes=?,usual_driver_guid=?,usual_guest_id=?,gear=?,revision=revision+1 WHERE id=? AND revision=?", r.Name, r.Location, r.Display, r.Notes, r.UsualDriverGUID, r.UsualGuestID, string(gear), id, r.Revision)
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				apiError(c, 409, "revision_conflict", "This rig changed or was removed. Reload it before saving.")
				return
			}
			r.Revision++
		}
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			apiError(c, 409, "duplicate_name", "Another rig already has this name.")
		} else {
			apiDbError(c, err)
		}
		return
	}
	c.JSON(200, gin.H{"rig": r})
}
func apiRigDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var n int
	if err := Dba.db.QueryRow("SELECT count(*) FROM source_rig WHERE rig_id=?", id).Scan(&n); err != nil {
		apiDbError(c, err)
		return
	}
	if n > 0 {
		apiError(c, 409, "sources_attached", "Detach this rig’s cameras before removing the rig.")
		return
	}
	result, err := Dba.db.Exec("DELETE FROM rig WHERE id=?", id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	n64, _ := result.RowsAffected()
	if n64 == 0 {
		apiNotFound(c)
		return
	}
	c.Status(204)
}

type CameraSource struct {
	Version             string `json:"version"`
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Enabled             bool   `json:"enabled"`
	PlayerURL           string `json:"player_url"`
	StatusURL           string `json:"status_url,omitempty"`
	CaptureURL          string `json:"capture_url,omitempty"`
	RecordingConfigured bool   `json:"recording_configured"`
	DriverGUID          string `json:"driver_guid"`
	InstanceID          *int   `json:"instance_id"`
	RigID               *int   `json:"rig_id"`
}

func (dba Dbaccess) cameraSources() ([]CameraSource, error) {
	// Compatibility projection: legacy URLs remain editable by their original APIs.
	// No identity migration, inferred rig, or duplicate stream configuration.
	streams, err := dba.selectDriverStreams()
	if err != nil {
		return nil, err
	}
	sources := []CameraSource{}
	for _, s := range streams {
		sources = append(sources, CameraSource{ID: fmt.Sprint("driver:", *s.Id), Name: derefOrEmpty(s.DisplayName), Enabled: isEnabled(s.Enabled), PlayerURL: derefOrEmpty(s.StreamEmbedUrl), StatusURL: derefOrEmpty(s.StreamStatusUrl), CaptureURL: derefOrEmpty(s.StreamCaptureUrl), DriverGUID: derefOrEmpty(s.DriverGuid)})
	}
	instances, err := dba.selectServerInstances()
	if err != nil {
		return nil, err
	}
	for _, i := range instances {
		if derefOrEmpty(i.StreamEmbedUrl) != "" {
			sources = append(sources, CameraSource{ID: fmt.Sprint("spectator:", *i.Id), Name: derefOrEmpty(i.Name) + " · spectator", Enabled: isEnabled(i.StreamEnabled), PlayerURL: derefOrEmpty(i.StreamEmbedUrl), StatusURL: derefOrEmpty(i.StreamStatusUrl), InstanceID: i.Id})
		}
	}
	rows, err := dba.db.Query("SELECT id,name,enabled,player_url,status_url,capture_url,driver_guid,instance_id FROM camera_source ORDER BY id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s CameraSource
		var id int
		if err = rows.Scan(&id, &s.Name, &s.Enabled, &s.PlayerURL, &s.StatusURL, &s.CaptureURL, &s.DriverGUID, &s.InstanceID); err != nil {
			rows.Close()
			return nil, err
		}
		s.ID = fmt.Sprint("source:", id)
		sources = append(sources, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	links, err := dba.db.Query("SELECT source_id,rig_id FROM source_rig")
	if err != nil {
		return nil, err
	}
	defer links.Close()
	rigs := map[string]int{}
	for links.Next() {
		var key string
		var id int
		if err = links.Scan(&key, &id); err != nil {
			return nil, err
		}
		rigs[key] = id
	}
	for i := range sources {
		if id, ok := rigs[sources[i].ID]; ok {
			sources[i].RigID = &id
		}
		sources[i].RecordingConfigured = sources[i].CaptureURL != ""
		if sources[i].Name == "" {
			sources[i].Name = sources[i].DriverGUID
		}
	}
	for i := range sources {
		raw, _ := json.Marshal(sources[i])
		sum := sha256.Sum256(raw)
		sources[i].Version = hex.EncodeToString(sum[:])
	}
	return sources, links.Err()
}
func apiCameraSources(c *gin.Context) {
	sources, err := Dba.cameraSources()
	if err != nil {
		apiDbError(c, err)
		return
	}
	role, _ := c.Get("role")
	if role != roleAdmin {
		for i := range sources {
			sources[i].CaptureURL = ""
			sources[i].StatusURL = ""
		}
	}
	c.JSON(200, gin.H{"sources": sources})
}
func validateCameraSource(s *CameraSource) error {
	s.Name = strings.TrimSpace(s.Name)
	s.PlayerURL = strings.TrimSpace(s.PlayerURL)
	s.StatusURL = strings.TrimSpace(s.StatusURL)
	s.CaptureURL = strings.TrimSpace(s.CaptureURL)
	if s.Name == "" || len(s.Name) > 160 {
		return errors.New("Give the camera a name of 1–160 characters")
	}
	if err := validateStreamURL("Player URL", &s.PlayerURL, true); err != nil {
		return err
	}
	if err := validateStreamURL("Status URL", trimmedStringPtr(&s.StatusURL), false); err != nil {
		return err
	}
	return validateCaptureURL("Recording source", trimmedStringPtr(&s.CaptureURL))
}
func apiCameraSourceSave(c *gin.Context) {
	var s CameraSource
	if c.ShouldBindJSON(&s) != nil {
		apiBadRequest(c, "Invalid camera source")
		return
	}
	if err := validateCameraSource(&s); err != nil {
		apiBadRequest(c, err.Error())
		return
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer tx.Rollback()
	key := c.Param("key")
	if key != "" {
		current, e := Dba.cameraSources()
		if e != nil {
			apiDbError(c, e)
			return
		}
		found := false
		for _, source := range current {
			if source.ID == key {
				found = true
				if source.Version != s.Version {
					apiError(c, 409, "source_changed", "This camera changed. Reload its saved settings before editing again.")
					return
				}
			}
		}
		if !found {
			apiNotFound(c)
			return
		}
	}
	var result sql.Result
	if key == "" {
		result, err = tx.Exec("INSERT INTO camera_source(name,enabled,player_url,status_url,capture_url,driver_guid,instance_id) VALUES(?,?,?,?,?,?,?)", s.Name, s.Enabled, s.PlayerURL, s.StatusURL, s.CaptureURL, s.DriverGUID, s.InstanceID)
		if err == nil {
			id, _ := result.LastInsertId()
			key = fmt.Sprint("source:", id)
		}
	} else {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			apiBadRequest(c, "Invalid source identity")
			return
		}
		id, e := strconv.Atoi(parts[1])
		if e != nil || id <= 0 {
			apiBadRequest(c, "Invalid source identity")
			return
		}
		switch parts[0] {
		case "driver":
			result, err = tx.Exec("UPDATE driver_stream SET display_name=?,enabled=?,stream_embed_url=?,stream_status_url=?,stream_capture_url=? WHERE id=?", s.Name, s.Enabled, s.PlayerURL, s.StatusURL, s.CaptureURL, id)
		case "spectator":
			result, err = tx.Exec("UPDATE server_instance SET stream_enabled=?,stream_embed_url=?,stream_status_url=? WHERE id=?", s.Enabled, s.PlayerURL, s.StatusURL, id)
		case "source":
			result, err = tx.Exec("UPDATE camera_source SET name=?,enabled=?,player_url=?,status_url=?,capture_url=?,driver_guid=?,instance_id=? WHERE id=?", s.Name, s.Enabled, s.PlayerURL, s.StatusURL, s.CaptureURL, s.DriverGUID, s.InstanceID, id)
		default:
			apiBadRequest(c, "Invalid source identity")
			return
		}
		if err == nil {
			n, _ := result.RowsAffected()
			if n == 0 {
				apiNotFound(c)
				return
			}
		}
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	if s.RigID != nil {
		_, err = tx.Exec("INSERT INTO source_rig(source_id,rig_id) VALUES(?,?) ON CONFLICT(source_id) DO UPDATE SET rig_id=excluded.rig_id", key, s.RigID)
	} else {
		_, err = tx.Exec("DELETE FROM source_rig WHERE source_id=?", key)
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	if err = tx.Commit(); err != nil {
		apiDbError(c, err)
		return
	}
	if strings.HasPrefix(key, "spectator:") {
		if err = Instances.LoadFromDb(); err != nil {
			apiDbError(c, err)
			return
		}
	}
	Captures.refresh()
	s.ID = key
	s.RecordingConfigured = s.CaptureURL != ""
	c.JSON(200, gin.H{"source": s})
}

type SourceHealth struct {
	StreamHealth
	CheckedAt int64 `json:"checked_at"`
}

func apiCameraSourceHealth(c *gin.Context) {
	sources, err := Dba.cameraSources()
	if err != nil {
		apiDbError(c, err)
		return
	}
	statuses := map[string]SourceHealth{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	limit := make(chan struct{}, 4)
	for _, s := range sources {
		if key := c.Param("key"); key != "" && key != s.ID {
			continue
		}
		wg.Add(1)
		go func(s CameraSource) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			health := streamHealth(s.Enabled, &s.PlayerURL, &s.StatusURL)
			health.Message = ""
			mu.Lock()
			statuses[s.ID] = SourceHealth{health, time.Now().UnixMilli()}
			mu.Unlock()
		}(s)
	}
	wg.Wait()
	if c.Param("key") != "" && len(statuses) == 0 {
		apiNotFound(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"statuses": statuses})
}
