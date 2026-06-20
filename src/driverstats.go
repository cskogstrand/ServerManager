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
}

type driverDetail struct {
	driverSummary
	Results []driverResult `json:"results"`
	Media   []mediaItem    `json:"media"`
	Stream  *streamRef     `json:"stream"`
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
}

type dsDriftRow struct {
	guid    string
	score   int
	endedAt int64
}

// dsDriftFullRow carries the track/car a drift run was set on, for the flat
// leaderboard feed (the trend query only needs guid/score/time).
type dsDriftFullRow struct {
	id          int64
	guid        string
	trackKey    string
	trackConfig string
	carKey      string
	score       int
	endedAt     int64
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
		sessionType: d.sessType,
		carKey:      d.Car,
		skinKey:     d.Skin,
		trackKey:    d.sessTrack,
		trackConfig: d.sessConfig,
		startedAt:   start,
		endedAt:     now,
		laps:        d.Laps,
		bestLapMs:   int(d.BestLapMs),
		driftBest:   d.DriftBest,
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
		}
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
   started_at, ended_at, laps, best_lap_ms, finish_pos, entrants, drift_best)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.guid, instanceId, r.sessionType, r.carKey, r.skinKey, r.trackKey, r.trackConfig,
		r.startedAt, r.endedAt, r.laps, r.bestLapMs, r.finishPos, r.entrants, r.driftBest)
	if err != nil {
		return tracerr.Wrap(err)
	}
	_, _ = dba.db.Exec(`UPDATE driver SET last_seen = ? WHERE guid = ? AND last_seen < ?`, r.endedAt, r.guid, r.endedAt)
	return nil
}

func (dba Dbaccess) insertDriftRun(instanceId int, r dsDriftInsert) error {
	_, err := dba.db.Exec(`
INSERT INTO driver_drift_run (driver_guid, instance_id, track_key, track_config, car_key, score, ended_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.guid, instanceId, r.trackKey, r.trackConfig, r.carKey, r.score, r.endedAt)
	if err != nil {
		return tracerr.Wrap(err)
	}
	_, _ = dba.db.Exec(`UPDATE driver SET last_seen = ? WHERE guid = ? AND last_seen < ?`, r.endedAt, r.guid, r.endedAt)
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
       started_at, ended_at, laps, best_lap_ms, finish_pos, entrants, drift_best
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
		if err := rows.Scan(&s.id, &s.guid, &s.sessionType, &carKey, &skinKey, &trackKey, &trackConfig,
			&s.startedAt, &s.endedAt, &s.laps, &s.bestLapMs, &s.finishPos, &s.entrants, &s.driftBest); err != nil {
			return nil, tracerr.Wrap(err)
		}
		s.carKey = carKey.String
		s.skinKey = skinKey.String
		s.trackKey = trackKey.String
		s.trackConfig = trackConfig.String
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

// queryDriftRuns returns runs oldest-first (for the trend sparkline). Pass "" for all.
func (dba Dbaccess) queryDriftRuns(guid string) ([]dsDriftRow, error) {
	q := `SELECT driver_guid, score, ended_at FROM driver_drift_run`
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
		if err := rows.Scan(&d.guid, &d.score, &d.endedAt); err != nil {
			return nil, tracerr.Wrap(err)
		}
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
SELECT id, driver_guid, track_key, track_config, car_key, score, ended_at
FROM driver_drift_run ORDER BY score DESC`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := make([]dsDriftFullRow, 0)
	for rows.Next() {
		var r dsDriftFullRow
		var trackKey, trackConfig, carKey sql.NullString
		if err := rows.Scan(&r.id, &r.guid, &trackKey, &trackConfig, &carKey, &r.score, &r.endedAt); err != nil {
			return nil, tracerr.Wrap(err)
		}
		r.trackKey = trackKey.String
		r.trackConfig = trackConfig.String
		r.carKey = carKey.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, tracerr.Wrap(err)
	}
	return out, nil
}

// queryAllClips returns every clip-kind media item, grouped by driver guid and
// sorted oldest-first, for relating drift runs to their highlight video.
func (dba Dbaccess) queryAllClips() (map[string][]mediaItem, error) {
	rows, err := dba.db.Query(`
SELECT driver_guid, id, kind, path, caption, captured_at, duration_s, trigger_score, trigger_delta
FROM driver_media WHERE kind = 'clip' ORDER BY captured_at ASC`)
	if err != nil {
		return nil, tracerr.Wrap(err)
	}
	defer rows.Close()

	out := map[string][]mediaItem{}
	for rows.Next() {
		var guid, kind, path string
		var id int64
		var caption sql.NullString
		var capturedAt int64
		var durationS, triggerScore, triggerDelta sql.NullInt64
		if err := rows.Scan(&guid, &id, &kind, &path, &caption, &capturedAt, &durationS, &triggerScore, &triggerDelta); err != nil {
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
		out[guid] = append(out[guid], m)
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
	return out, nil
}

// clipForRun pairs a drift run with its highlight clip. Capture fires at run
// end (driverDrift → Captures.onDriftRunEnd), so the clip whose captured_at is
// nearest the run's ended_at — within a tolerance — is that run's video. clips
// must be sorted by captured_at. Returns nil when nothing lands in the window.
func clipForRun(clips []mediaItem, endedAt int64) *mediaItem {
	const windowMs = 180_000 // 3 min — well beyond the run-end → file-written lag
	best := -1
	var bestDelta int64 = windowMs + 1
	for i := range clips {
		d := endedAt - clips[i].CapturedAt
		if d < 0 {
			d = -d
		}
		if d <= windowMs && d < bestDelta {
			best, bestDelta = i, d
		}
	}
	if best < 0 {
		return nil
	}
	c := clips[best]
	return &c
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
	clipsByGuid, err := Dba.queryAllClips()
	if err != nil {
		return nil, err
	}

	nameByGuid := make(map[string]string, len(drivers))
	for _, d := range drivers {
		nameByGuid[d.guid] = d.name
	}
	live := liveDriversByGuid()
	driverName := func(guid string) string {
		if n := nameByGuid[guid]; n != "" {
			return n
		}
		return guid
	}

	out := make([]scoreEntry, 0, len(drifts)+len(sessions))
	for _, r := range drifts {
		sc := r.score
		out = append(out, scoreEntry{
			Id:         "d" + strconv.FormatInt(r.id, 10),
			Guid:       r.guid,
			Driver:     driverName(r.guid),
			Kind:       "drift",
			Date:       r.endedAt,
			Track:      resolveTrack(r.trackKey, r.trackConfig, trackInfo),
			Car:        resolveCar(r.carKey, "", carNames),
			Online:     live[r.guid],
			DriftScore: &sc,
			Clip:       clipForRun(clipsByGuid[r.guid], r.endedAt),
		})
	}
	for _, s := range sessions {
		if s.bestLapMs <= 0 {
			continue
		}
		bl := s.bestLapMs
		e := scoreEntry{
			Id:        "l" + strconv.FormatInt(s.id, 10),
			Guid:      s.guid,
			Driver:    driverName(s.guid),
			Kind:      "lap",
			Date:      s.endedAt,
			Track:     resolveTrack(s.trackKey, s.trackConfig, trackInfo),
			Car:       resolveCar(s.carKey, s.skinKey, carNames),
			Online:    live[s.guid],
			BestLapMs: &bl,
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
	media, err := Dba.selectDriverMedia(guid)
	if err != nil {
		return nil, err
	}
	det.Media = media
	det.Stream = streamForGuid(guid)
	return det, nil
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
