package main

// Extra drivers: a roster of real people who can share one Assetto Corsa account
// (GUID). A completed leaderboard row (drift run / timed lap) or a live, connected
// car can be attributed to one of them instead of the GUID's own name, so "who
// actually drove" is recorded even when several people use a single account.
//
// Three write paths feed the same extra_driver_id column the leaderboard reads:
//   - manage the roster      -> extra_driver table CRUD
//   - reassign a past row    -> set extra_driver_id on driver_drift_run/_session
//   - tag a live car         -> Instance.setExtraDriverForCar, snapshotted onto
//                               each run/session as it is recorded (see drivers.go)

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ztrue/tracerr"
)

// nullableId maps a 0 "none" id to SQL NULL so cleared attributions store as NULL
// rather than a row that points at extra_driver 0 (which never exists).
func nullableId(id int) any {
	if id > 0 {
		return id
	}
	return nil
}

// ---- API / row shape --------------------------------------------------------

type extraDriverRow struct {
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	Notes     string `json:"notes,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

// ---- DB access --------------------------------------------------------------

func (dba Dbaccess) selectExtraDrivers() ([]extraDriverRow, error) {
	rows, err := dba.db.Query(`SELECT id, name, notes, created_at FROM extra_driver ORDER BY name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]extraDriverRow, 0)
	for rows.Next() {
		var r extraDriverRow
		var notes sql.NullString
		if err := rows.Scan(&r.Id, &r.Name, &notes, &r.CreatedAt); err != nil {
			return nil, tracerr.Wrap(err)
		}
		r.Notes = notes.String
		out = append(out, r)
	}
	return out, tracerr.Wrap(rows.Err())
}

// selectExtraDriverNames is the id→name lookup buildScores uses to override the
// displayed name on attributed rows.
func (dba Dbaccess) selectExtraDriverNames() (map[int]string, error) {
	rows, err := dba.db.Query(`SELECT id, name FROM extra_driver`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	m := map[int]string{}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, tracerr.Wrap(err)
		}
		m[id] = name
	}
	return m, tracerr.Wrap(rows.Err())
}

func (dba Dbaccess) extraDriverExists(id int) (bool, error) {
	var n int
	if err := dba.db.QueryRow(`SELECT COUNT(*) FROM extra_driver WHERE id = ?`, id).Scan(&n); err != nil {
		return false, tracerr.Wrap(err)
	}
	return n > 0, nil
}

func (dba Dbaccess) insertExtraDriver(name, notes string, now int64) (int64, error) {
	res, err := dba.db.Exec(`INSERT INTO extra_driver (name, notes, created_at) VALUES (?, ?, ?)`, name, notes, now)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	id, err := res.LastInsertId()
	return id, tracerr.Wrap(err)
}

func (dba Dbaccess) updateExtraDriver(id int, name, notes string) (int64, error) {
	res, err := dba.db.Exec(`UPDATE extra_driver SET name = ?, notes = ? WHERE id = ?`, name, notes, id)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	affected, err := res.RowsAffected()
	return affected, tracerr.Wrap(err)
}

// deleteExtraDriver removes the roster entry and reverts any rows it was
// attributed to back to the GUID's own name (NULL the override). No FK does this
// for us — the override columns are intentionally constraint-free.
func (dba Dbaccess) deleteExtraDriver(id int) error {
	if _, err := dba.db.Exec(`UPDATE driver_drift_run SET extra_driver_id = NULL WHERE extra_driver_id = ?`, id); err != nil {
		return tracerr.Wrap(err)
	}
	if _, err := dba.db.Exec(`UPDATE driver_session SET extra_driver_id = NULL WHERE extra_driver_id = ?`, id); err != nil {
		return tracerr.Wrap(err)
	}
	if _, err := dba.db.Exec(`DELETE FROM extra_driver WHERE id = ?`, id); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) setDriftRunExtraDriver(runId, edid int) (int64, error) {
	res, err := dba.db.Exec(`UPDATE driver_drift_run SET extra_driver_id = ? WHERE id = ?`, nullableId(edid), runId)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	affected, err := res.RowsAffected()
	return affected, tracerr.Wrap(err)
}

func (dba Dbaccess) setSessionExtraDriver(sessionId, edid int) (int64, error) {
	res, err := dba.db.Exec(`UPDATE driver_session SET extra_driver_id = ? WHERE id = ?`, nullableId(edid), sessionId)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	affected, err := res.RowsAffected()
	return affected, tracerr.Wrap(err)
}

// ---- HTTP handlers ----------------------------------------------------------

type extraDriverRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

func apiExtraDriversList(c *gin.Context) {
	list, err := Dba.selectExtraDrivers()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiExtraDriverCreate(c *gin.Context) {
	var req extraDriverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid extra driver payload")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		apiBadRequest(c, "A name is required.")
		return
	}
	id, err := Dba.insertExtraDriver(name, strings.TrimSpace(req.Notes), time.Now().UnixMilli())
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiExtraDriverUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var req extraDriverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid extra driver payload")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		apiBadRequest(c, "A name is required.")
		return
	}
	affected, err := Dba.updateExtraDriver(id, name, strings.TrimSpace(req.Notes))
	if err != nil {
		apiDbError(c, err)
		return
	}
	if affected == 0 {
		apiNotFound(c)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiExtraDriverDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if err := Dba.deleteExtraDriver(id); err != nil {
		apiDbError(c, err)
		return
	}
	// Drop the now-deleted driver from any live car so the roster stops pointing
	// at a missing id.
	for _, inst := range Instances.All() {
		inst.clearExtraDriverAssignment(id)
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// assignRequest is shared by the row-reassign and live-assign endpoints.
// A null extra_driver_id clears the attribution (revert to the GUID's name).
type assignRequest struct {
	ExtraDriverId *int `json:"extra_driver_id"`
}

func (r assignRequest) edid() int {
	if r.ExtraDriverId != nil {
		return *r.ExtraDriverId
	}
	return 0
}

// apiScoreAssign reassigns a single leaderboard row to an extra driver (or clears
// it). The score id is the leaderboard's "d<runId>" (drift) or "l<sessionId>"
// (timed lap) — the same id buildScores emits.
func apiScoreAssign(c *gin.Context) {
	raw := strings.TrimSpace(c.Param("id"))
	if len(raw) < 2 {
		apiBadRequest(c, "Invalid score id")
		return
	}
	kind := raw[0]
	rowId, err := strconv.Atoi(raw[1:])
	if err != nil || rowId <= 0 {
		apiBadRequest(c, "Invalid score id")
		return
	}

	var req assignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid assignment payload")
		return
	}
	edid := req.edid()
	if edid > 0 {
		exists, err := Dba.extraDriverExists(edid)
		if err != nil {
			apiDbError(c, err)
			return
		}
		if !exists {
			apiBadRequest(c, "That extra driver no longer exists.")
			return
		}
	}

	var affected int64
	switch kind {
	case 'd':
		affected, err = Dba.setDriftRunExtraDriver(rowId, edid)
	case 'l':
		affected, err = Dba.setSessionExtraDriver(rowId, edid)
	default:
		apiBadRequest(c, "Invalid score id")
		return
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	if affected == 0 {
		apiNotFound(c)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": raw, "extra_driver_id": req.ExtraDriverId})
}

// apiLiveDriverAssign tags a currently-connected car with an extra driver so its
// subsequent runs/sessions are recorded under that person. Instance comes from
// ?instance=N (raceControlInstance), car + extra driver from the body.
func apiLiveDriverAssign(c *gin.Context) {
	inst, ok := raceControlInstance(c)
	if !ok {
		return
	}
	var body struct {
		CarId         int  `json:"car_id"`
		ExtraDriverId *int `json:"extra_driver_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiBadRequest(c, "A car_id is required")
		return
	}
	edid := 0
	if body.ExtraDriverId != nil {
		edid = *body.ExtraDriverId
	}
	if edid > 0 {
		exists, err := Dba.extraDriverExists(edid)
		if err != nil {
			apiDbError(c, err)
			return
		}
		if !exists {
			apiBadRequest(c, "That extra driver no longer exists.")
			return
		}
	}
	if !inst.setExtraDriverForCar(body.CarId, edid) {
		apiError(c, http.StatusNotFound, "no_driver", "No connected car with that id.")
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"sent": true, "car_id": body.CarId, "extra_driver_id": body.ExtraDriverId})
}
