package main

// Guest drivers: a roster of real people who can share one Assetto Corsa account
// (GUID). A completed leaderboard row (drift run / timed lap) or a live, connected
// car can be attributed to one of them instead of the GUID's own name, so "who
// actually drove" is recorded even when several people use a single account.
// Each guest also gets its own profile page (avatar + KPIs + history), mirroring
// the GUID driver detail.
//
// Three write paths feed the same guest_driver_id column the leaderboard reads:
//   - manage the roster      -> guest_driver table CRUD
//   - reassign a past row    -> set guest_driver_id on driver_drift_run/_session
//   - tag a live car         -> Instance.setGuestDriverForCar, snapshotted onto
//                               each run/session as it is recorded (see drivers.go)

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ztrue/tracerr"
)

// nullableId maps a 0 "none" id to SQL NULL so cleared attributions store as NULL
// rather than a row that points at guest_driver 0 (which never exists).
func nullableId(id int) any {
	if id > 0 {
		return id
	}
	return nil
}

func guestAvatarUrl(id int64) string {
	return "/api/guest-drivers/" + strconv.FormatInt(id, 10) + "/avatar"
}

// ---- API / row shape --------------------------------------------------------

type guestDriverRow struct {
	Id        int64   `json:"id"`
	Name      string  `json:"name"`
	Notes     string  `json:"notes,omitempty"`
	AvatarUrl *string `json:"avatar_url,omitempty"`
	CreatedAt int64   `json:"created_at"`
}

// guestDriverDetail is the profile served at GET /guest-drivers/:id. It reuses
// the GUID driver summary shape (KPIs, favourites, trend) so the frontend can
// largely mirror the GUID driver detail, plus guest-only fields. Guests have no
// GUID and no stream, so Guid is empty and there is no stream field.
type guestDriverDetail struct {
	driverSummary
	Notes     string         `json:"notes,omitempty"`
	CreatedAt int64          `json:"created_at"`
	IsGuest   bool           `json:"is_guest"`
	Results   []driverResult `json:"results"`
	Media     []mediaItem    `json:"media"`
}

// ---- DB access --------------------------------------------------------------

func (dba Dbaccess) selectGuestDrivers() ([]guestDriverRow, error) {
	rows, err := dba.db.Query(`SELECT id, name, notes, avatar_path, created_at FROM guest_driver ORDER BY name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]guestDriverRow, 0)
	for rows.Next() {
		var r guestDriverRow
		var notes, avatar sql.NullString
		if err := rows.Scan(&r.Id, &r.Name, &notes, &avatar, &r.CreatedAt); err != nil {
			return nil, tracerr.Wrap(err)
		}
		r.Notes = notes.String
		if strings.TrimSpace(avatar.String) != "" {
			u := guestAvatarUrl(r.Id)
			r.AvatarUrl = &u
		}
		out = append(out, r)
	}
	return out, tracerr.Wrap(rows.Err())
}

func (dba Dbaccess) guestDriverByID(id int) (guestDriverRow, bool, error) {
	var r guestDriverRow
	var notes, avatar sql.NullString
	err := dba.db.QueryRow(`SELECT id, name, notes, avatar_path, created_at FROM guest_driver WHERE id = ?`, id).
		Scan(&r.Id, &r.Name, &notes, &avatar, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return guestDriverRow{}, false, nil
	}
	if err != nil {
		return guestDriverRow{}, false, tracerr.Wrap(err)
	}
	r.Notes = notes.String
	if strings.TrimSpace(avatar.String) != "" {
		u := guestAvatarUrl(r.Id)
		r.AvatarUrl = &u
	}
	return r, true, nil
}

// selectGuestDriverNames is the id→name lookup buildScores uses to override the
// displayed name on attributed rows.
func (dba Dbaccess) selectGuestDriverNames() (map[int]string, error) {
	rows, err := dba.db.Query(`SELECT id, name FROM guest_driver`)
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

func (dba Dbaccess) guestDriverExists(id int) (bool, error) {
	var n int
	if err := dba.db.QueryRow(`SELECT COUNT(*) FROM guest_driver WHERE id = ?`, id).Scan(&n); err != nil {
		return false, tracerr.Wrap(err)
	}
	return n > 0, nil
}

func (dba Dbaccess) insertGuestDriver(name, notes string, now int64) (int64, error) {
	res, err := dba.db.Exec(`INSERT INTO guest_driver (name, notes, created_at) VALUES (?, ?, ?)`, name, notes, now)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	id, err := res.LastInsertId()
	return id, tracerr.Wrap(err)
}

func (dba Dbaccess) updateGuestDriver(id int, name, notes string) (int64, error) {
	res, err := dba.db.Exec(`UPDATE guest_driver SET name = ?, notes = ? WHERE id = ?`, name, notes, id)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	affected, err := res.RowsAffected()
	return affected, tracerr.Wrap(err)
}

// deleteGuestDriver removes the roster entry and reverts any rows it was
// attributed to back to the GUID's own name (NULL the override). No FK does this
// for us — the override columns are intentionally constraint-free.
func (dba Dbaccess) deleteGuestDriver(id int) error {
	if _, err := dba.db.Exec(`UPDATE driver_drift_run SET guest_driver_id = NULL WHERE guest_driver_id = ?`, id); err != nil {
		return tracerr.Wrap(err)
	}
	if _, err := dba.db.Exec(`UPDATE driver_session SET guest_driver_id = NULL WHERE guest_driver_id = ?`, id); err != nil {
		return tracerr.Wrap(err)
	}
	if _, err := dba.db.Exec(`DELETE FROM guest_driver WHERE id = ?`, id); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) setDriftRunGuestDriver(runId, gdid int) (int64, error) {
	res, err := dba.db.Exec(`UPDATE driver_drift_run SET guest_driver_id = ? WHERE id = ?`, nullableId(gdid), runId)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	affected, err := res.RowsAffected()
	return affected, tracerr.Wrap(err)
}

func (dba Dbaccess) setSessionGuestDriver(sessionId, gdid int) (int64, error) {
	res, err := dba.db.Exec(`UPDATE driver_session SET guest_driver_id = ? WHERE id = ?`, nullableId(gdid), sessionId)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	affected, err := res.RowsAffected()
	return affected, tracerr.Wrap(err)
}

func (dba Dbaccess) getGuestDriverAvatarPath(id int) (string, error) {
	var p sql.NullString
	err := dba.db.QueryRow(`SELECT avatar_path FROM guest_driver WHERE id = ?`, id).Scan(&p)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", tracerr.Wrap(err)
	}
	return p.String, nil
}

func (dba Dbaccess) setGuestDriverAvatar(id int, rel string) error {
	if _, err := dba.db.Exec(`UPDATE guest_driver SET avatar_path = ? WHERE id = ?`, rel, id); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

// querySessionsForGuest returns a guest's attributed sessions, newest-first.
func (dba Dbaccess) querySessionsForGuest(id int) ([]dsSessionRow, error) {
	rows, err := dba.db.Query(`SELECT id, driver_guid, session_type, car_key, skin_key, track_key, track_config,
       started_at, ended_at, laps, best_lap_ms, finish_pos, entrants, drift_best, guest_driver_id
FROM driver_session WHERE guest_driver_id = ? ORDER BY ended_at DESC`, id)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]dsSessionRow, 0)
	for rows.Next() {
		var s dsSessionRow
		var carKey, skinKey, trackKey, trackConfig sql.NullString
		var guestDriverId sql.NullInt64
		if err := rows.Scan(&s.id, &s.guid, &s.sessionType, &carKey, &skinKey, &trackKey, &trackConfig,
			&s.startedAt, &s.endedAt, &s.laps, &s.bestLapMs, &s.finishPos, &s.entrants, &s.driftBest, &guestDriverId); err != nil {
			return nil, tracerr.Wrap(err)
		}
		s.carKey = carKey.String
		s.skinKey = skinKey.String
		s.trackKey = trackKey.String
		s.trackConfig = trackConfig.String
		s.guestDriverId = int(guestDriverId.Int64)
		out = append(out, s)
	}
	return out, tracerr.Wrap(rows.Err())
}

// queryDriftRunsForGuest returns a guest's attributed drift runs oldest-first
// (for the trend sparkline and best-score math).
func (dba Dbaccess) queryDriftRunsForGuest(id int) ([]dsDriftRow, error) {
	rows, err := dba.db.Query(`SELECT driver_guid, score, ended_at FROM driver_drift_run WHERE guest_driver_id = ? ORDER BY ended_at ASC`, id)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]dsDriftRow, 0)
	for rows.Next() {
		var d dsDriftRow
		if err := rows.Scan(&d.guid, &d.score, &d.endedAt); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out = append(out, d)
	}
	return out, tracerr.Wrap(rows.Err())
}

// driftRunIdsForGuest returns the ids of a guest's attributed drift runs,
// newest-first — used to gather their auto-captured highlight clips.
func (dba Dbaccess) driftRunIdsForGuest(id int) ([]int64, error) {
	rows, err := dba.db.Query(`SELECT id FROM driver_drift_run WHERE guest_driver_id = ? ORDER BY ended_at DESC`, id)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]int64, 0)
	for rows.Next() {
		var rid int64
		if err := rows.Scan(&rid); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out = append(out, rid)
	}
	return out, tracerr.Wrap(rows.Err())
}

// buildGuestSummaries returns Driver Stats rows for every roster guest that has
// at least one attributed result, aggregated from the sessions/drift runs tagged
// to them. They share the GUID driver summary shape but carry IsGuest + GuestId
// so the UI badges them and links to their profile. carNames/trackInfo are passed
// in so the caller's cache maps are reused.
func buildGuestSummaries(carNames map[string]string, trackInfo map[string]trackMeta) ([]driverSummary, error) {
	guests, err := Dba.selectGuestDrivers()
	if err != nil {
		return nil, err
	}
	out := make([]driverSummary, 0, len(guests))
	for _, g := range guests {
		sessions, err := Dba.querySessionsForGuest(int(g.Id))
		if err != nil {
			return nil, err
		}
		drifts, err := Dba.queryDriftRunsForGuest(int(g.Id))
		if err != nil {
			return nil, err
		}
		// A guest with nothing attributed yet has no stats — leave them to the
		// roster page rather than show an empty leaderboard row.
		if len(sessions) == 0 && len(drifts) == 0 {
			continue
		}
		dr := driverRow{name: g.Name, firstSeen: g.CreatedAt, lastSeen: g.CreatedAt}
		sum := summaryFor(dr, sessions, drifts, carNames, trackInfo)
		sum.Guid = ""
		sum.AvatarUrl = g.AvatarUrl
		sum.IsGuest = true
		sum.GuestId = int(g.Id)
		if len(sessions) > 0 && sessions[0].endedAt > sum.LastSeen {
			sum.LastSeen = sessions[0].endedAt
		}
		out = append(out, sum)
	}
	return out, nil
}

// getGuestDriverDetail aggregates one guest's attributed results into the same
// summary shape used for GUID drivers, plus the guest's own roster fields and a
// highlight reel built from clips attached to their attributed drift runs.
func getGuestDriverDetail(id int) (*guestDriverDetail, error) {
	g, found, err := Dba.guestDriverByID(id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	sessions, err := Dba.querySessionsForGuest(id)
	if err != nil {
		return nil, err
	}
	drifts, err := Dba.queryDriftRunsForGuest(id)
	if err != nil {
		return nil, err
	}
	carNames, err := Dba.selectCarNameMap()
	if err != nil {
		return nil, err
	}
	trackInfo, err := Dba.selectTrackInfoMap()
	if err != nil {
		return nil, err
	}

	// Synthesize a driverRow so summaryFor can do the KPI/favourites/trend math.
	// Guests have no GUID; the avatar is served from the guest endpoint instead.
	dr := driverRow{name: g.Name, firstSeen: g.CreatedAt, lastSeen: g.CreatedAt}
	sum := summaryFor(dr, sessions, drifts, carNames, trackInfo)
	sum.Guid = ""
	sum.AvatarUrl = g.AvatarUrl
	// Last activity = newest session end (sessions are newest-first).
	if len(sessions) > 0 && sessions[0].endedAt > sum.LastSeen {
		sum.LastSeen = sessions[0].endedAt
	}

	det := &guestDriverDetail{
		driverSummary: sum,
		Notes:         g.Notes,
		CreatedAt:     g.CreatedAt,
		IsGuest:       true,
	}
	det.Results = make([]driverResult, 0, len(sessions))
	for _, s := range sessions {
		det.Results = append(det.Results, resultFromSession(s, carNames, trackInfo))
	}

	// Highlight reel: clips attached to this guest's attributed drift runs.
	runIds, err := Dba.driftRunIdsForGuest(id)
	if err != nil {
		return nil, err
	}
	det.Media = make([]mediaItem, 0)
	if len(runIds) > 0 {
		clipByRun, err := Dba.clipsByRun()
		if err != nil {
			return nil, err
		}
		for _, rid := range runIds {
			if clip, ok := clipByRun[rid]; ok {
				det.Media = append(det.Media, clip)
			}
		}
	}
	return det, nil
}

// ---- HTTP handlers ----------------------------------------------------------

type guestDriverRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

func apiGuestDriversList(c *gin.Context) {
	list, err := Dba.selectGuestDrivers()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiGuestDriverGet(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	det, err := getGuestDriverDetail(id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if det == nil {
		apiNotFound(c)
		return
	}
	c.PureJSON(http.StatusOK, det)
}

func apiGuestDriverCreate(c *gin.Context) {
	var req guestDriverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid guest driver payload")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		apiBadRequest(c, "A name is required.")
		return
	}
	id, err := Dba.insertGuestDriver(name, strings.TrimSpace(req.Notes), time.Now().UnixMilli())
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiGuestDriverUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var req guestDriverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid guest driver payload")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		apiBadRequest(c, "A name is required.")
		return
	}
	affected, err := Dba.updateGuestDriver(id, name, strings.TrimSpace(req.Notes))
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

func apiGuestDriverDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if err := Dba.deleteGuestDriver(id); err != nil {
		apiDbError(c, err)
		return
	}
	// Drop the now-deleted driver from any live car so the roster stops pointing
	// at a missing id.
	for _, inst := range Instances.All() {
		inst.clearGuestDriverAssignment(id)
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiGuestDriverAvatar(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	rel, err := Dba.getGuestDriverAvatarPath(id)
	if err != nil || strings.TrimSpace(rel) == "" {
		apiNotFound(c)
		return
	}
	// rel is set by us (avatars/guest-<id>.ext); Clean defends against surprises.
	abs := filepath.Join(mediaBaseDir(), filepath.Clean("/"+rel)[1:])
	c.File(abs)
}

func apiGuestDriverAvatarUpload(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	exists, err := Dba.guestDriverExists(id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if !exists {
		apiNotFound(c)
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		apiBadRequest(c, "No file uploaded")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowedAvatarExt[ext] {
		apiBadRequest(c, "Unsupported image type. Use JPG, PNG, WEBP or GIF.")
		return
	}
	if fh.Size > 8<<20 {
		apiBadRequest(c, "Image too large (max 8 MB).")
		return
	}

	dir := filepath.Join(mediaBaseDir(), "avatars")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		apiDbError(c, err)
		return
	}
	fname := "guest-" + strconv.Itoa(id) + ext
	abs := filepath.Join(dir, fname)
	rel := filepath.Join("avatars", fname)
	if err := c.SaveUploadedFile(fh, abs); err != nil {
		apiDbError(c, err)
		return
	}
	if err := Dba.setGuestDriverAvatar(id, rel); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"avatar_url": guestAvatarUrl(int64(id))})
}

// assignRequest is shared by the row-reassign and live-assign endpoints.
// A null guest_driver_id clears the attribution (revert to the GUID's name).
type assignRequest struct {
	GuestDriverId *int `json:"guest_driver_id"`
}

func (r assignRequest) gdid() int {
	if r.GuestDriverId != nil {
		return *r.GuestDriverId
	}
	return 0
}

// apiScoreAssign reassigns a single leaderboard row to a guest driver (or clears
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
	gdid := req.gdid()
	if gdid > 0 {
		exists, err := Dba.guestDriverExists(gdid)
		if err != nil {
			apiDbError(c, err)
			return
		}
		if !exists {
			apiBadRequest(c, "That guest driver no longer exists.")
			return
		}
	}

	var affected int64
	switch kind {
	case 'd':
		affected, err = Dba.setDriftRunGuestDriver(rowId, gdid)
	case 'l':
		affected, err = Dba.setSessionGuestDriver(rowId, gdid)
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
	c.PureJSON(http.StatusOK, gin.H{"id": raw, "guest_driver_id": req.GuestDriverId})
}

// apiLiveDriverAssign tags a currently-connected car with a guest driver so its
// subsequent runs/sessions are recorded under that person. Instance comes from
// ?instance=N (raceControlInstance), car + guest driver from the body.
func apiLiveDriverAssign(c *gin.Context) {
	inst, ok := raceControlInstance(c)
	if !ok {
		return
	}
	var body struct {
		CarId         int  `json:"car_id"`
		GuestDriverId *int `json:"guest_driver_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiBadRequest(c, "A car_id is required")
		return
	}
	gdid := 0
	if body.GuestDriverId != nil {
		gdid = *body.GuestDriverId
	}
	if gdid > 0 {
		exists, err := Dba.guestDriverExists(gdid)
		if err != nil {
			apiDbError(c, err)
			return
		}
		if !exists {
			apiBadRequest(c, "That guest driver no longer exists.")
			return
		}
	}
	if !inst.setGuestDriverForCar(body.CarId, gdid) {
		apiError(c, http.StatusNotFound, "no_driver", "No connected car with that id.")
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"sent": true, "car_id": body.CarId, "guest_driver_id": body.GuestDriverId})
}
