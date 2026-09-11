package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// These queries follow explicit execution links. Legacy rows remain unlinked.
func apiDrivingRecap(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	type execution struct {
		ID        int             `json:"id"`
		Instance  int             `json:"instance_id"`
		State     string          `json:"state"`
		Setup     json.RawMessage `json:"setup"`
		Requested int64           `json:"requested_at"`
		Started   *int64          `json:"started_at"`
		Ended     *int64          `json:"ended_at"`
		Failure   string          `json:"failure"`
	}
	executions := []execution{}
	rows, err := Dba.db.Query("SELECT id,instance_id,state,setup_snapshot,requested_at,started_at,ended_at,failure FROM driving_execution WHERE session_id=? ORDER BY id", id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	for rows.Next() {
		var e execution
		var setup string
		if err = rows.Scan(&e.ID, &e.Instance, &e.State, &setup, &e.Requested, &e.Started, &e.Ended, &e.Failure); err != nil {
			rows.Close()
			apiDbError(c, err)
			return
		}
		e.Setup = json.RawMessage(setup)
		executions = append(executions, e)
	}
	rows.Close()
	stints := []map[string]any{}
	rows, err = Dba.db.Query(`SELECT s.id,s.execution_id,s.connection_id,s.driver_guid,s.guest_driver_id,COALESCE(g.name,d.name,s.driver_guid),s.session_type,s.car_key,s.track_key,s.track_config,s.laps,s.best_lap_ms,s.drift_best,s.finish_pos,s.started_at,s.ended_at FROM driver_session s LEFT JOIN driver d ON d.guid=s.driver_guid LEFT JOIN guest_driver g ON g.id=s.guest_driver_id WHERE s.execution_id IN(SELECT id FROM driving_execution WHERE session_id=?) ORDER BY s.started_at,s.id`, id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	for rows.Next() {
		var sid, eid, phase, laps, best, drift int
		var connection, guest, place *int
		var guid, name string
		var car, track, layout *string
		var start, end int64
		if err = rows.Scan(&sid, &eid, &connection, &guid, &guest, &name, &phase, &car, &track, &layout, &laps, &best, &drift, &place, &start, &end); err != nil {
			rows.Close()
			apiDbError(c, err)
			return
		}
		stints = append(stints, map[string]any{"id": sid, "execution_id": eid, "connection_id": connection, "guid": guid, "guest_id": guest, "name": name, "phase": phase, "car": car, "track": track, "layout": layout, "laps": laps, "best_lap_ms": best, "drift_best": drift, "finish_pos": place, "started_at": start, "ended_at": end})
	}
	rows.Close()
	media := []map[string]any{}
	rows, err = Dba.db.Query("SELECT id,execution_id,driver_guid,COALESCE(source_id,''),kind,path,COALESCE(caption,''),captured_at,guest_driver_id FROM driver_media WHERE execution_id IN(SELECT id FROM driving_execution WHERE session_id=?) ORDER BY captured_at DESC", id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	for rows.Next() {
		var mid, eid int
		var guid, source, kind, path, caption string
		var captured int64
		var guest *int
		if err = rows.Scan(&mid, &eid, &guid, &source, &kind, &path, &caption, &captured, &guest); err != nil {
			rows.Close()
			apiDbError(c, err)
			return
		}
		uri := "/api/drivers/" + url.PathEscape(guid) + "/media/" + url.PathEscape(path)
		if source != "" {
			uri = "/api/sources/" + url.PathEscape(source) + "/media/" + url.PathEscape(path)
		}
		media = append(media, map[string]any{"id": mid, "execution_id": eid, "guid": guid, "source_id": source, "guest_id": guest, "kind": kind, "url": uri, "caption": caption, "captured_at": captured})
	}
	rows.Close()
	phases := []map[string]any{}
	rows, err = Dba.db.Query("SELECT id,execution_id,started_at,payload FROM execution_phase WHERE execution_id IN(SELECT id FROM driving_execution WHERE session_id=?) ORDER BY id", id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	for rows.Next() {
		var pid, eid int
		var start int64
		var raw string
		if err = rows.Scan(&pid, &eid, &start, &raw); err != nil {
			rows.Close()
			apiDbError(c, err)
			return
		}
		phases = append(phases, map[string]any{"id": pid, "execution_id": eid, "started_at": start, "phase": json.RawMessage(raw)})
	}
	rows.Close()
	results := []map[string]any{}
	rows, err = Dba.db.Query("SELECT id,execution_id,name,created_at FROM execution_result WHERE execution_id IN(SELECT id FROM driving_execution WHERE session_id=?) ORDER BY id", id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	for rows.Next() {
		var rid, eid int
		var name string
		var created int64
		if err = rows.Scan(&rid, &eid, &name, &created); err != nil {
			rows.Close()
			apiDbError(c, err)
			return
		}
		results = append(results, map[string]any{"id": rid, "execution_id": eid, "name": name, "created_at": created})
	}
	rows.Close()
	c.JSON(200, gin.H{"results": results, "executions": executions, "stints": stints, "media": media, "phases": phases})
}
func recordExecutionPhase(inst *Instance, s SessionInfo, newPhase bool) {
	inst.mu.Lock()
	id := inst.executionID
	signature := fmt.Sprintf("%d:%d:%d:%d:%s:%s:%s", id, s.sessionIndex, s.currentSessionIndex, s.typ, s.track, s.trackConfig, s.name)
	if !newPhase && inst.phaseSignature == signature {
		inst.mu.Unlock()
		return
	}
	inst.phaseSignature = signature
	inst.mu.Unlock()
	if id == 0 {
		return
	}
	raw, err := json.Marshal(sessionEventPayload(s))
	if err != nil {
		return
	}
	if _, err = Dba.db.Exec("INSERT INTO execution_phase(execution_id,started_at,payload) VALUES(?,?,?)", id, time.Now().UnixMilli(), string(raw)); err != nil {
		inst.appendPitlaneError("Could not persist the game phase")
	}
}

// ACSP_END_SESSION supplies the concrete result filename for the current
// execution. Copy it now so repeated runs cannot overwrite the recap.
func recordExecutionResult(inst *Instance, name string) {
	inst.mu.Lock()
	execution := inst.executionID
	inst.mu.Unlock()
	if execution == 0 || name == "" {
		return
	}
	root, err := filepath.EvalSymlinks(inst.Dir())
	if err != nil {
		return
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		inst.appendPitlaneError("Result file is not yet readable: " + filepath.Base(name))
		return
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		inst.appendPitlaneError("Result filename is outside this server's run directory")
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8*1024*1024+1))
	if err != nil || len(data) > 8*1024*1024 || !json.Valid(data) {
		inst.appendPitlaneError("Result file is invalid or exceeds 8 MiB")
		return
	}
	digest := sha256.Sum256(data)
	if _, err = Dba.db.Exec("INSERT OR IGNORE INTO execution_result(execution_id,name,digest,data,created_at) VALUES(?,?,?,?,?)", execution, relative, hex.EncodeToString(digest[:]), string(data), time.Now().UnixMilli()); err != nil {
		inst.appendPitlaneError("Could not retain the execution result file")
	}
}
func apiExecutionResult(c *gin.Context) {
	session, ok := pathId(c)
	if !ok {
		return
	}
	var data string
	if err := Dba.db.QueryRow("SELECT data FROM execution_result WHERE id=? AND execution_id IN(SELECT id FROM driving_execution WHERE session_id=?)", c.Param("result"), session).Scan(&data); err != nil {
		apiNotFound(c)
		return
	}
	c.Data(200, "application/json; charset=utf-8", []byte(data))
}
