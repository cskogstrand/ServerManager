package main

// Driver-stats persistence + aggregation. Live driver state (drivers.go) is
// session-scoped and wiped each session; this layer writes completed sessions
// and drift runs through to SQLite so the Driver Stats / Driver Detail pages
// can show history, favourites and personal bests across sessions.
//
// Write path (called from the udpLoop goroutine, so events for one instance are
// serialized — no extra locking needed beyond Instance.mu for the live maps):
//   - driverJoin            -> recordDriverSeen (creates/updates the driver row)
//   - driverDrift (final)   -> insertDriftRun
//   - acspEndSession        -> finalizeCurrentSession (one driver_session per
//                              connected driver, with race finishing order)
//   - driverLeave / stop    -> record any not-yet-recorded session as a fallback
//
// Read path: buildDriverSummaries / getDriverDetail assemble the API responses
// (shapes mirror webapp/src/types/driverStats.ts).

import (
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ztrue/tracerr"
)

// ---- API response shapes (snake_case JSON to match the frontend contract) ---

type carRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Skin string `json:"skin,omitempty"`
}

type trackRef struct {
	Key     string `json:"key"`
	Config  string `json:"config,omitempty"`
	Name    string `json:"name"`
	Country string `json:"country,omitempty"`
}

type driverResult struct {
	SessionId  string   `json:"session_id"`
	Kind       string   `json:"kind"`
	Date       int64    `json:"date"`
	Track      trackRef `json:"track"`
	Car        carRef   `json:"car"`
	Position   *int     `json:"position,omitempty"`
	Entrants   *int     `json:"entrants,omitempty"`
	BestLapMs  *int     `json:"best_lap_ms,omitempty"`
	Laps       *int     `json:"laps,omitempty"`
	DriftScore *int     `json:"drift_score,omitempty"`
	DriftBest  *int     `json:"drift_best,omitempty"`
}

type mediaTrigger struct {
	DriftScore int    `json:"drift_score"`
	Delta      int    `json:"delta"`
	Track      string `json:"track,omitempty"`
}

type mediaItem struct {
	Id         string        `json:"id"`
	Kind       string        `json:"kind"`
	Url        string        `json:"url"`
	ThumbUrl   string        `json:"thumb_url,omitempty"`
	Caption    string        `json:"caption"`
	CapturedAt int64         `json:"captured_at"`
	DurationS  *int          `json:"duration_s,omitempty"`
	Trigger    *mediaTrigger `json:"trigger,omitempty"`
	// GuestDriverId attributes a standalone manual recording to a guest driver
	// (nil = the GUID's own name). Only set/used for the manual-recordings reel.
	GuestDriverId *int `json:"guest_driver_id,omitempty"`
}

type streamRef struct {
	EmbedUrl string `json:"embed_url"`
	Status   string `json:"status"`
}

type driverSummary struct {
	Guid           string        `json:"guid"`
	Name           string        `json:"name"`
	Online         bool          `json:"online"`
	AvatarUrl      *string       `json:"avatar_url,omitempty"`
	FirstSeen      int64         `json:"first_seen"`
	LastSeen       int64         `json:"last_seen"`
	Sessions       int           `json:"sessions"`
	TotalLaps      int           `json:"total_laps"`
	BestDrift      int           `json:"best_drift"`
	BestLapMs      int           `json:"best_lap_ms"`
	Podiums        int           `json:"podiums"`
	FavouriteCar   *carRef       `json:"favourite_car,omitempty"`
	FavouriteTrack *trackRef     `json:"favourite_track,omitempty"`
	LastResult     *driverResult `json:"last_result"`
	DriftTrend     []int         `json:"drift_trend"`
	// Set for roster guest drivers listed alongside GUID drivers: Guid is then
	// empty and GuestId points at the guest's profile (/guest-drivers/:id).
	IsGuest bool `json:"is_guest,omitempty"`
	GuestId int  `json:"guest_id,omitempty"`
}

type driverDetail struct {
	driverSummary
	Results  []driverResult  `json:"results"`
	Media    []mediaItem     `json:"media"`
	Stream   *streamRef      `json:"stream"`
	// session_history, not "sessions": driverSummary already marshals a "sessions"
	// count, and two fields with the same JSON tag would collide.
	Sessions []driverSession `json:"session_history"`
}

// sessionLap is one timed lap inside a session (connection). is_best flags the
// session's fastest lap so the UI can highlight it.
type sessionLap struct {
	Lap       int  `json:"lap"`
	LaptimeMs int  `json:"laptime_ms"`
	Cuts      int  `json:"cuts"`
	IsBest    bool `json:"is_best,omitempty"`
}

// driftRunItem is one completed drift run inside a session, with the highlight
// clip captured on it when one exists.
type driftRunItem struct {
	Id      string     `json:"id"`
	Score   int        `json:"score"`
	EndedAt int64      `json:"ended_at"`
	Clip    *mediaItem `json:"clip,omitempty"`
}

// driverSession is one connection (connect→disconnect) — the user-facing
// "session". It groups the AC-session segments, per-lap times, drift runs and
// captured media of a single stint, plus searchable tags. left_at is null while
// the driver is still connected. Shape mirrors DriverSession in
// webapp/src/types/driverStats.ts.
type driverSession struct {
	Id        string         `json:"id"`
	JoinedAt  int64          `json:"joined_at"`
	LeftAt    *int64         `json:"left_at"`
	Online    bool           `json:"online"`
	Track     trackRef       `json:"track"`
	Car       carRef         `json:"car"`
	Tags      []string       `json:"tags"`
	BestLapMs *int           `json:"best_lap_ms"`
	LapsTotal int            `json:"laps_total"`
	BestDrift *int           `json:"best_drift"`
	Segments  []driverResult `json:"segments"`
	Laps      []sessionLap   `json:"laps"`
	DriftRuns []driftRunItem `json:"drift_runs"`
	Media     []mediaItem    `json:"media"`
	// GuestDriverId is set when this whole connection is attributed to a guest
	// driver (apiSessionAssign cascades it to every row); nil = the GUID's own
	// name. The UI resolves the name from its loaded guest roster.
	GuestDriverId *int `json:"guest_driver_id,omitempty"`
}

// sessionSearchRow is one hit in the global session search (/api/driver-sessions):
// a connection matched by tag/name/track, with enough to render a result card and
// deep-link to /drivers/:guid?session=:id.
type sessionSearchRow struct {
	Id        string   `json:"id"`
	Guid      string   `json:"guid"`
	Driver    string   `json:"driver"`
	AvatarUrl *string  `json:"avatar_url,omitempty"`
	JoinedAt  int64    `json:"joined_at"`
	LeftAt    *int64   `json:"left_at"`
	Online    bool     `json:"online"`
	Track     trackRef `json:"track"`
	Car       carRef   `json:"car"`
	Tags      []string `json:"tags"`
	Laps      int      `json:"laps"`
	BestLapMs *int     `json:"best_lap_ms,omitempty"`
	BestDrift *int     `json:"best_drift,omitempty"`
}

// scoreEntry is one ranked result in the all-servers leaderboard: a single
// drift run (kind "drift") or a session's best timed lap (kind "lap"). Unlike
// driverSummary it is NOT collapsed per driver — every run/lap is its own row,
// so a driver can appear many times. Shape mirrors ScoreEntry in
// webapp/src/types/driverStats.ts.
type scoreEntry struct {
	Id         string   `json:"id"`
	Guid       string   `json:"guid"`
	Driver     string   `json:"driver"`
	Kind       string   `json:"kind"`
	Date       int64    `json:"date"`
	Track      trackRef `json:"track"`
	Car        carRef   `json:"car"`
	Online     bool       `json:"online"`
	DriftScore *int       `json:"drift_score,omitempty"`
	BestLapMs  *int       `json:"best_lap_ms,omitempty"`
	Position   *int       `json:"position,omitempty"`
	Entrants   *int       `json:"entrants,omitempty"`
	Clip       *mediaItem `json:"clip,omitempty"` // highlight clip captured on this drift run, if any
	// GuestDriverId is set when this row is attributed to a guest_driver (a real
	// person sharing the account's GUID) — Driver above is then that person's
	// name. Lets the leaderboard editor preselect the current assignment and link
	// the row to the guest's profile.
	GuestDriverId *int `json:"guest_driver_id,omitempty"`
}

// ---- internal row shapes ----------------------------------------------------

type driverRow struct {
	guid       string
	name       string
	firstSeen  int64
	lastSeen   int64
	avatarPath sql.NullString
}

type dsSessionRow struct {
	id          int64
	guid        string
	// name is the driver's display name at session end, carried for the
	// session_end SSE event only (not persisted; the driver table owns the name).
	name        string
	sessionType int
	carKey      string
	skinKey     string
	trackKey    string
	trackConfig string
	startedAt   int64
	endedAt     int64
	laps        int
	bestLapMs   int
	finishPos   sql.NullInt64
	entrants    sql.NullInt64
	driftBest   int
	// guestDriverId attributes this row to a guest_driver instead of the GUID's
	// own name in the leaderboard. 0 = none.
	guestDriverId int
	// connectionId is the driver_connection (session) this segment belongs to.
	// 0 = legacy row written before connections existed.
	connectionId int64
}

type dsDriftRow struct {
	id            int64
	guid          string
	score         int
	endedAt       int64
	connectionId  int64
	guestDriverId int // guest_driver attribution; 0 = the GUID's own name
}

// dsDriftFullRow carries the track/car a drift run was set on, for the flat
// leaderboard feed (the trend query only needs guid/score/time).
type dsDriftFullRow struct {
	id            int64
	guid          string
	trackKey      string
	trackConfig   string
	carKey        string
	score         int
	endedAt       int64
	guestDriverId int
}

// dsDriftInsert is the payload captured under Instance.mu and written after the
// lock is released.
type dsDriftInsert struct {
	guid        string
	trackKey    string
	trackConfig string
	carKey      string
	score       int
	endedAt     int64
	// guestDriverId snapshots the car's live guest-driver assignment so the run
	// is attributed to the right person in the leaderboard. 0 = none.
	guestDriverId int
	// connectionId is the driver_connection (session) this run happened in.
	connectionId int64
}

// dsConnInsert opens a driver_connection (session) row when a driver joins.
type dsConnInsert struct {
	guid        string
	carKey      string
	skinKey     string
	trackKey    string
	trackConfig string
	joinedAt    int64
}

// dsLapInsert is one completed lap, attributed to its connection.
type dsLapInsert struct {
	connectionId int64
	guid         string
	sessionType  int
	trackKey     string
	trackConfig  string
	carKey       string
	lapNumber    int
	laptimeMs    int
	cuts         int
	recordedAt   int64
}

type trackMeta struct {
	name    string
	country string
}

// ---- session finalization (write path) -------------------------------------

// sessionRowFromDriverLocked snapshots a driver's just-finished session. The
// caller must hold inst.mu.
func sessionRowFromDriverLocked(d *DriverState, now int64) dsSessionRow {
	start := d.joinedAt
	if start == 0 {
		start = now
	}
	return dsSessionRow{
		guid:        d.Guid,
		name:        d.Name,
		sessionType: d.sessType,
		carKey:      d.Car,
		skinKey:     d.Skin,
		trackKey:    d.sessTrack,
		trackConfig: d.sessConfig,
		startedAt:     start,
		endedAt:       now,
		laps:          d.Laps,
		bestLapMs:     int(d.BestLapMs),
		driftBest:     d.DriftBest,
		guestDriverId: d.GuestDriverId,
		connectionId:  d.connectionId,
	}
}

// collectFinishedSessionsLocked builds a session row for every connected driver
// that did something this session and hasn't been recorded yet, assigning race
// finishing order. It marks those drivers recorded. The caller must hold
// inst.mu.
func (inst *Instance) collectFinishedSessionsLocked(now int64) []dsSessionRow {
	cands := make([]*DriverState, 0, len(inst.drivers))
	for _, d := range inst.drivers {
		if d.recorded || d.Guid == "" {
			continue
		}
		if d.Laps == 0 && d.DriftBest == 0 {
			continue
		}
		cands = append(cands, d)
	}

	// Race finishing order among same-session race candidates: most laps first,
	// then fastest best lap; cars without a timed lap rank last.
	raceList := make([]*DriverState, 0, len(cands))
	for _, d := range cands {
		if d.sessType == 3 {
			raceList = append(raceList, d)
		}
	}
	sort.SliceStable(raceList, func(i, j int) bool {
		a, b := raceList[i], raceList[j]
		if a.Laps != b.Laps {
			return a.Laps > b.Laps
		}
		if (a.BestLapMs == 0) != (b.BestLapMs == 0) {
			return a.BestLapMs != 0
		}
		return a.BestLapMs < b.BestLapMs
	})
	rank := make(map[int]int, len(raceList))
	for i, d := range raceList {
		rank[d.CarId] = i + 1
	}
	entrants := len(raceList)

	rows := make([]dsSessionRow, 0, len(cands))
	for _, d := range cands {
		row := sessionRowFromDriverLocked(d, now)
		if d.sessType == 3 {
			if pos, ok := rank[d.CarId]; ok {
				row.finishPos = sql.NullInt64{Int64: int64(pos), Valid: true}
				row.entrants = sql.NullInt64{Int64: int64(entrants), Valid: true}
			}
		}
		d.recorded = true
		rows = append(rows, row)
	}
	return rows
}

// finalizeCurrentSession records the session that just ended for every
// connected driver. Called on ACSP_END_SESSION, before players are kicked for a
// track change, so race results are captured with finishing order.
func (inst *Instance) finalizeCurrentSession() {
	now := time.Now().UnixMilli()
	inst.mu.Lock()
	rows := inst.collectFinishedSessionsLocked(now)
	inst.mu.Unlock()
	persistFinishedSessions(inst.Id(), rows)
}

func persistFinishedSessions(instanceId int, rows []dsSessionRow) {
	for _, r := range rows {
		if strings.TrimSpace(r.guid) == "" {
			continue
		}
		if err := Dba.insertDriverSession(instanceId, r); err != nil {
			log.Print("driverstats: insert session: ", err)
			continue
		}
		Events.Publish("session_end", instanceId, sessionEndPayload(r))
	}
}

// liveDriversByGuid is the set of guids currently connected to any running
// instance, used to flag drivers online in the stats responses.
func liveDriversByGuid() map[string]bool {
	out := map[string]bool{}
	for _, inst := range Instances.All() {
		for _, d := range inst.driversSnapshot() {
			if d.Connected && d.Guid != "" {
				out[d.Guid] = true
			}
		}
	}
	return out
}

// liveConnectionIdForGuid returns the open connection id of the given guid if
// they are connected to any running instance right now, else 0. Manual captures
// (record/snapshot) use it to file their media under the live session.
func liveConnectionIdForGuid(guid string) int64 {
	if guid == "" {
		return 0
	}
	for _, inst := range Instances.All() {
		inst.mu.Lock()
		for _, d := range inst.drivers {
			if d.Connected && d.Guid == guid && d.connectionId > 0 {
				id := d.connectionId
				inst.mu.Unlock()
				return id
			}
		}
		inst.mu.Unlock()
	}
	return 0
}

// ---- DB access --------------------------------------------------------------

func (dba Dbaccess) recordDriverSeen(guid, name string, now int64) error {
	_, err := dba.db.Exec(`
INSERT INTO driver (guid, name, first_seen, last_seen) VALUES (?, ?, ?, ?)
ON CONFLICT(guid) DO UPDATE SET name = excluded.name, last_seen = excluded.last_seen`,
		guid, name, now, now)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) insertDriverSession(instanceId int, r dsSessionRow) error {
	_, err := dba.db.Exec(`
INSERT INTO driver_session
  (driver_guid, instance_id, session_type, car_key, skin_key, track_key, track_config,
   started_at, ended_at, laps, best_lap_ms, finish_pos, entrants, drift_best, guest_driver_id, connection_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.guid, instanceId, r.sessionType, r.carKey, r.skinKey, r.trackKey, r.trackConfig,
		r.startedAt, r.endedAt, r.laps, r.bestLapMs, r.finishPos, r.entrants, r.driftBest, nullableId(r.guestDriverId), nullableConnId(r.connectionId))
	if err != nil {
		return tracerr.Wrap(err)
	}
	_, _ = dba.db.Exec(`UPDATE driver SET last_seen = ? WHERE guid = ? AND last_seen < ?`, r.endedAt, r.guid, r.endedAt)
	return nil
}

// insertDriftRun persists a completed run and returns its new row id so the
// auto-capture can tag the resulting media with it (0 on error).
func (dba Dbaccess) insertDriftRun(instanceId int, r dsDriftInsert) (int64, error) {
	res, err := dba.db.Exec(`
INSERT INTO driver_drift_run (driver_guid, instance_id, track_key, track_config, car_key, score, ended_at, guest_driver_id, connection_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.guid, instanceId, r.trackKey, r.trackConfig, r.carKey, r.score, r.endedAt, nullableId(r.guestDriverId), nullableConnId(r.connectionId))
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	_, _ = dba.db.Exec(`UPDATE driver SET last_seen = ? WHERE guid = ? AND last_seen < ?`, r.endedAt, r.guid, r.endedAt)
	id, err := res.LastInsertId()
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	return id, nil
}

// nullableConnId maps a 0 connection id to SQL NULL (legacy/anonymous rows).
func nullableConnId(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// openDriverConnection writes a new open driver_connection (session) row and
// returns its id. left_at stays NULL until the driver disconnects.
func (dba Dbaccess) openDriverConnection(instanceId int, r dsConnInsert) (int64, error) {
	res, err := dba.db.Exec(`
INSERT INTO driver_connection (driver_guid, instance_id, joined_at, car_key, skin_key, track_key, track_config)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.guid, instanceId, r.joinedAt, r.carKey, r.skinKey, r.trackKey, r.trackConfig)
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, tracerr.Wrap(err)
	}
	return id, nil
}

// closeDriverConnection stamps left_at on a connection when the driver leaves
// (or the instance clears). Idempotent: only the first close wins.
func (dba Dbaccess) closeDriverConnection(id, now int64) error {
	_, err := dba.db.Exec(`UPDATE driver_connection SET left_at = ? WHERE id = ? AND left_at IS NULL`, now, id)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) insertDriverLap(instanceId int, r dsLapInsert) error {
	_, err := dba.db.Exec(`
INSERT INTO driver_lap
  (connection_id, driver_guid, instance_id, session_type, track_key, track_config, car_key, lap_number, laptime_ms, cuts, recorded_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.connectionId, r.guid, instanceId, r.sessionType, r.trackKey, r.trackConfig, r.carKey, r.lapNumber, r.laptimeMs, r.cuts, r.recordedAt)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) selectDrivers() ([]driverRow, error) {
	rows, err := dba.db.Query(`SELECT guid, name, first_seen, last_seen, avatar_path FROM driver`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]driverRow, 0)
	for rows.Next() {
		var dr driverRow
		if err := rows.Scan(&dr.guid, &dr.name, &dr.firstSeen, &dr.lastSeen, &dr.avatarPath); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out = append(out, dr)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

func (dba Dbaccess) selectDriver(guid string) (driverRow, bool, error) {
	var dr driverRow
	err := dba.db.QueryRow(`SELECT guid, name, first_seen, last_seen, avatar_path FROM driver WHERE guid = ?`, guid).
		Scan(&dr.guid, &dr.name, &dr.firstSeen, &dr.lastSeen, &dr.avatarPath)
	if err == sql.ErrNoRows {
		return driverRow{}, false, nil
	}
	if err != nil {
		return driverRow{}, false, tracerr.Wrap(err)
	}
	return dr, true, nil
}

// querySessions returns sessions newest-first. Pass "" for all drivers.
func (dba Dbaccess) querySessions(guid string) ([]dsSessionRow, error) {
	q := `SELECT id, driver_guid, session_type, car_key, skin_key, track_key, track_config,
       started_at, ended_at, laps, best_lap_ms, finish_pos, entrants, drift_best, guest_driver_id, connection_id
FROM driver_session`
	var rows *sql.Rows
	var err error
	if guid != "" {
		rows, err = dba.db.Query(q+` WHERE driver_guid = ? ORDER BY ended_at DESC`, guid)
	} else {
		rows, err = dba.db.Query(q + ` ORDER BY ended_at DESC`)
	}
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]dsSessionRow, 0)
	for rows.Next() {
		var s dsSessionRow
		var carKey, skinKey, trackKey, trackConfig sql.NullString
		var guestDriverId, connectionId sql.NullInt64
		if err := rows.Scan(&s.id, &s.guid, &s.sessionType, &carKey, &skinKey, &trackKey, &trackConfig,
			&s.startedAt, &s.endedAt, &s.laps, &s.bestLapMs, &s.finishPos, &s.entrants, &s.driftBest, &guestDriverId, &connectionId); err != nil {
			return nil, tracerr.Wrap(err)
		}
		s.carKey = carKey.String
		s.skinKey = skinKey.String
		s.trackKey = trackKey.String
		s.trackConfig = trackConfig.String
		s.guestDriverId = int(guestDriverId.Int64)
		s.connectionId = connectionId.Int64
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

// queryDriftRuns returns runs oldest-first (for the trend sparkline). Pass "" for all.
func (dba Dbaccess) queryDriftRuns(guid string) ([]dsDriftRow, error) {
	q := `SELECT id, driver_guid, score, ended_at, connection_id, guest_driver_id FROM driver_drift_run`
	var rows *sql.Rows
	var err error
	if guid != "" {
		rows, err = dba.db.Query(q+` WHERE driver_guid = ? ORDER BY ended_at ASC`, guid)
	} else {
		rows, err = dba.db.Query(q + ` ORDER BY ended_at ASC`)
	}
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]dsDriftRow, 0)
	for rows.Next() {
		var d dsDriftRow
		var connectionId, guestDriverId sql.NullInt64
		if err := rows.Scan(&d.id, &d.guid, &d.score, &d.endedAt, &connectionId, &guestDriverId); err != nil {
			return nil, tracerr.Wrap(err)
		}
		d.connectionId = connectionId.Int64
		d.guestDriverId = int(guestDriverId.Int64)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

// queryAllDriftRunsFull returns every drift run with its track/car, highest
// score first, for the flat leaderboard.
func (dba Dbaccess) queryAllDriftRunsFull() ([]dsDriftFullRow, error) {
	rows, err := dba.db.Query(`
SELECT id, driver_guid, track_key, track_config, car_key, score, ended_at, guest_driver_id
FROM driver_drift_run ORDER BY score DESC`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]dsDriftFullRow, 0)
	for rows.Next() {
		var r dsDriftFullRow
		var trackKey, trackConfig, carKey sql.NullString
		var guestDriverId sql.NullInt64
		if err := rows.Scan(&r.id, &r.guid, &trackKey, &trackConfig, &carKey, &r.score, &r.endedAt, &guestDriverId); err != nil {
			return nil, tracerr.Wrap(err)
		}
		r.trackKey = trackKey.String
		r.trackConfig = trackConfig.String
		r.carKey = carKey.String
		r.guestDriverId = int(guestDriverId.Int64)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

// ---- connections / laps / tags (session grouping) --------------------------

type dsConnRow struct {
	id          int64
	joinedAt    int64
	leftAt      sql.NullInt64
	carKey      string
	skinKey     string
	trackKey    string
	trackConfig string
}

// queryConnections returns a driver's connections (sessions) newest-first.
func (dba Dbaccess) queryConnections(guid string) ([]dsConnRow, error) {
	rows, err := dba.db.Query(`
SELECT id, joined_at, left_at, car_key, skin_key, track_key, track_config
FROM driver_connection WHERE driver_guid = ? ORDER BY joined_at DESC`, guid)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := make([]dsConnRow, 0)
	for rows.Next() {
		var c dsConnRow
		var carKey, skinKey, trackKey, trackConfig sql.NullString
		if err := rows.Scan(&c.id, &c.joinedAt, &c.leftAt, &carKey, &skinKey, &trackKey, &trackConfig); err != nil {
			return nil, tracerr.Wrap(err)
		}
		c.carKey = carKey.String
		c.skinKey = skinKey.String
		c.trackKey = trackKey.String
		c.trackConfig = trackConfig.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// searchConnRow is one connection enriched for the global search list: driver
// name/avatar plus per-connection roll-ups (laps, best lap, best drift, tags).
type searchConnRow struct {
	id          int64
	guid        string
	name        string
	avatarPath  string
	joinedAt    int64
	leftAt      sql.NullInt64
	carKey      string
	skinKey     string
	trackKey    string
	trackConfig string
	laps        int
	bestLapMs   int
	bestDrift   int
	tags        []string
}

// queryConnectionsForSearch returns connections matching an optional tag
// (contains match) and/or free-text query (driver name or track), newest first.
// Roll-ups come from the per-lap / drift-run / tag tables via subqueries.
func (dba Dbaccess) queryConnectionsForSearch(tag, q string, limit int) ([]searchConnRow, error) {
	query := `
SELECT c.id, c.driver_guid, COALESCE(d.name, c.driver_guid) AS dname, d.avatar_path,
       c.joined_at, c.left_at, c.car_key, c.skin_key, c.track_key, c.track_config,
       (SELECT COUNT(*) FROM driver_lap l WHERE l.connection_id = c.id) AS laps,
       (SELECT MIN(laptime_ms) FROM driver_lap l WHERE l.connection_id = c.id AND l.laptime_ms > 0) AS best_lap,
       (SELECT MAX(score) FROM driver_drift_run r WHERE r.connection_id = c.id) AS best_drift,
       (SELECT GROUP_CONCAT(t.tag, char(31)) FROM driver_session_tag t WHERE t.connection_id = c.id) AS tags
FROM driver_connection c
LEFT JOIN driver d ON d.guid = c.driver_guid`
	conds := []string{}
	args := []any{}
	if s := strings.TrimSpace(tag); s != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM driver_session_tag t WHERE t.connection_id = c.id AND t.tag LIKE ?)`)
		args = append(args, "%"+s+"%")
	}
	if s := strings.TrimSpace(q); s != "" {
		conds = append(conds, `(COALESCE(d.name, c.driver_guid) LIKE ? OR c.track_key LIKE ?)`)
		args = append(args, "%"+s+"%", "%"+s+"%")
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY c.joined_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := dba.db.Query(query, args...)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := make([]searchConnRow, 0)
	for rows.Next() {
		var r searchConnRow
		var avatar, carKey, skinKey, trackKey, trackConfig, tags sql.NullString
		var bestLap, bestDrift sql.NullInt64
		if err := rows.Scan(&r.id, &r.guid, &r.name, &avatar, &r.joinedAt, &r.leftAt,
			&carKey, &skinKey, &trackKey, &trackConfig, &r.laps, &bestLap, &bestDrift, &tags); err != nil {
			return nil, tracerr.Wrap(err)
		}
		r.avatarPath = avatar.String
		r.carKey = carKey.String
		r.skinKey = skinKey.String
		r.trackKey = trackKey.String
		r.trackConfig = trackConfig.String
		r.bestLapMs = int(bestLap.Int64)
		r.bestDrift = int(bestDrift.Int64)
		if tags.Valid && tags.String != "" {
			r.tags = strings.Split(tags.String, "\x1f")
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// queryLapsByConnection returns a driver's laps grouped by connection id, each
// group ordered by lap number.
func (dba Dbaccess) queryLapsByConnection(guid string) (map[int64][]sessionLap, error) {
	rows, err := dba.db.Query(`
SELECT connection_id, lap_number, laptime_ms, cuts
FROM driver_lap WHERE driver_guid = ? ORDER BY connection_id, lap_number ASC`, guid)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := map[int64][]sessionLap{}
	for rows.Next() {
		var connId int64
		var l sessionLap
		if err := rows.Scan(&connId, &l.Lap, &l.LaptimeMs, &l.Cuts); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out[connId] = append(out[connId], l)
	}
	return out, rows.Err()
}

// queryTagsByConnection returns the tags of all a driver's connections, keyed by
// connection id (oldest tag first).
func (dba Dbaccess) queryTagsByConnection(guid string) (map[int64][]string, error) {
	rows, err := dba.db.Query(`
SELECT t.connection_id, t.tag
FROM driver_session_tag t JOIN driver_connection c ON c.id = t.connection_id
WHERE c.driver_guid = ? ORDER BY t.created_at ASC`, guid)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var connId int64
		var tag string
		if err := rows.Scan(&connId, &tag); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out[connId] = append(out[connId], tag)
	}
	return out, rows.Err()
}

// connectionGuid returns the driver_guid that owns a connection, so tag writes
// can be scoped to the URL's :guid (ownership check). ok is false if no such row.
func (dba Dbaccess) connectionGuid(id int64) (string, bool, error) {
	var guid string
	err := dba.db.QueryRow(`SELECT driver_guid FROM driver_connection WHERE id = ?`, id).Scan(&guid)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, tracerr.Wrap(err)
	}
	return guid, true, nil
}

// deleteConnection wipes a whole session (connection) and everything tied to
// it — segments, laps, drift runs, media rows and tags — in one transaction. It
// returns the media file basenames so the caller can unlink them from disk
// (rows are gone regardless; a missing file isn't fatal). No FK cascade exists
// on these tables, so each child is deleted explicitly.
func (dba Dbaccess) deleteConnection(connId int64) ([]string, error) {
	tx, err := dba.db.Begin()
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT path FROM driver_media WHERE connection_id = ?`, connId)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	var files []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return nil, tracerr.Wrap(err)
		}
		files = append(files, filepath.Base(p))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}

	for _, q := range []string{
		`DELETE FROM driver_media WHERE connection_id = ?`,
		`DELETE FROM driver_lap WHERE connection_id = ?`,
		`DELETE FROM driver_drift_run WHERE connection_id = ?`,
		`DELETE FROM driver_session WHERE connection_id = ?`,
		`DELETE FROM driver_session_tag WHERE connection_id = ?`,
		`DELETE FROM driver_connection WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, connId); err != nil {
			return nil, tracerr.Wrap(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return files, nil
}

// addSessionTag adds a tag to a connection (no-op if it already has it).
func (dba Dbaccess) addSessionTag(connId int64, tag string, now int64) error {
	_, err := dba.db.Exec(`
INSERT INTO driver_session_tag (connection_id, tag, created_at) VALUES (?, ?, ?)
ON CONFLICT(connection_id, tag) DO NOTHING`, connId, tag, now)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

// removeSessionTag drops one tag from a connection.
func (dba Dbaccess) removeSessionTag(connId int64, tag string) error {
	_, err := dba.db.Exec(`DELETE FROM driver_session_tag WHERE connection_id = ? AND tag = ?`, connId, tag)
	if err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

// tagsForConnection returns one connection's tags (oldest first).
func (dba Dbaccess) tagsForConnection(connId int64) ([]string, error) {
	rows, err := dba.db.Query(`SELECT tag FROM driver_session_tag WHERE connection_id = ? ORDER BY created_at ASC`, connId)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, tracerr.Wrap(err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// mediaWithConn pairs a media item with the connection it was captured in, so
// getDriverDetail can build the flat reel and the per-session groups in one read.
type mediaWithConn struct {
	item       mediaItem
	connId     int64
	driftRunId int64 // 0 = not tied to a scored drift run (manual/snapshot/legacy)
}

// queryDriverMediaRows returns a driver's media newest-first, each tagged with
// its connection id (0 when unknown/legacy).
func (dba Dbaccess) queryDriverMediaRows(guid string) ([]mediaWithConn, error) {
	rows, err := dba.db.Query(`
SELECT id, kind, path, caption, captured_at, duration_s, trigger_score, trigger_delta, connection_id, drift_run_id, guest_driver_id
FROM driver_media WHERE driver_guid = ? ORDER BY captured_at DESC`, guid)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	out := make([]mediaWithConn, 0)
	for rows.Next() {
		var id int64
		var kind, path string
		var caption sql.NullString
		var capturedAt int64
		var durationS, triggerScore, triggerDelta, connId, driftRunId, guestId sql.NullInt64
		if err := rows.Scan(&id, &kind, &path, &caption, &capturedAt, &durationS, &triggerScore, &triggerDelta, &connId, &driftRunId, &guestId); err != nil {
			return nil, tracerr.Wrap(err)
		}
		m := mediaItem{
			Id:         strconv.FormatInt(id, 10),
			Kind:       kind,
			Url:        "/api/drivers/" + url.PathEscape(guid) + "/media/" + filepath.Base(path),
			Caption:    caption.String,
			CapturedAt: capturedAt,
		}
		if durationS.Valid {
			v := int(durationS.Int64)
			m.DurationS = &v
		}
		if triggerScore.Valid || triggerDelta.Valid {
			m.Trigger = &mediaTrigger{DriftScore: int(triggerScore.Int64), Delta: int(triggerDelta.Int64)}
		}
		if guestId.Valid {
			v := int(guestId.Int64)
			m.GuestDriverId = &v
		}
		out = append(out, mediaWithConn{item: m, connId: connId.Int64, driftRunId: driftRunId.Int64})
	}
	return out, rows.Err()
}

// clipsByRun returns clip-kind media keyed by the drift_run_id that triggered
// the capture — the exact score→video link for the leaderboard. Media with a
// NULL drift_run_id (manual recordings, legacy rows) are skipped.
func (dba Dbaccess) clipsByRun() (map[int64]mediaItem, error) {
	rows, err := dba.db.Query(`
SELECT driver_guid, id, kind, path, caption, captured_at, duration_s, trigger_score, trigger_delta, drift_run_id
FROM driver_media WHERE kind = 'clip' AND drift_run_id IS NOT NULL ORDER BY captured_at ASC`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := map[int64]mediaItem{}
	for rows.Next() {
		var guid, kind, path string
		var id int64
		var caption sql.NullString
		var capturedAt int64
		var durationS, triggerScore, triggerDelta, driftRunId sql.NullInt64
		if err := rows.Scan(&guid, &id, &kind, &path, &caption, &capturedAt, &durationS, &triggerScore, &triggerDelta, &driftRunId); err != nil {
			return nil, tracerr.Wrap(err)
		}
		if !driftRunId.Valid {
			continue
		}
		m := mediaItem{
			Id:         strconv.FormatInt(id, 10),
			Kind:       kind,
			Url:        "/api/drivers/" + url.PathEscape(guid) + "/media/" + filepath.Base(path),
			Caption:    caption.String,
			CapturedAt: capturedAt,
		}
		if durationS.Valid {
			v := int(durationS.Int64)
			m.DurationS = &v
		}
		if triggerScore.Valid || triggerDelta.Valid {
			m.Trigger = &mediaTrigger{DriftScore: int(triggerScore.Int64), Delta: int(triggerDelta.Int64)}
		}
		// One capture per run, so last write wins on the rare duplicate.
		out[driftRunId.Int64] = m
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

func (dba Dbaccess) selectDriverMedia(guid string) ([]mediaItem, error) {
	rows, err := dba.db.Query(`
SELECT id, kind, path, caption, captured_at, duration_s, trigger_score, trigger_delta
FROM driver_media WHERE driver_guid = ? ORDER BY captured_at DESC`, guid)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]mediaItem, 0)
	for rows.Next() {
		var id int64
		var kind, path string
		var caption sql.NullString
		var capturedAt int64
		var durationS, triggerScore, triggerDelta sql.NullInt64
		if err := rows.Scan(&id, &kind, &path, &caption, &capturedAt, &durationS, &triggerScore, &triggerDelta); err != nil {
			return nil, tracerr.Wrap(err)
		}
		m := mediaItem{
			Id:         strconv.FormatInt(id, 10),
			Kind:       kind,
			Url:        "/api/drivers/" + url.PathEscape(guid) + "/media/" + filepath.Base(path),
			Caption:    caption.String,
			CapturedAt: capturedAt,
		}
		if durationS.Valid {
			v := int(durationS.Int64)
			m.DurationS = &v
		}
		if triggerScore.Valid || triggerDelta.Valid {
			m.Trigger = &mediaTrigger{DriftScore: int(triggerScore.Int64), Delta: int(triggerDelta.Int64)}
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

func (dba Dbaccess) getDriverAvatarPath(guid string) (string, error) {
	var p sql.NullString
	err := dba.db.QueryRow(`SELECT avatar_path FROM driver WHERE guid = ?`, guid).Scan(&p)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", tracerr.Wrap(err)
	}
	return p.String, nil
}

func (dba Dbaccess) setDriverAvatar(guid, rel string) error {
	now := time.Now().UnixMilli()
	if _, err := dba.db.Exec(`
INSERT INTO driver (guid, name, first_seen, last_seen) VALUES (?, ?, ?, ?)
ON CONFLICT(guid) DO NOTHING`, guid, guid, now, now); err != nil {
		return tracerr.Wrap(err)
	}
	if _, err := dba.db.Exec(`UPDATE driver SET avatar_path = ? WHERE guid = ?`, rel, guid); err != nil {
		return tracerr.Wrap(err)
	}
	return nil
}

func (dba Dbaccess) selectCarNameMap() (map[string]string, error) {
	rows, err := dba.db.Query(`SELECT key, name FROM cache_car`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, n string
		if err := rows.Scan(&k, &n); err != nil {
			return nil, tracerr.Wrap(err)
		}
		m[k] = n
	}
	return m, rows.Err()
}

func (dba Dbaccess) selectTrackInfoMap() (map[string]trackMeta, error) {
	rows, err := dba.db.Query(`SELECT key, config, name, country FROM cache_track`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()
	m := map[string]trackMeta{}
	for rows.Next() {
		var k, cfg, n string
		var country sql.NullString
		if err := rows.Scan(&k, &cfg, &n, &country); err != nil {
			return nil, tracerr.Wrap(err)
		}
		meta := trackMeta{name: n, country: country.String}
		m[k+"\x00"+cfg] = meta
		if _, ok := m[k]; !ok {
			m[k] = meta
		}
	}
	return m, rows.Err()
}

// ---- aggregation ------------------------------------------------------------

func resolveCar(key, skin string, carNames map[string]string) carRef {
	name := carNames[key]
	if name == "" {
		name = key
	}
	return carRef{Key: key, Name: name, Skin: skin}
}

func resolveTrack(key, config string, trackInfo map[string]trackMeta) trackRef {
	meta, ok := trackInfo[key+"\x00"+config]
	if !ok {
		meta, ok = trackInfo[key]
	}
	name := key
	country := ""
	if ok {
		if meta.name != "" {
			name = meta.name
		}
		country = meta.country
	}
	return trackRef{Key: key, Config: config, Name: name, Country: country}
}

// topKey returns the most frequent key; ties resolve to the lexically smallest
// key so the result is stable across calls.
func topKey(counts map[string]int) string {
	best := ""
	bestN := 0
	for k, n := range counts {
		if n > bestN || (n == bestN && (best == "" || k < best)) {
			best, bestN = k, n
		}
	}
	return best
}

func resultFromSession(s dsSessionRow, carNames map[string]string, trackInfo map[string]trackMeta) driverResult {
	date := s.endedAt
	if date == 0 {
		date = s.startedAt
	}
	r := driverResult{
		SessionId: strconv.FormatInt(s.id, 10),
		Date:      date,
		Track:     resolveTrack(s.trackKey, s.trackConfig, trackInfo),
		Car:       resolveCar(s.carKey, s.skinKey, carNames),
	}
	switch {
	case s.sessionType == 3:
		r.Kind = "race"
	case s.driftBest > 0:
		r.Kind = "drift"
	case s.sessionType == 2:
		r.Kind = "qualify"
	default:
		r.Kind = "practice"
	}
	if r.Kind == "drift" {
		sc := s.driftBest
		r.DriftScore = &sc
		r.DriftBest = &sc
		return r
	}
	laps := s.laps
	r.Laps = &laps
	if s.bestLapMs > 0 {
		bl := s.bestLapMs
		r.BestLapMs = &bl
	}
	if s.finishPos.Valid {
		p := int(s.finishPos.Int64)
		r.Position = &p
	}
	if s.entrants.Valid {
		e := int(s.entrants.Int64)
		r.Entrants = &e
	}
	return r
}

// summaryFor aggregates one driver. sessions must be newest-first; drifts
// oldest-first.
func summaryFor(dr driverRow, sessions []dsSessionRow, drifts []dsDriftRow, carNames map[string]string, trackInfo map[string]trackMeta) driverSummary {
	sum := driverSummary{
		Guid:       dr.guid,
		Name:       dr.name,
		FirstSeen:  dr.firstSeen,
		LastSeen:   dr.lastSeen,
		DriftTrend: []int{},
	}
	if dr.avatarPath.Valid && strings.TrimSpace(dr.avatarPath.String) != "" {
		u := "/api/drivers/" + url.PathEscape(dr.guid) + "/avatar"
		sum.AvatarUrl = &u
	}

	carCount := map[string]int{}
	trackCount := map[string]int{}
	bestLap := 0
	for _, s := range sessions {
		sum.Sessions++
		sum.TotalLaps += s.laps
		if s.bestLapMs > 0 && (bestLap == 0 || s.bestLapMs < bestLap) {
			bestLap = s.bestLapMs
		}
		if s.driftBest > sum.BestDrift {
			sum.BestDrift = s.driftBest
		}
		if s.sessionType == 3 && s.finishPos.Valid && s.finishPos.Int64 >= 1 && s.finishPos.Int64 <= 3 {
			sum.Podiums++
		}
		if s.carKey != "" {
			carCount[s.carKey]++
		}
		if s.trackKey != "" {
			trackCount[s.trackKey+"\x00"+s.trackConfig]++
		}
	}
	sum.BestLapMs = bestLap

	for _, d := range drifts {
		if d.score > sum.BestDrift {
			sum.BestDrift = d.score
		}
	}
	start := 0
	if len(drifts) > 10 {
		start = len(drifts) - 10
	}
	for _, d := range drifts[start:] {
		sum.DriftTrend = append(sum.DriftTrend, d.score)
	}

	if k := topKey(carCount); k != "" {
		car := resolveCar(k, "", carNames)
		sum.FavouriteCar = &car
	}
	if k := topKey(trackCount); k != "" {
		parts := strings.SplitN(k, "\x00", 2)
		cfg := ""
		if len(parts) > 1 {
			cfg = parts[1]
		}
		tr := resolveTrack(parts[0], cfg, trackInfo)
		sum.FavouriteTrack = &tr
	}

	if len(sessions) > 0 {
		r := resultFromSession(sessions[0], carNames, trackInfo)
		sum.LastResult = &r
	}
	return sum
}

func buildDriverSummaries() ([]driverSummary, error) {
	drivers, err := Dba.selectDrivers()
	if err != nil {
		return nil, err
	}
	sessions, err := Dba.querySessions("")
	if err != nil {
		return nil, err
	}
	drifts, err := Dba.queryDriftRuns("")
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

	byGuidSessions := map[string][]dsSessionRow{}
	for _, s := range sessions {
		byGuidSessions[s.guid] = append(byGuidSessions[s.guid], s)
	}
	byGuidDrift := map[string][]dsDriftRow{}
	for _, d := range drifts {
		byGuidDrift[d.guid] = append(byGuidDrift[d.guid], d)
	}
	live := liveDriversByGuid()

	out := make([]driverSummary, 0, len(drivers))
	for _, dr := range drivers {
		sum := summaryFor(dr, byGuidSessions[dr.guid], byGuidDrift[dr.guid], carNames, trackInfo)
		if live[dr.guid] {
			sum.Online = true
		}
		out = append(out, sum)
	}
	// Roster guests with attributed results appear in the same list, flagged so
	// the UI can badge them and link to their profile instead of a GUID.
	guestSums, err := buildGuestSummaries(carNames, trackInfo)
	if err != nil {
		return nil, err
	}
	out = append(out, guestSums...)
	return out, nil
}

// buildScores assembles the flat leaderboard: one row per drift run and one per
// timed-lap session, across all drivers. Names/tracks/cars are resolved through
// the cache maps; rows are flagged online from the live set.
func buildScores() ([]scoreEntry, error) {
	drivers, err := Dba.selectDrivers()
	if err != nil {
		return nil, err
	}
	drifts, err := Dba.queryAllDriftRunsFull()
	if err != nil {
		return nil, err
	}
	sessions, err := Dba.querySessions("")
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
	clipByRun, err := Dba.clipsByRun()
	if err != nil {
		return nil, err
	}

	nameByGuid := make(map[string]string, len(drivers))
	for _, d := range drivers {
		nameByGuid[d.guid] = d.name
	}
	// Guest-driver name overrides: a row stamped with a guest_driver_id shows
	// that person's name instead of the GUID's own.
	guestNames, err := Dba.selectGuestDriverNames()
	if err != nil {
		return nil, err
	}
	live := liveDriversByGuid()
	driverName := func(guid string) string {
		if n := nameByGuid[guid]; n != "" {
			return n
		}
		return guid
	}
	// displayName resolves the leaderboard name for a row: the guest driver if
	// one is assigned and still exists, else the GUID's own name.
	displayName := func(guid string, guestDriverId int) (string, *int) {
		if guestDriverId > 0 {
			if n := guestNames[guestDriverId]; n != "" {
				id := guestDriverId
				return n, &id
			}
		}
		return driverName(guid), nil
	}

	out := make([]scoreEntry, 0, len(drifts)+len(sessions))
	for _, r := range drifts {
		sc := r.score
		name, gdid := displayName(r.guid, r.guestDriverId)
		e := scoreEntry{
			Id:            "d" + strconv.FormatInt(r.id, 10),
			Guid:          r.guid,
			Driver:        name,
			GuestDriverId: gdid,
			Kind:          "drift",
			Date:          r.endedAt,
			Track:         resolveTrack(r.trackKey, r.trackConfig, trackInfo),
			Car:           resolveCar(r.carKey, "", carNames),
			Online:        live[r.guid],
			DriftScore:    &sc,
		}
		if clip, ok := clipByRun[r.id]; ok {
			c := clip
			e.Clip = &c
		}
		out = append(out, e)
	}
	for _, s := range sessions {
		if s.bestLapMs <= 0 {
			continue
		}
		bl := s.bestLapMs
		name, gdid := displayName(s.guid, s.guestDriverId)
		e := scoreEntry{
			Id:            "l" + strconv.FormatInt(s.id, 10),
			Guid:          s.guid,
			Driver:        name,
			GuestDriverId: gdid,
			Kind:          "lap",
			Date:          s.endedAt,
			Track:         resolveTrack(s.trackKey, s.trackConfig, trackInfo),
			Car:           resolveCar(s.carKey, s.skinKey, carNames),
			Online:        live[s.guid],
			BestLapMs:     &bl,
		}
		if s.finishPos.Valid {
			p := int(s.finishPos.Int64)
			e.Position = &p
		}
		if s.entrants.Valid {
			en := int(s.entrants.Int64)
			e.Entrants = &en
		}
		out = append(out, e)
	}
	return out, nil
}

func streamForGuid(guid string) *streamRef {
	m, err := Dba.selectDriverStreamsByGuids([]string{guid})
	if err != nil {
		return nil
	}
	ds, ok := m[guid]
	if !ok {
		return nil
	}
	h := streamHealth(isEnabled(ds.Enabled), ds.StreamEmbedUrl, ds.StreamStatusUrl)
	return &streamRef{EmbedUrl: derefOrEmpty(ds.StreamEmbedUrl), Status: h.Status}
}

func getDriverDetail(guid string) (*driverDetail, error) {
	dr, found, err := Dba.selectDriver(guid)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	sessions, err := Dba.querySessions(guid)
	if err != nil {
		return nil, err
	}
	drifts, err := Dba.queryDriftRuns(guid)
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

	sum := summaryFor(dr, sessions, drifts, carNames, trackInfo)
	if liveDriversByGuid()[guid] {
		sum.Online = true
	}

	det := &driverDetail{driverSummary: sum}
	det.Results = make([]driverResult, 0, len(sessions))
	for _, s := range sessions {
		det.Results = append(det.Results, resultFromSession(s, carNames, trackInfo))
	}

	// Session (connection) grouping: the connect→disconnect view of this driver's
	// activity, with per-lap times, drift runs + clips, media and tags.
	conns, err := Dba.queryConnections(guid)
	if err != nil {
		return nil, err
	}
	lapsByConn, err := Dba.queryLapsByConnection(guid)
	if err != nil {
		return nil, err
	}
	tagsByConn, err := Dba.queryTagsByConnection(guid)
	if err != nil {
		return nil, err
	}
	mediaRows, err := Dba.queryDriverMediaRows(guid)
	if err != nil {
		return nil, err
	}
	clipByRun, err := Dba.clipsByRun()
	if err != nil {
		return nil, err
	}
	// Manual recordings made outside any session (no connection, no scored run)
	// get their own reel; everything else stays grouped under its session.
	det.Media = make([]mediaItem, 0)
	grouped := make([]mediaWithConn, 0, len(mediaRows))
	for _, mw := range mediaRows {
		if mw.connId == 0 && mw.driftRunId == 0 {
			det.Media = append(det.Media, mw.item)
		} else {
			grouped = append(grouped, mw)
		}
	}
	det.Sessions = assembleSessions(conns, sessions, drifts, lapsByConn, tagsByConn, grouped, clipByRun, carNames, trackInfo, sum.Online)
	det.Stream = streamForGuid(guid)
	return det, nil
}

// assembleSessions groups a driver's segments, laps, drift runs and media under
// the connections (sessions) they happened in, newest connection first. Activity
// that predates connections (NULL connection_id) is surfaced as one trailing
// "ungrouped" session (id "0") so nothing is lost on existing databases.
func assembleSessions(
	conns []dsConnRow,
	sessions []dsSessionRow,
	drifts []dsDriftRow,
	lapsByConn map[int64][]sessionLap,
	tagsByConn map[int64][]string,
	mediaRows []mediaWithConn,
	clipByRun map[int64]mediaItem,
	carNames map[string]string,
	trackInfo map[string]trackMeta,
	online bool,
) []driverSession {
	segByConn := map[int64][]dsSessionRow{}
	for _, s := range sessions {
		segByConn[s.connectionId] = append(segByConn[s.connectionId], s)
	}
	driftByConn := map[int64][]dsDriftRow{}
	for _, d := range drifts {
		driftByConn[d.connectionId] = append(driftByConn[d.connectionId], d)
	}
	mediaByConn := map[int64][]mediaItem{}
	for _, mw := range mediaRows {
		mediaByConn[mw.connId] = append(mediaByConn[mw.connId], mw.item)
	}

	out := make([]driverSession, 0, len(conns)+1)
	for _, c := range conns {
		out = append(out, buildSession(c.id, c.joinedAt, c.leftAt, c.carKey, c.skinKey, c.trackKey, c.trackConfig,
			segByConn[c.id], driftByConn[c.id], lapsByConn[c.id], tagsByConn[c.id], mediaByConn[c.id],
			clipByRun, carNames, trackInfo, online))
	}

	// Trailing "ungrouped" bucket for legacy rows whose connection_id is NULL/0.
	if len(segByConn[0]) > 0 || len(driftByConn[0]) > 0 || len(mediaByConn[0]) > 0 {
		segs, drifts0, media0 := segByConn[0], driftByConn[0], mediaByConn[0]
		var minT, maxT int64
		upd := func(t int64) {
			if t == 0 {
				return
			}
			if minT == 0 || t < minT {
				minT = t
			}
			if t > maxT {
				maxT = t
			}
		}
		for _, s := range segs {
			upd(s.startedAt)
			upd(s.endedAt)
		}
		for _, d := range drifts0 {
			upd(d.endedAt)
		}
		for _, m := range media0 {
			upd(m.CapturedAt)
		}
		tk, tc, ck, sk := "", "", "", ""
		if len(segs) > 0 {
			tk, tc, ck, sk = segs[0].trackKey, segs[0].trackConfig, segs[0].carKey, segs[0].skinKey
		}
		out = append(out, buildSession(0, minT, sql.NullInt64{Int64: maxT, Valid: maxT > 0}, ck, sk, tk, tc,
			segs, drifts0, nil, nil, media0, clipByRun, carNames, trackInfo, false))
	}
	return out
}

// buildSession assembles one driverSession from the activity of a single
// connection. segs/drifts may be in any order; they are sorted here (segments
// chronologically practice→qualify→race, drift runs newest-first). online is
// applied only while the connection is still open (left_at NULL).
func buildSession(
	id int64, joinedAt int64, leftAt sql.NullInt64,
	carKey, skinKey, trackKey, trackConfig string,
	segs []dsSessionRow, driftRows []dsDriftRow, laps []sessionLap, tags []string, media []mediaItem,
	clipByRun map[int64]mediaItem,
	carNames map[string]string, trackInfo map[string]trackMeta, online bool,
) driverSession {
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].startedAt < segs[j].startedAt })
	// Fall back to a segment's car/track if the connection never captured one.
	if trackKey == "" && len(segs) > 0 {
		trackKey, trackConfig = segs[0].trackKey, segs[0].trackConfig
	}
	if carKey == "" && len(segs) > 0 {
		carKey, skinKey = segs[0].carKey, segs[0].skinKey
	}

	ds := driverSession{
		Id:        strconv.FormatInt(id, 10),
		JoinedAt:  joinedAt,
		Track:     resolveTrack(trackKey, trackConfig, trackInfo),
		Car:       resolveCar(carKey, skinKey, carNames),
		Tags:      tags,
		Segments:  make([]driverResult, 0, len(segs)),
		Laps:      laps,
		DriftRuns: make([]driftRunItem, 0, len(driftRows)),
		Media:     media,
	}
	if ds.Tags == nil {
		ds.Tags = []string{}
	}
	if ds.Laps == nil {
		ds.Laps = []sessionLap{}
	}
	if ds.Media == nil {
		ds.Media = []mediaItem{}
	}
	if leftAt.Valid {
		v := leftAt.Int64
		ds.LeftAt = &v
	} else {
		ds.Online = online // still connected → online if the driver is live now
	}

	lapsTotal := 0
	bestLap := 0
	for _, s := range segs {
		ds.Segments = append(ds.Segments, resultFromSession(s, carNames, trackInfo))
		lapsTotal += s.laps
		if s.bestLapMs > 0 && (bestLap == 0 || s.bestLapMs < bestLap) {
			bestLap = s.bestLapMs
		}
	}
	for _, l := range ds.Laps {
		if l.LaptimeMs > 0 && (bestLap == 0 || l.LaptimeMs < bestLap) {
			bestLap = l.LaptimeMs
		}
	}
	if len(segs) == 0 {
		lapsTotal = len(ds.Laps)
	}
	if bestLap > 0 {
		for i := range ds.Laps {
			if ds.Laps[i].LaptimeMs == bestLap {
				ds.Laps[i].IsBest = true
				break
			}
		}
		bl := bestLap
		ds.BestLapMs = &bl
	}
	ds.LapsTotal = lapsTotal

	sort.SliceStable(driftRows, func(i, j int) bool { return driftRows[i].endedAt > driftRows[j].endedAt })
	bestDrift := 0
	for _, s := range segs {
		if s.driftBest > bestDrift {
			bestDrift = s.driftBest
		}
	}
	for _, r := range driftRows {
		item := driftRunItem{Id: strconv.FormatInt(r.id, 10), Score: r.score, EndedAt: r.endedAt}
		if clip, ok := clipByRun[r.id]; ok {
			c := clip
			item.Clip = &c
		}
		ds.DriftRuns = append(ds.DriftRuns, item)
		if r.score > bestDrift {
			bestDrift = r.score
		}
	}
	if bestDrift > 0 {
		bd := bestDrift
		ds.BestDrift = &bd
	}

	// Connection-level guest attribution: apiSessionAssign cascades one guest to
	// every row under a connection, so any attributed row reflects the stint.
	for _, s := range segs {
		if s.guestDriverId > 0 {
			gid := s.guestDriverId
			ds.GuestDriverId = &gid
			break
		}
	}
	if ds.GuestDriverId == nil {
		for _, r := range driftRows {
			if r.guestDriverId > 0 {
				gid := r.guestDriverId
				ds.GuestDriverId = &gid
				break
			}
		}
	}
	return ds
}

// searchSessions returns connections matching a tag and/or a free-text query
// (driver name or track key), newest first, capped at limit. Either filter may
// be empty. Used by the global session search page.
func searchSessions(tag, q string, limit int) ([]sessionSearchRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	carNames, err := Dba.selectCarNameMap()
	if err != nil {
		return nil, err
	}
	trackInfo, err := Dba.selectTrackInfoMap()
	if err != nil {
		return nil, err
	}
	conns, err := Dba.queryConnectionsForSearch(tag, q, limit)
	if err != nil {
		return nil, err
	}
	live := liveDriversByGuid()
	out := make([]sessionSearchRow, 0, len(conns))
	for _, c := range conns {
		row := sessionSearchRow{
			Id:       strconv.FormatInt(c.id, 10),
			Guid:     c.guid,
			Driver:   c.name,
			JoinedAt: c.joinedAt,
			Track:    resolveTrack(c.trackKey, c.trackConfig, trackInfo),
			Car:      resolveCar(c.carKey, c.skinKey, carNames),
			Tags:     c.tags,
			Laps:     c.laps,
		}
		if c.tags == nil {
			row.Tags = []string{}
		}
		if c.leftAt.Valid {
			v := c.leftAt.Int64
			row.LeftAt = &v
		} else {
			row.Online = live[c.guid]
		}
		if c.avatarPath != "" {
			u := "/api/drivers/" + url.PathEscape(c.guid) + "/avatar"
			row.AvatarUrl = &u
		}
		if c.bestLapMs > 0 {
			bl := c.bestLapMs
			row.BestLapMs = &bl
		}
		if c.bestDrift > 0 {
			bd := c.bestDrift
			row.BestDrift = &bd
		}
		out = append(out, row)
	}
	return out, nil
}

// ---- HTTP handlers ----------------------------------------------------------

func apiDriversList(c *gin.Context) {
	list, err := buildDriverSummaries()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"drivers": list})
}

func apiScoresList(c *gin.Context) {
	list, err := buildScores()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"scores": list})
}

func apiDriverGet(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	if guid == "" {
		apiBadRequest(c, "Invalid driver")
		return
	}
	det, err := getDriverDetail(guid)
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

// resolveSessionConn parses the :id session param and confirms the connection
// belongs to the URL's :guid. Returns the connection id, or writes the error
// response and returns ok=false.
func resolveSessionConn(c *gin.Context) (int64, bool) {
	guid := strings.TrimSpace(c.Param("guid"))
	connId, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if guid == "" || err != nil || connId <= 0 {
		apiBadRequest(c, "Invalid session")
		return 0, false
	}
	owner, found, err := Dba.connectionGuid(connId)
	if err != nil {
		apiDbError(c, err)
		return 0, false
	}
	if !found || owner != guid {
		apiNotFound(c)
		return 0, false
	}
	return connId, true
}

type sessionTagRequest struct {
	Tag string `json:"tag"`
}

// apiSessionTagAdd (POST /api/drivers/:guid/sessions/:id/tags) adds a tag to a
// session and returns the session's full tag set.
func apiSessionTagAdd(c *gin.Context) {
	connId, ok := resolveSessionConn(c)
	if !ok {
		return
	}
	var req sessionTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid tag payload")
		return
	}
	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		apiBadRequest(c, "A tag is required.")
		return
	}
	if len(tag) > 40 {
		apiBadRequest(c, "Tags are limited to 40 characters.")
		return
	}
	if err := Dba.addSessionTag(connId, tag, time.Now().UnixMilli()); err != nil {
		apiDbError(c, err)
		return
	}
	tags, err := Dba.tagsForConnection(connId)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"tags": tags})
}

// apiSessionTagRemove (DELETE /api/drivers/:guid/sessions/:id/tags?tag=) drops a
// tag (passed as a query param so spaces/slashes are safe) and returns the rest.
func apiSessionTagRemove(c *gin.Context) {
	connId, ok := resolveSessionConn(c)
	if !ok {
		return
	}
	tag := strings.TrimSpace(c.Query("tag"))
	if tag == "" {
		apiBadRequest(c, "Invalid tag")
		return
	}
	if err := Dba.removeSessionTag(connId, tag); err != nil {
		apiDbError(c, err)
		return
	}
	tags, err := Dba.tagsForConnection(connId)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"tags": tags})
}

// apiDriverSessionDelete (DELETE /api/drivers/:guid/sessions/:id) deletes a
// whole session (connection): its segments, laps, drift runs, tags and captured
// media rows + files on disk. No undo.
func apiDriverSessionDelete(c *gin.Context) {
	connId, ok := resolveSessionConn(c)
	if !ok {
		return
	}
	guid := strings.TrimSpace(c.Param("guid"))
	files, err := Dba.deleteConnection(connId)
	if err != nil {
		apiDbError(c, err)
		return
	}
	// Rows are gone; unlink the media files best-effort (same dir as the serve
	// handler). A missing file shouldn't fail the request.
	dir := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid))
	for _, f := range files {
		_ = os.Remove(filepath.Join(dir, f))
	}
	c.PureJSON(http.StatusOK, gin.H{"status": "deleted"})
}

// apiDriverSessionSearch (GET /api/driver-sessions) finds sessions across all
// drivers by tag and/or free-text (driver name or track).
func apiDriverSessionSearch(c *gin.Context) {
	tag := c.Query("tag")
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := searchSessions(tag, q, limit)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"sessions": list})
}

func mediaBaseDir() string {
	return filepath.Join(ConfigFolder, "media")
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "driver"
	}
	return out
}

func apiDriverAvatar(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	rel, err := Dba.getDriverAvatarPath(guid)
	if err != nil || strings.TrimSpace(rel) == "" {
		apiNotFound(c)
		return
	}
	// rel is set by us (avatars/<sanitized>.ext); Clean defends against surprises.
	abs := filepath.Join(mediaBaseDir(), filepath.Clean("/"+rel)[1:])
	c.File(abs)
}

var allowedAvatarExt = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}

func apiDriverAvatarUpload(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	if guid == "" {
		apiBadRequest(c, "Invalid driver")
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
	fname := sanitizeFilename(guid) + ext
	abs := filepath.Join(dir, fname)
	rel := filepath.Join("avatars", fname)
	if err := c.SaveUploadedFile(fh, abs); err != nil {
		apiDbError(c, err)
		return
	}
	if err := Dba.setDriverAvatar(guid, rel); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"avatar_url": "/api/drivers/" + url.PathEscape(guid) + "/avatar"})
}

func apiDriverMedia(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	file := filepath.Base(c.Param("file"))
	if guid == "" || file == "" || file == "." || file == "/" {
		apiNotFound(c)
		return
	}
	abs := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid), file)
	// ?download=1 forces a save dialog (Content-Disposition: attachment) instead
	// of inline playback/preview.
	if c.Query("download") != "" {
		c.FileAttachment(abs, file)
		return
	}
	c.File(abs)
}

func apiDriverMediaDelete(c *gin.Context) {
	guid := strings.TrimSpace(c.Param("guid"))
	file := filepath.Base(c.Param("file"))
	if guid == "" || file == "" || file == "." || file == "/" {
		apiNotFound(c)
		return
	}
	// Match by filename, scoped to the driver — guarantees the item belongs to
	// this guid before we touch the DB row or the file on disk.
	rows, err := Dba.listDriverMedia(guid)
	if err != nil {
		apiDbError(c, err)
		return
	}
	var ids []int64
	for _, r := range rows {
		if filepath.Base(r.path) == file {
			ids = append(ids, r.id)
		}
	}
	if len(ids) == 0 {
		apiNotFound(c)
		return
	}
	if err := Dba.deleteDriverMedia(ids); err != nil {
		apiDbError(c, err)
		return
	}
	// Row is gone; remove the file best-effort (a missing file shouldn't fail).
	abs := filepath.Join(mediaBaseDir(), "drivers", sanitizeFilename(guid), file)
	_ = os.Remove(abs)
	c.PureJSON(http.StatusOK, gin.H{"status": "deleted"})
}
