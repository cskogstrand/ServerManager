package main

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// An explicit server association can own a standalone camera's moment. A
// disconnected account alone is never used to guess an execution.
func sourceCaptureOwner(key string) (guid string, connection int64, guest int, execution int64) {
	guid = captureDriverGUID(key)
	connection, guest, execution = liveCaptureOwner(guid)
	if execution == 0 && strings.HasPrefix(key, "source:") {
		var instanceID int
		if Dba.db.QueryRow("SELECT instance_id FROM camera_source WHERE id=?", strings.TrimPrefix(key, "source:")).Scan(&instanceID) == nil {
			for _, inst := range Instances.All() {
				if inst.Id() == instanceID {
					inst.mu.Lock()
					if inst.cmd != nil {
						execution = inst.executionID
					}
					inst.mu.Unlock()
				}
			}
		}
	}
	return
}

func apiSourceMediaMutate(c *gin.Context) {
	key, file := c.Param("key"), c.Param("file")
	if file == "" || file == "." || filepath.Base(file) != file {
		apiNotFound(c)
		return
	}
	storage, column := key, "source_id"
	if strings.HasPrefix(key, "driver:") {
		column = "driver_guid"
		if Dba.db.QueryRow("SELECT driver_guid FROM driver_stream WHERE id=?", strings.TrimPrefix(key, "driver:")).Scan(&storage) != nil {
			apiNotFound(c)
			return
		}
	} else if !strings.HasPrefix(key, "source:") {
		apiNotFound(c)
		return
	}
	var id int64
	query := "SELECT id FROM driver_media WHERE " + column + "=? AND path=?"
	if column == "driver_guid" {
		query += " AND COALESCE(source_id,'')=''"
	}
	err := Dba.db.QueryRow(query, storage, file).Scan(&id)
	if err == sql.ErrNoRows {
		apiNotFound(c)
		return
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	if c.Request.Method == "DELETE" {
		if err = Dba.deleteDriverMedia([]int64{id}); err != nil {
			apiDbError(c, err)
			return
		}
		_ = os.Remove(filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(storage), file))
		c.JSON(200, gin.H{"status": "deleted"})
		return
	}
	var req assignRequest
	if c.ShouldBindJSON(&req) != nil {
		apiBadRequest(c, "Invalid assignment payload")
		return
	}
	if req.gdid() > 0 {
		exists, e := Dba.guestDriverExists(req.gdid())
		if e != nil {
			apiDbError(c, e)
			return
		}
		if !exists {
			apiBadRequest(c, "That guest driver no longer exists.")
			return
		}
	}
	if _, err = Dba.db.Exec("UPDATE driver_media SET guest_driver_id=? WHERE id=?", nullableId(req.gdid()), id); err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"guest_driver_id": req.GuestDriverId})
}

func captureDriverGUID(key string) string {
	if !strings.HasPrefix(key, "source:") {
		return key
	}
	var guid string
	_ = Dba.db.QueryRow("SELECT driver_guid FROM camera_source WHERE id=?", strings.TrimPrefix(key, "source:")).Scan(&guid)
	return guid
}
func sourceCaptureKey(c *gin.Context) (string, bool) {
	sources, err := Dba.cameraSources()
	if err != nil {
		apiDbError(c, err)
		return "", false
	}
	for _, source := range sources {
		if source.ID == c.Param("key") {
			if !source.Enabled {
				apiError(c, 409, "source_disabled", "Enable this source before capturing")
				return "", false
			}
			if strings.HasPrefix(source.ID, "source:") {
				return source.ID, true
			}
			if source.DriverGUID != "" {
				return source.DriverGUID, true
			}
			apiError(c, 409, "no_recorder", "This spectator source has no recording input")
			return "", false
		}
	}
	apiNotFound(c)
	return "", false
}
func apiSourceCapture(c *gin.Context) {
	key, ok := sourceCaptureKey(c)
	if !ok {
		return
	}
	if Captures == nil {
		apiError(c, 503, "capture_unavailable", "Capture is unavailable")
		return
	}
	var err error
	switch c.Param("action") {
	case "snapshot":
		var file string
		file, err = Captures.takeSnapshot(key)
		if err == nil {
			c.JSON(200, gin.H{"status": "captured", "url": "/api/sources/" + url.PathEscape(c.Param("key")) + "/media/" + url.PathEscape(file)})
			return
		}
	case "record":
		err = Captures.startManualRecording(key)
	case "stop":
		err = Captures.stopManualRecording(key)
	default:
		apiNotFound(c)
		return
	}
	if err != nil {
		apiError(c, 409, "capture_error", err.Error())
		return
	}
	c.JSON(200, gin.H{"status": map[string]string{"record": "recording", "stop": "assembling"}[c.Param("action")]})
}
func apiSourceMedia(c *gin.Context) {
	key := c.Param("key")
	file := c.Param("file")
	if file != "" {
		if filepath.Base(file) != file || file == "." {
			apiNotFound(c)
			return
		}
		var exists int
		var storage string
		if strings.HasPrefix(key, "source:") {
			storage = key
			_ = Dba.db.QueryRow("SELECT count(*) FROM driver_media WHERE source_id=? AND path=?", key, file).Scan(&exists)
		} else {
			sources, err := Dba.cameraSources()
			if err != nil {
				apiDbError(c, err)
				return
			}
			for _, s := range sources {
				if s.ID == key {
					storage = s.DriverGUID
					break
				}
			}
			_ = Dba.db.QueryRow("SELECT count(*) FROM driver_media WHERE driver_guid=? AND path=? AND COALESCE(source_id,'')=''", storage, file).Scan(&exists)
		}
		if exists == 0 {
			apiNotFound(c)
			return
		}
		path := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(storage), file)
		if c.Query("download") != "" {
			c.FileAttachment(path, file)
		} else {
			c.File(path)
		}
		return
	}
	owner := ""
	if strings.HasPrefix(key, "driver:") {
		_ = Dba.db.QueryRow("SELECT driver_guid FROM driver_stream WHERE id=?", strings.TrimPrefix(key, "driver:")).Scan(&owner)
	}
	rows, err := Dba.db.Query("SELECT id,kind,path,COALESCE(caption,''),captured_at,driver_guid,guest_driver_id FROM driver_media WHERE source_id=? OR (?<>'' AND driver_guid=? AND COALESCE(source_id,'')='') ORDER BY captured_at DESC LIMIT 100", key, owner, owner)
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int
		var kind, path, caption, guid string
		var captured int64
		var guest *int
		if err = rows.Scan(&id, &kind, &path, &caption, &captured, &guid, &guest); err != nil {
			apiDbError(c, err)
			return
		}
		items = append(items, map[string]any{"id": id, "kind": kind, "url": "/api/sources/" + url.PathEscape(key) + "/media/" + url.PathEscape(path), "caption": caption, "captured_at": captured, "driver_guid": guid, "guest_driver_id": guest})
	}
	c.JSON(200, gin.H{"items": items})
}

func apiSourceCaptureJobs(c *gin.Context) {
	key := c.Param("key")
	if strings.HasPrefix(key, "driver:") {
		_ = Dba.db.QueryRow("SELECT driver_guid FROM driver_stream WHERE id=?", strings.TrimPrefix(key, "driver:")).Scan(&key)
	}
	rows, err := Dba.db.Query("SELECT id,state,requested_at,ended_at,path,message FROM capture_job WHERE source_key=? ORDER BY id DESC LIMIT 30", key)
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int
		var state, path, message string
		var requested int64
		var ended *int64
		if err = rows.Scan(&id, &state, &requested, &ended, &path, &message); err != nil {
			apiDbError(c, err)
			return
		}
		items = append(items, map[string]any{"id": id, "state": state, "requested_at": requested, "ended_at": ended, "path": path, "message": message})
	}
	c.JSON(200, gin.H{"items": items})
}
