package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Values belong to this session. Existing domain editors can edit these structs
// without ever mutating a shared preset. Persist the whole draft in one request.
type DrivingSetup struct {
	CustomINI   string            `json:"custom_ini"`
	TrackKey    string            `json:"track_key"`
	TrackConfig string            `json:"track_config"`
	RaceLaps    int               `json:"race_laps"`
	Difficulty  UserDifficulty    `json:"difficulty"`
	Phases      UserSession       `json:"phases"`
	Conditions  timeUpdateRequest `json:"conditions"`
	Grid        UserClass         `json:"grid"`
	Scoring     DriftScoringMode  `json:"scoring"`
}
type DrivingSession struct {
	ID          int          `json:"id"`
	Revision    int          `json:"revision"`
	Name        string       `json:"name"`
	Experience  string       `json:"experience"`
	Lifecycle   string       `json:"lifecycle"`
	Setup       DrivingSetup `json:"setup"`
	EventID     *int         `json:"event_id"`
	InstanceID  *int         `json:"instance_id"`
	ScheduledAt *int64       `json:"scheduled_at"`
	TimeZone    string       `json:"time_zone"`
	CreatedAt   int64        `json:"created_at"`
	StartedAt   *int64       `json:"started_at"`
	EndedAt     *int64       `json:"ended_at"`
	Failure     string       `json:"failure"`
}

const drivingSessionColumns = "id,revision,name,experience,lifecycle,setup,event_id,instance_id,scheduled_at,time_zone,created_at,started_at,ended_at,failure"

func scanDrivingSession(row interface{ Scan(...any) error }) (DrivingSession, error) {
	var s DrivingSession
	var setup string
	err := row.Scan(&s.ID, &s.Revision, &s.Name, &s.Experience, &s.Lifecycle, &setup, &s.EventID, &s.InstanceID, &s.ScheduledAt, &s.TimeZone, &s.CreatedAt, &s.StartedAt, &s.EndedAt, &s.Failure)
	if err == nil {
		err = json.Unmarshal([]byte(setup), &s.Setup)
	}
	return s, err
}
func apiDrivingSessions(c *gin.Context) {
	rows, err := Dba.db.Query("SELECT " + drivingSessionColumns + " FROM driving_session ORDER BY created_at DESC,id DESC")
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer rows.Close()
	sessions := []DrivingSession{}
	for rows.Next() {
		s, e := scanDrivingSession(rows)
		if e != nil {
			apiDbError(c, e)
			return
		}
		sessions = append(sessions, s)
	}
	if err = rows.Err(); err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"sessions": sessions})
}
func apiDrivingSession(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	s, err := scanDrivingSession(Dba.db.QueryRow("SELECT "+drivingSessionColumns+" FROM driving_session WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		apiNotFound(c)
		return
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, s)
}
func textPtr(value string) *string { return &value }
func apiDrivingDefaults(c *gin.Context) {
	experience := c.DefaultQuery("experience", "drift")
	if experience != "drift" && experience != "race" && experience != "practice" {
		apiBadRequest(c, "Choose Drift, Race or Practice")
		return
	}
	tracks, err := Dba.selectCacheTracks()
	if err != nil {
		apiDbError(c, err)
		return
	}
	cars, err := Dba.selectCacheCars()
	if err != nil {
		apiDbError(c, err)
		return
	}
	weathers, err := Dba.selectCacheWeathers()
	if err != nil {
		apiDbError(c, err)
		return
	}
	if len(tracks) == 0 || len(cars) == 0 || len(weathers) == 0 {
		apiError(c, 409, "missing_content", "Install at least one track layout, car with a skin, and weather before creating a session.")
		return
	}
	// Stable defaults from the installed catalogue. Prefer explicitly tagged drift
	// content only when present; never substitute another layout during validation.
	sort.SliceStable(tracks, func(i, j int) bool { return derefOrEmpty(tracks[i].Name) < derefOrEmpty(tracks[j].Name) })
	sort.SliceStable(cars, func(i, j int) bool { return derefOrEmpty(cars[i].Name) < derefOrEmpty(cars[j].Name) })
	if experience == "drift" {
		sort.SliceStable(cars, func(i, j int) bool {
			return strings.Contains(strings.ToLower(derefOrEmpty(cars[i].Name)), "drift") && !strings.Contains(strings.ToLower(derefOrEmpty(cars[j].Name)), "drift")
		})
	}
	var car CacheCar
	for _, candidate := range cars {
		if contentArchived("car", derefOrEmpty(candidate.Key), "") {
			continue
		}
		detail, e := Dba.selectCacheCar(derefOrEmpty(candidate.Key))
		if e == nil && len(detail.Skins) > 0 {
			car = detail
			break
		}
	}
	if car.Key == nil {
		apiError(c, 409, "missing_skins", "No installed car has a usable skin. Inspect Cars & tracks.")
		return
	}
	availableTracks := tracks[:0]
	for _, t := range tracks {
		if !contentArchived("track", derefOrEmpty(t.Key), derefOrEmpty(t.Config)) {
			availableTracks = append(availableTracks, t)
		}
	}
	availableWeather := weathers[:0]
	for _, w := range weathers {
		if !contentArchived("weather", derefOrEmpty(w.Key), "") {
			availableWeather = append(availableWeather, w)
		}
	}
	tracks, weathers = availableTracks, availableWeather
	if len(tracks) == 0 || len(weathers) == 0 {
		apiError(c, 409, "archived_content", "Unarchive a track layout and weather panel in Cars & tracks before creating a session.")
		return
	}
	track := tracks[0]
	count := 8
	if track.Pitboxes != nil && *track.Pitboxes < count {
		count = *track.Pitboxes
	}
	cfg, _ := Dba.selectConfig()
	if cfg.MaxClients != nil && *cfg.MaxClients < count {
		count = *cfg.MaxClients
	}
	if count < 1 {
		apiError(c, 409, "invalid_capacity", "The selected track or server has no usable grid slots.")
		return
	}
	setup := DrivingSetup{TrackKey: derefOrEmpty(track.Key), TrackConfig: derefOrEmpty(track.Config),
		Difficulty: UserDifficulty{Name: textPtr("Session assists"), StartRule: intPtr(0), BlacklistMode: intPtr(0), StabilityAllowed: intPtr(0), ForceVirtualMirror: intPtr(0), RaceGasPenalityDisabled: intPtr(0), DynamicTrack: intPtr(1), Randomness: intPtr(0), MaxBallastKg: intPtr(0), KickQuorum: intPtr(70), VotingQuorum: intPtr(70), VoteDuration: intPtr(30), AbsAllowed: intPtr(1), TcAllowed: intPtr(1), AutoclutchAllowed: intPtr(1), TyreBlanketsAllowed: intPtr(1), FuelRate: intPtr(100), DamageMultiplier: intPtr(30), TyreWearRate: intPtr(100), AllowedTyresOut: intPtr(4), SessionStart: intPtr(100), SessionTransfer: intPtr(100), LapGain: intPtr(10)},
		Phases:     UserSession{Name: textPtr("Driving phases"), PracticeEnabled: intPtr(1), PracticeTime: intPtr(60), PracticeIsOpen: intPtr(1), QualifyEnabled: intPtr(0), RaceEnabled: intPtr(0)},
		Conditions: timeUpdateRequest{Name: textPtr("Afternoon"), Time: textPtr("16:00"), TimeOfDayMulti: intPtr(1), CspEnabled: intPtr(0), Weathers: []timeWeatherRequest{{Graphics: weathers[0].Key, BaseTemperatureAmbient: intPtr(20), BaseTemperatureRoad: intPtr(7), VariationAmbient: intPtr(2), VariationRoad: intPtr(2), WindBaseSpeedMin: intPtr(0), WindBaseSpeedMax: intPtr(5), WindBaseDirection: intPtr(0), WindVariationDirection: intPtr(0)}}},
		Grid:       UserClass{Name: textPtr("Session grid"), Entries: []UserClassEntry{{CacheCarKey: car.Key, SkinKey: textPtr(car.Skins[0].Key), Count: intPtr(count), Ballast: intPtr(0)}}}, Scoring: driftModeDefaults()}
	if experience == "race" {
		setup.Phases.PracticeTime = intPtr(15)
		setup.Phases.QualifyEnabled = intPtr(1)
		setup.Phases.QualifyTime = intPtr(10)
		setup.Phases.QualifyIsOpen = intPtr(1)
		setup.Phases.RaceEnabled = intPtr(1)
		setup.Phases.RaceTime = intPtr(20)
		setup.Phases.RaceIsOpen = intPtr(1)
		setup.Phases.RaceWaitTime = intPtr(60)
		setup.Phases.RaceOverTime = intPtr(120)
	}
	c.JSON(200, DrivingSession{Name: strings.ToUpper(experience[:1]) + experience[1:] + " at " + derefOrEmpty(track.Name), Experience: experience, Lifecycle: "draft", Setup: setup, TimeZone: "UTC"})
}
func validateDrivingSetup(s DrivingSession, instance *Instance) error {
	if len(strings.TrimSpace(s.Name)) == 0 || len(s.Name) > 160 {
		return errors.New("Give this session a name of 1–160 characters")
	}
	switch s.Experience {
	case "drift", "race", "practice":
	default:
		return errors.New("Choose Drift, Race or Practice")
	}
	st, overrideErr := applyDrivingOverrides(s.Setup)
	if overrideErr != nil {
		return overrideErr
	}
	if err := validateDrivingValues(st); err != nil {
		return err
	}
	track, err := Dba.selectCacheTrack(st.TrackKey, st.TrackConfig)
	if err != nil {
		return errors.New("This exact track layout is not installed")
	}
	if track.Pitboxes == nil || *track.Pitboxes < 1 {
		return errors.New("The track layout has no valid pitbox capacity")
	}
	cfg, err := Dba.selectConfig()
	if err != nil {
		return err
	}
	max := *track.Pitboxes
	if cfg.MaxClients != nil && *cfg.MaxClients < max {
		max = *cfg.MaxClients
	}
	if instance != nil && spectatorSlotConfigured(instance.Conf) {
		max--
	}
	total := 0
	if len(st.Grid.Entries) == 0 || len(st.Grid.Entries) > 100 {
		return errors.New("Choose between 1 and 100 grid rows")
	}
	for _, entry := range st.Grid.Entries {
		if entry.Count == nil || *entry.Count < 1 || *entry.Count > 100 {
			return errors.New("Every grid row needs a count between 1 and 100")
		}
		total += *entry.Count
		car, e := Dba.selectCacheCar(derefOrEmpty(entry.CacheCarKey))
		if e != nil {
			return fmt.Errorf("Car %s is not installed", derefOrEmpty(entry.CacheCarKey))
		}
		found := false
		for _, skin := range car.Skins {
			if skin.Key == derefOrEmpty(entry.SkinKey) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("The selected skin for %s is not installed", derefOrEmpty(car.Name))
		}
		if entry.Ballast != nil && (*entry.Ballast < 0 || *entry.Ballast > 1000) {
			return errors.New("Ballast must be between 0 and 1,000 kg")
		}
	}
	if total > max {
		return fmt.Errorf("The grid has %d cars but this layout/server has %d available slots. Reduce the grid or choose another layout", total, max)
	}
	if len(st.Conditions.Weathers) < 1 || len(st.Conditions.Weathers) > 32 {
		return errors.New("Choose 1–32 weather panels")
	}
	for _, w := range st.Conditions.Weathers {
		if _, e := Dba.selectCacheWeather(derefOrEmpty(w.Graphics)); e != nil {
			return errors.New("A selected weather panel is not installed")
		}
	}
	if _, err = time.Parse("15:04", derefOrEmpty(st.Conditions.Time)); err != nil {
		return errors.New("Choose a valid time of day")
	}
	if st.RaceLaps < 0 || st.RaceLaps > 10000 {
		return errors.New("Race laps must be between 0 and 10,000")
	}
	if !isEnabled(st.Phases.PracticeEnabled) && !isEnabled(st.Phases.QualifyEnabled) && !isEnabled(st.Phases.RaceEnabled) && !isEnabled(st.Phases.BookingEnabled) {
		return errors.New("Enable at least one driving phase")
	}
	for _, duration := range []*int{st.Phases.BookingTime, st.Phases.PracticeTime, st.Phases.QualifyTime, st.Phases.RaceTime} {
		if duration != nil && (*duration < 0 || *duration > 1440) {
			return errors.New("Phase durations must be between 0 and 1,440 minutes")
		}
	}
	return nil
}

// Validate whole-value drafts at the API boundary, including collapsed editors.
// Bounds match the existing driving controls. Nil retains renderer defaults.
func validateDrivingValues(s DrivingSetup) error {
	d, p, c, m := s.Difficulty, s.Phases, s.Conditions, s.Scoring
	checks := []struct {
		name     string
		value    *int
		min, max int
	}{
		{"ABS", d.AbsAllowed, 0, 2}, {"Traction control", d.TcAllowed, 0, 2},
		{"Stability", d.StabilityAllowed, 0, 1}, {"Auto clutch", d.AutoclutchAllowed, 0, 1}, {"Tyre blankets", d.TyreBlanketsAllowed, 0, 1}, {"Virtual mirror", d.ForceVirtualMirror, 0, 1},
		{"Fuel rate", d.FuelRate, 0, 500}, {"Damage rate", d.DamageMultiplier, 0, 100}, {"Tyre wear", d.TyreWearRate, 0, 500}, {"Tyres out", d.AllowedTyresOut, 0, 4}, {"Maximum ballast", d.MaxBallastKg, 0, 1000},
		{"Start rule", d.StartRule, 0, 2}, {"Gas penalty", d.RaceGasPenalityDisabled, 0, 1}, {"Dynamic track", d.DynamicTrack, 0, 1}, {"Track preset", d.DynamicTrackPreset, 0, 6},
		{"Start grip", d.SessionStart, 0, 100}, {"Grip variation", d.Randomness, 0, 100}, {"Grip transfer", d.SessionTransfer, 0, 100}, {"Grip lap gain", d.LapGain, 0, 100000},
		{"Kick quorum", d.KickQuorum, 0, 100}, {"Vote quorum", d.VotingQuorum, 0, 100}, {"Vote duration", d.VoteDuration, 0, 3600}, {"Blacklist mode", d.BlacklistMode, 0, 1}, {"Collisions per km", d.MaxContactsPerKm, -1, 100000},
		{"Booking", p.BookingEnabled, 0, 1}, {"Practice", p.PracticeEnabled, 0, 1}, {"Qualifying", p.QualifyEnabled, 0, 1}, {"Race", p.RaceEnabled, 0, 1}, {"Practice joining", p.PracticeIsOpen, 0, 1}, {"Qualifying joining", p.QualifyIsOpen, 0, 1}, {"Race joining", p.RaceIsOpen, 0, 2},
		{"Qualifying wait", p.QualifyMaxWaitPerc, 0, 1000}, {"Extra lap", p.RaceExtraLap, 0, 1}, {"Race overtime", p.RaceOverTime, 0, 86400}, {"Race wait", p.RaceWaitTime, 0, 86400}, {"Reversed grid", p.ReversedGridPositions, -1, 1000}, {"Pit window start", p.RacePitWindowStart, 0, 10000}, {"Pit window end", p.RacePitWindowEnd, 0, 10000},
		{"CSP weather", c.CspEnabled, 0, 1}, {"Time multiplier", c.TimeOfDayMulti, 1, 10},
		{"Collision reset", m.CollisionResetScore, 0, 1}, {"Car collision reset", m.CarCollisionResetScore, 0, 1}, {"Score reset", m.ResetScoreEnabled, 0, 1}, {"Multiplier reset", m.ResetMultiplierEnabled, 0, 1}, {"Multiplier cap", m.MultiplierCap, 1, 50},
	}
	for i, w := range c.Weathers {
		checks = append(checks, []struct {
			name     string
			value    *int
			min, max int
		}{
			{"Ambient temperature", w.BaseTemperatureAmbient, -20, 50}, {"Road temperature", w.BaseTemperatureRoad, -20, 50}, {"Ambient variation", w.VariationAmbient, 0, 20}, {"Road variation", w.VariationRoad, 0, 20},
			{"Minimum wind", w.WindBaseSpeedMin, 0, 40}, {"Maximum wind", w.WindBaseSpeedMax, 0, 40}, {"Wind direction", w.WindBaseDirection, 0, 359}, {"Wind variation", w.WindVariationDirection, 0, 359}, {"CSP time multiplier", w.CspTimeOfDayMulti, 0, 60},
		}...)
		if w.WindBaseSpeedMin != nil && w.WindBaseSpeedMax != nil && *w.WindBaseSpeedMin > *w.WindBaseSpeedMax {
			return fmt.Errorf("Weather panel %d: minimum wind exceeds maximum", i+1)
		}
		if isEnabled(c.CspEnabled) {
			if _, err := time.Parse("15:04", derefOrEmpty(w.CspTime)); err != nil {
				return fmt.Errorf("Weather panel %d: choose a valid CSP time", i+1)
			}
			if date := derefOrEmpty(w.CspDate); date != "" {
				if _, err := time.Parse("2006-01-02", date); err != nil {
					return fmt.Errorf("Weather panel %d: choose a valid date", i+1)
				}
			}
		}
	}
	for _, v := range checks {
		if v.value != nil && (*v.value < v.min || *v.value > v.max) {
			return fmt.Errorf("%s must be between %d and %d", v.name, v.min, v.max)
		}
	}
	for _, v := range []struct {
		name     string
		value    *float64
		min, max float64
	}{
		{"Score reset seconds", m.ResetScoreSeconds, 0, 10}, {"Multiplier reset seconds", m.ResetMultiplierSeconds, 0, 10}, {"Minimum drift speed", m.MinSpeedKmh, 0, 200}, {"Minimum drift angle", m.MinAngleDeg, 0, 60}, {"Angle weight", m.AngleWeight, 0, .05}, {"Speed weight", m.SpeedWeight, 0, .02}, {"Proximity weight", m.ProximityWeight, 0, 5}, {"Proximity range", m.ProximityRangeM, .1, 20}, {"Multiplier gain", m.MultiplierGain, 0, 1},
	} {
		if v.value != nil && (*v.value < v.min || *v.value > v.max) {
			return fmt.Errorf("%s must be between %g and %g", v.name, v.min, v.max)
		}
	}
	if p.RacePitWindowStart != nil && p.RacePitWindowEnd != nil && *p.RacePitWindowEnd > 0 && *p.RacePitWindowStart > *p.RacePitWindowEnd {
		return errors.New("Pit window must end after it starts")
	}
	return nil
}

// insertPrivatePreset maps only declared struct fields, never client-supplied SQL
// identifiers. Child rows use the renderer's existing tables and ownership keys.
func insertPrivatePreset(tx *sql.Tx, table string, value any, extra map[string]any) (int, error) {
	cols := []string{}
	args := []any{}
	marks := []string{}
	v := reflect.ValueOf(value)
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if key == "" || key == "id" || key == "name" || key == "weathers" || key == "entries" || key == "user_class_id" {
			continue
		}
		if _, exists := extra[key]; exists {
			continue
		}
		cols = append(cols, key)
		args = append(args, v.Field(i).Interface())
		marks = append(marks, "?")
	}
	keys := []string{}
	for key := range extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cols = append(cols, key)
		args = append(args, extra[key])
		marks = append(marks, "?")
	}
	result, err := tx.Exec("INSERT INTO "+table+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return int(id), err
}
func savePrivateSetup(tx *sql.Tx, s DrivingSession) (int, error) {
	name := "Pitlane · " + s.Name
	ids := []int{}
	for _, p := range []struct {
		table string
		value any
	}{{"user_difficulty", s.Setup.Difficulty}, {"user_session", s.Setup.Phases}, {"user_time", s.Setup.Conditions}, {"user_class", s.Setup.Grid}, {"drift_scoring_mode", s.Setup.Scoring}} {
		extra := map[string]any{"name": name}
		if p.table != "drift_scoring_mode" {
			extra["filled"] = 1
		}
		id, err := insertPrivatePreset(tx, p.table, p.value, extra)
		if err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	for _, w := range s.Setup.Conditions.Weathers {
		if _, err := insertPrivatePreset(tx, "user_time_weather", w, map[string]any{"user_time_id": ids[2]}); err != nil {
			return 0, err
		}
	}
	for _, g := range s.Setup.Grid.Entries {
		_, err := tx.Exec("INSERT INTO user_class_entry(user_class_id,cache_car_key,skin_key,car_count,ballast) VALUES(?,?,?,?,?)", ids[3], g.CacheCarKey, g.SkinKey, g.Count, g.Ballast)
		if err != nil {
			return 0, err
		}
	}
	var category int
	err := tx.QueryRow("SELECT id FROM user_event_category WHERE name='Pitlane sessions' ORDER BY id LIMIT 1").Scan(&category)
	if errors.Is(err, sql.ErrNoRows) {
		result, e := tx.Exec("INSERT INTO user_event_category(name) VALUES('Pitlane sessions')")
		if e != nil {
			return 0, e
		}
		id, _ := result.LastInsertId()
		category = int(id)
	} else if err != nil {
		return 0, err
	}
	result, err := tx.Exec("INSERT INTO user_event(event_category_id,cache_track_key,cache_track_config,difficulty_id,session_id,time_id,class_id,drift_scoring_mode_id,name,race_laps,strategy) VALUES(?,?,?,?,?,?,?,?,?,?,1)", category, s.Setup.TrackKey, s.Setup.TrackConfig, ids[0], ids[1], ids[2], ids[3], ids[4], s.Name, s.Setup.RaceLaps)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	setup, err := json.Marshal(s.Setup)
	if err == nil {
		_, err = tx.Exec("INSERT INTO driving_setup(event_id,name,experience,setup) VALUES(?,?,?,?)", id, s.Name, s.Experience, string(setup))
	}
	return int(id), err
}
func apiDrivingSessionSave(c *gin.Context) {
	var s DrivingSession
	if c.ShouldBindJSON(&s) != nil {
		apiBadRequest(c, "Invalid driving session")
		return
	}
	if err := validateDrivingSetup(s, nil); err != nil {
		apiBadRequest(c, err.Error())
		return
	}
	setup, err := json.Marshal(s.Setup)
	if err != nil {
		apiBadRequest(c, "Invalid setup")
		return
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer tx.Rollback()
	s.ID = 0 // The route, never a client-supplied body ID, decides create versus edit.
	if c.Param("id") != "" {
		id, ok := pathId(c)
		if !ok {
			return
		}
		s.ID = id
		var revision int
		var lifecycle string
		err = tx.QueryRow("SELECT revision,lifecycle FROM driving_session WHERE id=?", id).Scan(&revision, &lifecycle)
		if err != nil {
			apiNotFound(c)
			return
		}
		if revision != s.Revision || lifecycle != "draft" {
			apiError(c, 409, "revision_conflict", "This session changed or is already planned. Reload, or use Run it again to make a new draft.")
			return
		}
	}
	eventID, err := savePrivateSetup(tx, s)
	if err != nil {
		apiDbError(c, err)
		return
	}
	var result sql.Result
	if s.ID == 0 {
		result, err = tx.Exec("INSERT INTO driving_session(name,experience,setup,event_id,created_at) VALUES(?,?,?,?,?)", strings.TrimSpace(s.Name), s.Experience, string(setup), eventID, time.Now().UnixMilli())
		if err == nil {
			id, _ := result.LastInsertId()
			s.ID = int(id)
		}
	} else {
		_, err = tx.Exec("UPDATE driving_session SET name=?,experience=?,setup=?,event_id=?,revision=revision+1 WHERE id=?", s.Name, s.Experience, string(setup), eventID, s.ID)
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	if err = tx.Commit(); err != nil {
		apiDbError(c, err)
		return
	}
	saved, err := scanDrivingSession(Dba.db.QueryRow("SELECT "+drivingSessionColumns+" FROM driving_session WHERE id=?", s.ID))
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, saved)
}

type LaunchRequest struct {
	Key         string `json:"idempotency_key"`
	Revision    int    `json:"expected_revision"`
	InstanceID  int    `json:"instance_id"`
	Mode        string `json:"mode"`
	ScheduledAt *int64 `json:"scheduled_at"`
	TimeZone    string `json:"time_zone"`
}

// Serializes launch requests through their durable intent. Process operations on
// different instances remain independent after request completion.
// ponytail: small club launch volume; use keyed request locks if this becomes busy.
var drivingLaunchMu sync.Mutex

func apiDrivingSessionLaunch(c *gin.Context) {
	drivingLaunchMu.Lock()
	defer drivingLaunchMu.Unlock()
	contentLifecycleMu.RLock()
	defer contentLifecycleMu.RUnlock()
	id, ok := pathId(c)
	if !ok {
		return
	}
	var req LaunchRequest
	if c.ShouldBindJSON(&req) != nil || len(req.Key) < 16 || len(req.Key) > 128 {
		apiBadRequest(c, "A durable idempotency key is required")
		return
	}
	if req.Mode == "" {
		req.Mode = "now"
	}
	if req.Mode != "now" && req.Mode != "after" && req.Mode != "later" {
		apiBadRequest(c, "Choose now, after current, or later")
		return
	}
	if req.TimeZone == "" {
		req.TimeZone = "UTC"
	}
	if _, err := time.LoadLocation(req.TimeZone); err != nil {
		apiBadRequest(c, "Choose a valid IANA time zone")
		return
	}

	// The database record is the idempotency authority, including across restarts.
	var existingSession, execution int
	var state, oldRequest, oldFailure string
	requestJSON, _ := json.Marshal(req)
	err := Dba.db.QueryRow("SELECT id,session_id,state,request_json,failure FROM driving_execution WHERE idempotency_key=?", req.Key).Scan(&execution, &existingSession, &state, &oldRequest, &oldFailure)
	if err == nil {
		if existingSession != id || (oldRequest != "" && oldRequest != string(requestJSON)) {
			apiError(c, 409, "idempotency_conflict", "This operation key belongs to a different request. Review the saved session before making a new attempt.")
			return
		}
		c.JSON(200, gin.H{"execution_id": execution, "session_id": id, "state": state, "failure": oldFailure})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		apiDbError(c, err)
		return
	}
	if req.Mode == "later" && (req.ScheduledAt == nil || *req.ScheduledAt <= time.Now().UnixMilli()) {
		apiBadRequest(c, "Choose a future scheduled time")
		return
	}
	s, err := scanDrivingSession(Dba.db.QueryRow("SELECT "+drivingSessionColumns+" FROM driving_session WHERE id=?", id))
	if err != nil {
		apiNotFound(c)
		return
	}
	inst, err := selectDrivingInstance(s, req.InstanceID, req.Mode)
	if err != nil {
		apiError(c, 409, "instance_conflict", err.Error())
		return
	}
	inst.operationMu.Lock()
	defer inst.operationMu.Unlock()
	if req.Mode == "now" && inst.isRunning() {
		apiError(c, 409, "instance_busy", "This server just started another session. Your draft is preserved.")
		return
	}
	if err = validateDrivingSetup(s, inst); err != nil {
		apiBadRequest(c, err.Error())
		return
	}
	if err = validateDrivingInstallation(s, inst); err != nil {
		apiError(c, 409, "installation_incomplete", err.Error())
		return
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer tx.Rollback()
	var revision int
	var lifecycle string
	err = tx.QueryRow("SELECT revision,lifecycle FROM driving_session WHERE id=?", id).Scan(&revision, &lifecycle)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if revision != req.Revision || lifecycle != "draft" {
		apiError(c, 409, "revision_conflict", "This session changed or has already been planned. Reload to review the current state.")
		return
	}
	var pending int
	err = tx.QueryRow("SELECT count(*) FROM server_event WHERE instance_id=? AND finished=0", inst.Id()).Scan(&pending)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if req.Mode == "now" && pending > 0 {
		apiError(c, 409, "queue_conflict", "Another session was queued. Review the running order; nothing was changed.")
		return
	}
	var reservation int
	err = tx.QueryRow("SELECT count(*) FROM driving_session WHERE instance_id=? AND lifecycle='starting'", inst.Id()).Scan(&reservation)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if reservation > 0 {
		apiError(c, 409, "reservation_conflict", "This server has another planned session. Choose another server.")
		return
	}
	nextState := "starting"
	if req.Mode == "later" {
		nextState = "planned"
	}
	if req.Mode == "after" {
		nextState = "queued"
	}
	var queueID *int
	if req.Mode != "later" {
		result, e := tx.Exec("INSERT INTO server_event(user_event_id,instance_id,orderby) SELECT ?,?,COALESCE(MAX(orderby),0)+1 FROM server_event WHERE instance_id=?", s.EventID, inst.Id(), inst.Id())
		if e != nil {
			apiDbError(c, e)
			return
		}
		qid, _ := result.LastInsertId()
		q := int(qid)
		queueID = &q
	}
	snapshot, _ := json.Marshal(s.Setup)
	result, err := tx.Exec("INSERT INTO driving_execution(session_id,instance_id,queue_id,idempotency_key,expected_revision,state,setup_snapshot,requested_at,request_json) VALUES(?,?,?,?,?,?,?,?,?)", id, inst.Id(), queueID, req.Key, s.Revision, nextState, string(snapshot), time.Now().UnixMilli(), string(requestJSON))
	if err != nil {
		apiDbError(c, err)
		return
	}
	eid, _ := result.LastInsertId()
	execution = int(eid)
	if req.Mode == "now" {
		if _, err = tx.Exec("INSERT INTO instance_launch(instance_id,execution_id) VALUES(?,?)", inst.Id(), execution); err != nil {
			apiError(c, 409, "launch_conflict", "Another launch reserved this server.")
			return
		}
	}
	_, err = tx.Exec("UPDATE driving_session SET lifecycle=?,instance_id=?,scheduled_at=?,time_zone=? WHERE id=?", nextState, inst.Id(), req.ScheduledAt, req.TimeZone, id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if err = tx.Commit(); err != nil {
		apiDbError(c, err)
		return
	}
	if req.Mode == "now" {
		if err = launchDrivingExecution(inst, execution, *queueID); err != nil {
			apiError(c, 409, "launch_failed", err.Error())
			return
		}
		nextState = "live"
	}
	c.JSON(200, gin.H{"execution_id": execution, "session_id": id, "instance_id": inst.Id(), "state": nextState})
}
func launchDrivingExecution(inst *Instance, execution, queue int) error {
	se, err := Dba.selectServerEvent(queue)
	if err == nil {
		var ok bool
		ok, err = applyServerEvent(inst, se)
		if !ok && err == nil {
			err = errors.New("The server could not apply this exact session")
		}
	}
	if err == nil {
		err = inst.start()
	}
	if err != nil {
		tx, e := Dba.db.Begin()
		if e != nil {
			return e
		}
		defer tx.Rollback()
		now := time.Now().UnixMilli()
		if _, e = tx.Exec("UPDATE driving_execution SET state='failed',failure=?,ended_at=? WHERE id=? AND state NOT IN ('finished','cancelled')", err.Error(), now, execution); e != nil {
			return e
		}
		if _, e = tx.Exec("UPDATE driving_session SET lifecycle='failed',failure=?,ended_at=? WHERE id=(SELECT session_id FROM driving_execution WHERE id=?)", err.Error(), now, execution); e != nil {
			return e
		}
		if _, e = tx.Exec("UPDATE server_event SET finished=1 WHERE id=?", queue); e != nil {
			return e
		}
		if _, e = tx.Exec("DELETE FROM instance_launch WHERE execution_id=?", execution); e != nil {
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		inst.mu.Lock()
		inst.executionID = 0
		inst.mu.Unlock()
		return err
	}
	_, err = Dba.db.Exec("DELETE FROM instance_launch WHERE execution_id=?", execution)
	return err
}

func (cr *ConfigRenderer) sessionTime(event UserEvent) (UserTime, error) {
	if cr.draft == nil {
		return Dba.selectTimeWeather(*event.TimeId)
	}
	effective, err := applyDrivingOverrides(*cr.draft)
	if err != nil {
		return UserTime{}, err
	}
	s := effective.Conditions
	out := UserTime{Name: s.Name, Time: s.Time, TimeOfDayMulti: s.TimeOfDayMulti, CspEnabled: s.CspEnabled}
	for _, w := range s.Weathers {
		out.Weathers = append(out.Weathers, UserTimeWeather{Graphics: w.Graphics, BaseTemperatureAmbient: w.BaseTemperatureAmbient, BaseTemperatureRoad: w.BaseTemperatureRoad, VariationAmbient: w.VariationAmbient, VariationRoad: w.VariationRoad, WindBaseSpeedMin: w.WindBaseSpeedMin, WindBaseSpeedMax: w.WindBaseSpeedMax, WindBaseDirection: w.WindBaseDirection, WindVariationDirection: w.WindVariationDirection, CspTime: w.CspTime, CspTimeOfDayMulti: w.CspTimeOfDayMulti, CspDate: w.CspDate})
	}
	return out, nil
}
func (cr *ConfigRenderer) sessionDifficulty(event UserEvent) (UserDifficulty, error) {
	if cr.draft != nil {
		effective, err := applyDrivingOverrides(*cr.draft)
		if err != nil {
			return UserDifficulty{}, err
		}
		return effective.Difficulty, nil
	}
	return Dba.selectDifficulty(*event.DifficultyId)
}
func (cr *ConfigRenderer) sessionPhases(event UserEvent) (UserSession, error) {
	if cr.draft != nil {
		effective, err := applyDrivingOverrides(*cr.draft)
		if err != nil {
			return UserSession{}, err
		}
		return effective.Phases, nil
	}
	return Dba.selectSession(*event.SessionId)
}
func (cr *ConfigRenderer) sessionGrid(event UserEvent) (UserClass, error) {
	if cr.draft != nil {
		return cr.draft.Grid, nil
	}
	return Dba.selectClassEntries(*event.ClassId)
}
func selectDrivingInstance(s DrivingSession, instanceID int, mode string) (*Instance, error) {
	var inst *Instance
	for _, candidate := range Instances.All() {
		if instanceID != 0 && instanceID != candidate.Id() {
			continue
		}
		if !isEnabled(candidate.Conf.Enabled) {
			continue
		}
		if mode == "now" && candidate.isRunning() {
			continue
		}
		if _, repeat := candidate.repeatEventId(); repeat {
			continue
		}
		var pending int
		_ = Dba.db.QueryRow("SELECT count(*) FROM server_event WHERE instance_id=? AND finished=0", candidate.Id()).Scan(&pending)
		if mode == "now" && pending > 0 {
			continue
		}
		if candidate.Conf.ScheduledStart != nil && *candidate.Conf.ScheduledStart > 0 {
			continue
		}
		var reserved int
		_ = Dba.db.QueryRow("SELECT count(*) FROM driving_session WHERE instance_id=? AND lifecycle='starting'", candidate.Id()).Scan(&reserved)
		if reserved > 0 {
			continue
		}
		if e := validateDrivingSetup(s, candidate); e != nil {
			if instanceID != 0 {
				return nil, e
			}
			continue
		}
		inst = candidate
		break
	}
	if inst == nil {
		return nil, errors.New("No compatible server is free for this plan. Choose another server, review its running order, or queue after the current session.")
	}
	return inst, nil
}

func apiDrivingPreview(c *gin.Context) {
	var s DrivingSession
	if c.ShouldBindJSON(&s) != nil {
		apiBadRequest(c, "Invalid driving session")
		return
	}
	instanceID := 0
	if s.InstanceID != nil {
		instanceID = *s.InstanceID
	}
	mode := c.DefaultQuery("mode", "now")
	if mode != "now" && mode != "after" && mode != "later" {
		apiBadRequest(c, "Choose a valid start mode")
		return
	}
	inst, err := selectDrivingInstance(s, instanceID, mode)
	if err != nil {
		apiError(c, 409, "instance_conflict", err.Error())
		return
	}
	if err := validateDrivingSetup(s, inst); err != nil {
		apiBadRequest(c, err.Error())
		return
	}
	cr := ConfigRenderer{draft: &s.Setup}
	zero := 0
	one := 1
	cr.renderEvent(UserEvent{Name: &s.Name, CacheTrackKey: &s.Setup.TrackKey, CacheTrackConfig: &s.Setup.TrackConfig, TimeId: &zero, SessionId: &zero, DifficultyId: &zero, ClassId: &zero, RaceLaps: &s.Setup.RaceLaps, Strategy: &one}, inst.Conf, nil, "")
	if cr.renderErr != nil {
		apiBadRequest(c, cr.renderErr.Error())
		return
	}
	count := 0
	for _, g := range s.Setup.Grid.Entries {
		count += *g.Count
	}
	c.JSON(200, gin.H{"instance_id": inst.Id(), "server_cfg": cr.serverCfgResult, "entry_list": cr.entryListResult, "grid_count": count, "requested_grid": count, "pitboxes": cr.track.Pitboxes, "max_clients": cr.maxClients, "warnings": []string{}, "render_errors": []string{}})
}
func apiDrivingFinish(c *gin.Context) {
	drivingLaunchMu.Lock()
	defer drivingLaunchMu.Unlock()
	id, ok := pathId(c)
	if !ok {
		return
	}
	s, err := scanDrivingSession(Dba.db.QueryRow("SELECT "+drivingSessionColumns+" FROM driving_session WHERE id=?", id))
	if err != nil {
		apiNotFound(c)
		return
	}
	if s.Lifecycle == "finished" || s.Lifecycle == "cancelled" {
		c.JSON(200, s)
		return
	}
	if s.InstanceID != nil {
		inst := Instances.Get(*s.InstanceID)
		if inst != nil {
			inst.operationMu.Lock()
			defer inst.operationMu.Unlock()
			if inst.isRunning() && (s.Lifecycle == "live" || s.Lifecycle == "starting") {
				inst.mu.Lock()
				execution := inst.executionID
				inst.mu.Unlock()
				var owner int
				_ = Dba.db.QueryRow("SELECT session_id FROM driving_execution WHERE id=?", execution).Scan(&owner)
				if owner != s.ID {
					apiError(c, 409, "instance_changed", "This server is running a different session. Refresh before finishing.")
					return
				}
				inst.stop()
			}
		}
	}
	next := "cancelled"
	if s.Lifecycle == "live" || s.Lifecycle == "starting" {
		next = "finished"
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if _, err = tx.Exec("DELETE FROM instance_launch WHERE execution_id IN (SELECT id FROM driving_execution WHERE session_id=?)", id); err == nil {
		_, err = tx.Exec("UPDATE server_event SET finished=1 WHERE id IN (SELECT queue_id FROM driving_execution WHERE session_id=?)", id)
	}
	if err == nil {
		_, err = tx.Exec("UPDATE driving_execution SET state=?,ended_at=? WHERE session_id=? AND state NOT IN ('finished','failed','cancelled')", next, now, id)
	}
	if err == nil {
		_, err = tx.Exec("UPDATE driving_session SET lifecycle=?,ended_at=? WHERE id=?", next, now, id)
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	if err = tx.Commit(); err != nil {
		apiDbError(c, err)
		return
	}
	s.Lifecycle = next
	s.EndedAt = &now
	c.JSON(200, s)
}

// Called only after the exact event has rendered and provisioned successfully.
// Repeat mode enters here again, creating a distinct execution every time.
func beginDrivingExecution(inst *Instance, se ServerEvent) (bool, error) {
	if se.UserEvent.Id == nil {
		return true, nil
	}
	s, err := scanDrivingSession(Dba.db.QueryRow("SELECT "+drivingSessionColumns+" FROM driving_session WHERE event_id=? ORDER BY id DESC LIMIT 1", *se.UserEvent.Id))
	if errors.Is(err, sql.ErrNoRows) {
		inst.mu.Lock()
		inst.executionID = 0
		inst.mu.Unlock()
		return true, nil
	}
	if err != nil {
		return false, err
	}
	snapshot, _ := json.Marshal(s.Setup)
	scoring, _ := json.Marshal(inst.driftMode)
	var execution int64
	err = Dba.db.QueryRow("SELECT id FROM driving_execution WHERE session_id=? AND instance_id=? AND queue_id IS ? AND state IN ('starting','queued') ORDER BY id DESC LIMIT 1", s.ID, inst.Id(), se.Id).Scan(&execution)
	if errors.Is(err, sql.ErrNoRows) {
		result, e := Dba.db.Exec("INSERT INTO driving_execution(session_id,instance_id,queue_id,expected_revision,state,setup_snapshot,requested_at) VALUES(?,?,?,?, 'starting',?,?)", s.ID, inst.Id(), se.Id, s.Revision, string(snapshot), time.Now().UnixMilli())
		if e != nil {
			return false, e
		}
		execution, err = result.LastInsertId()
	}
	if err != nil {
		return false, err
	}
	if _, err = Dba.db.Exec("UPDATE driving_execution SET server_cfg=?,entry_list=?,scoring_snapshot=? WHERE id=?", inst.Cr.serverCfgResult, inst.Cr.entryListResult, string(scoring), execution); err != nil {
		return false, err
	}
	inst.mu.Lock()
	inst.executionID = execution
	inst.mu.Unlock()
	return true, nil
}
func drivingRunning(inst *Instance, running bool) {
	inst.mu.Lock()
	execution := inst.executionID
	if !running {
		inst.executionID = 0
	}
	inst.mu.Unlock()
	if execution == 0 || Dba.db == nil {
		return
	}
	now := time.Now().UnixMilli()
	state := "finished"
	if running {
		state = "live"
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	if running {
		_, err = tx.Exec("UPDATE driving_execution SET state='live',started_at=? WHERE id=?", now, execution)
	} else {
		_, err = tx.Exec("UPDATE driving_execution SET state=CASE WHEN state='failed' THEN state ELSE 'finished' END,ended_at=? WHERE id=?", now, execution)
	}
	if err == nil {
		_, err = tx.Exec("UPDATE driving_session SET lifecycle=(SELECT state FROM driving_execution WHERE id=?),failure=(SELECT failure FROM driving_execution WHERE id=?),started_at=COALESCE(started_at,?),ended_at=CASE WHEN ?='finished' THEN ? ELSE NULL END WHERE id=(SELECT session_id FROM driving_execution WHERE id=?)", execution, execution, now, state, now, execution)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		inst.appendPitlaneError("Could not persist execution lifecycle")
	}
}
func (inst *Instance) appendPitlaneError(message string) {
	inst.mu.Lock()
	inst.lines += "\nPitlane: " + message + "\n"
	inst.mu.Unlock()
}

// Every legacy process-control entry point uses the same per-instance boundary.
func pitlaneLifecycleMiddleware(c *gin.Context) {
	path := strings.TrimPrefix(c.Request.URL.Path, "/api")
	if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" || strings.HasPrefix(path, "/driving-sessions") || path == "/queue/skipevent" {
		c.Next()
		return
	}
	// ponytail: serialize operator mutations during a launch; a club has low write
	// volume. Split by resource only if slow administrative actions require it.
	drivingLaunchMu.Lock()
	defer drivingLaunchMu.Unlock()
	if c.Request.Method != "POST" || (path != "/server/start" && path != "/server/stop" && path != "/server/restart" && path != "/server/current-event") {
		c.Next()
		return
	}
	inst, err := instanceFromRequest(c)
	if err != nil {
		apiInstanceError(c, err)
		c.Abort()
		return
	}
	contentLifecycleMu.RLock()
	defer contentLifecycleMu.RUnlock()
	inst.operationMu.Lock()
	defer inst.operationMu.Unlock()
	var reserved int
	if err = Dba.db.QueryRow("SELECT count(*) FROM instance_launch WHERE instance_id=?", inst.Id()).Scan(&reserved); err != nil {
		apiDbError(c, err)
		c.Abort()
		return
	}
	if reserved > 0 {
		apiError(c, 409, "launch_reserved", "This server has a recorded launch awaiting recovery. Open its session for status.")
		c.Abort()
		return
	}
	c.Next()
}

// Recovery never guesses whether an untracked OS process is still safe to
// restart. An interrupted launch is persisted as failed, keeping its snapshot.
func reconcileDrivingExecutions() error {
	tx, err := Dba.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UnixMilli()
	if _, err = tx.Exec("UPDATE driving_execution SET state='failed',failure='ServerManager restarted during this execution. Inspect server status before running it again.',ended_at=? WHERE state IN ('starting','live')", now); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE driving_session SET lifecycle='failed',failure='Execution interrupted by a ServerManager restart. Review server status before running it again.',ended_at=? WHERE lifecycle IN ('starting','live')", now); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE server_event SET finished=1 WHERE id IN (SELECT queue_id FROM driving_execution WHERE state='failed')"); err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE capture_job SET state='failed',message='Recording interrupted by a ServerManager restart',ended_at=? WHERE state IN ('recording','assembling')", now); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM instance_launch"); err != nil {
		return err
	}
	return tx.Commit()
}
func checkDrivingSchedules() {
	rows, err := Dba.db.Query("SELECT id,instance_id,event_id FROM driving_session WHERE (lifecycle='planned' AND scheduled_at<=?) OR lifecycle='queued' ORDER BY COALESCE(scheduled_at,0),id", time.Now().UnixMilli())
	if err != nil {
		return
	}
	type due struct{ id, instance, event int }
	list := []due{}
	for rows.Next() {
		var d due
		if rows.Scan(&d.id, &d.instance, &d.event) == nil {
			list = append(list, d)
		}
	}
	rows.Close()
	for _, d := range list {
		inst := Instances.Get(d.instance)
		if inst == nil {
			continue
		}
		func() {
			inst.operationMu.Lock()
			defer inst.operationMu.Unlock()
			if inst.isRunning() {
				return
			}
			if _, repeat := inst.repeatEventId(); repeat {
				return
			}
			tx, e := Dba.db.Begin()
			if e != nil {
				return
			}
			defer tx.Rollback()
			var count int

			var life string
			if e = tx.QueryRow("SELECT lifecycle FROM driving_session WHERE id=?", d.id).Scan(&life); e != nil || (life != "planned" && life != "queued") {
				return
			}
			var execution int
			var existingQueue *int
			if e = tx.QueryRow("SELECT id,queue_id FROM driving_execution WHERE session_id=? AND state IN ('planned','queued') ORDER BY id LIMIT 1", d.id).Scan(&execution, &existingQueue); e != nil {
				return
			}
			var qid int64
			if life == "queued" {
				if existingQueue == nil {
					return
				}
				if e = tx.QueryRow("SELECT id FROM server_event WHERE instance_id=? AND finished=0 ORDER BY orderby,id LIMIT 1", inst.Id()).Scan(&count); e != nil || count != *existingQueue {
					return
				}
				qid = int64(*existingQueue)
			} else {
				if e = tx.QueryRow("SELECT count(*) FROM server_event WHERE instance_id=? AND finished=0", inst.Id()).Scan(&count); e != nil || count > 0 {
					return
				}
				result, err := tx.Exec("INSERT INTO server_event(user_event_id,instance_id,orderby) SELECT ?,?,COALESCE(MAX(orderby),0)+1 FROM server_event WHERE instance_id=?", d.event, inst.Id(), inst.Id())
				if err != nil {
					return
				}
				qid, _ = result.LastInsertId()
			}
			if _, e = tx.Exec("UPDATE driving_execution SET state='starting',queue_id=? WHERE id=?", qid, execution); e != nil {
				return
			}
			if _, e = tx.Exec("INSERT INTO instance_launch(instance_id,execution_id) VALUES(?,?)", inst.Id(), execution); e != nil {
				return
			}
			if _, e = tx.Exec("UPDATE driving_session SET lifecycle='starting' WHERE id=?", d.id); e != nil {
				return
			}
			if e = tx.Commit(); e != nil {
				return
			}
			if e = launchDrivingExecution(inst, execution, int(qid)); e != nil {
				inst.appendPitlaneError(e.Error())
			}
		}()
	}
}

var captureOwnershipMu sync.Mutex

func liveCaptureOwner(guid string) (connectionID int64, guestDriverID int, executionID int64) {
	for _, inst := range Instances.All() {
		inst.mu.Lock()
		for _, d := range inst.drivers {
			if d.Connected && d.Guid == guid {
				connectionID, guestDriverID, executionID = d.connectionId, d.GuestDriverId, d.executionID
				inst.mu.Unlock()
				return
			}
		}
		inst.mu.Unlock()
	}
	return
}

// Handover closes the previous attribution segment while telemetry is locked.
// An active drift run or moving car must reach a boundary before ownership changes.
func (inst *Instance) handoverDriver(carID, guestID int) error {
	captureOwnershipMu.Lock()
	defer captureOwnershipMu.Unlock()
	name := ""
	if guestID > 0 {
		names, err := Dba.selectGuestDriverNames()
		if err != nil {
			return err
		}
		name = names[guestID]
		if name == "" {
			return errors.New("That guest driver no longer exists")
		}
	}
	inst.mu.Lock()
	d := inst.drivers[carID]
	if d == nil || !d.Connected {
		inst.mu.Unlock()
		return errors.New("No connected driver in this car")
	}
	if d.DriftLive > 0 || d.driftRunStartMs > 0 {
		inst.mu.Unlock()
		return errors.New("Finish the current drift run before handing over. Previous ownership has not changed")
	}
	if p := inst.positions[carID]; p != nil && (p.VelocityX*p.VelocityX+p.VelocityZ*p.VelocityZ) > 0.1 {
		inst.mu.Unlock()
		return errors.New("Bring the car to a stop before handing over. Previous ownership has not changed")
	}
	now := time.Now().UnixMilli()
	if !d.recorded && d.Guid != "" && (d.Laps > 0 || d.DriftBest > 0) {
		row := sessionRowFromDriverLocked(d, now)
		if err := Dba.insertDriverSession(inst.Id(), row); err != nil {
			inst.mu.Unlock()
			return err
		}
	}
	if Captures != nil {
		Captures.finalizeManual(d.Guid)
		sources, e := Dba.cameraSources()
		if e == nil {
			for _, source := range sources {
				if source.DriverGUID == d.Guid {
					Captures.finalizeManual(source.ID)
				}
			}
		}
	}
	if name == "" {
		name = d.accountName
		if name == "" {
			name = d.Name
		}
	}
	d.GuestDriverId = guestID
	d.Name = name
	d.joinedAt = now
	d.Laps = 0
	d.LastLapMs = 0
	d.BestLapMs = 0
	d.DriftLive = 0
	d.DriftLast = 0
	d.DriftBest = 0
	d.recorded = false
	d.captureCount = 0
	delete(inst.driftScorers, carID)
	inst.mu.Unlock()
	inst.publishDrivers()
	return nil
}

func ownedDrivingSetup(eventID int) (*DrivingSession, error) {
	var s DrivingSession
	var raw string
	err := Dba.db.QueryRow("SELECT name,experience,setup FROM driving_setup WHERE event_id=?", eventID).Scan(&s.Name, &s.Experience, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(raw), &s.Setup); err != nil {
		return nil, err
	}
	return &s, nil
}

// applyOwnedLiveSetup saves another private version for the next process start.
// The execution currently on track keeps its original effective snapshot.
func applyOwnedLiveSetup(inst *Instance, payload DashboardEventUpdate) (bool, error) {
	owned, err := ownedDrivingSetup(payload.EventID)
	if err != nil || owned == nil {
		return false, err
	}
	if inst.Cr.serverEvent.UserEvent.Id == nil || *inst.Cr.serverEvent.UserEvent.Id != payload.EventID {
		return true, errors.New("This server's current setup changed. Refresh before editing.")
	}
	inst.mu.Lock()
	execution := inst.executionID
	inst.mu.Unlock()
	s, err := scanDrivingSession(Dba.db.QueryRow("SELECT "+drivingSessionColumns+" FROM driving_session WHERE id=(SELECT session_id FROM driving_execution WHERE id=?)", execution))
	if err != nil {
		return true, errors.New("This execution has ended. Use Run it again to prepare a new session.")
	}
	if payload.Track != "" {
		parts := strings.SplitN(payload.Track, ":", 2)
		s.Setup.TrackKey = parts[0]
		s.Setup.TrackConfig = ""
		if len(parts) == 2 {
			s.Setup.TrackConfig = parts[1]
		}
	}
	if payload.ClassID > 0 {
		s.Setup.Grid, err = Dba.selectClassEntries(payload.ClassID)
		if err != nil {
			return true, err
		}
	}
	if payload.TimeID > 0 {
		conditions, e := Dba.selectTimeWeather(payload.TimeID)
		err = e
		raw, _ := json.Marshal(conditions)
		if err == nil {
			err = json.Unmarshal(raw, &s.Setup.Conditions)
		}
		if err != nil {
			return true, err
		}
	}
	if len(payload.ClassEntries) > 0 {
		s.Setup.Grid.Entries = nil
		for _, entry := range payload.ClassEntries {
			s.Setup.Grid.Entries = append(s.Setup.Grid.Entries, UserClassEntry{CacheCarKey: &entry.CacheCarKey, SkinKey: &entry.SkinKey, Count: &entry.Count})
		}
	}
	if payload.WeatherKey != "" && len(s.Setup.Conditions.Weathers) > 0 {
		s.Setup.Conditions.Weathers[0].Graphics = &payload.WeatherKey
	}
	if payload.CspTime != "" && len(s.Setup.Conditions.Weathers) > 0 {
		s.Setup.Conditions.Weathers[0].CspTime = &payload.CspTime
	}
	if payload.CspTimeMul > 0 && len(s.Setup.Conditions.Weathers) > 0 {
		s.Setup.Conditions.Weathers[0].CspTimeOfDayMulti = &payload.CspTimeMul
	}
	if err = validateDrivingSetup(s, inst); err != nil {
		return true, err
	}
	if err = validateDrivingInstallation(s, inst); err != nil {
		return true, err
	}
	if inst.Cr.serverEvent.Id == nil {
		return true, errors.New("Switch this repeated session to its manual queue before saving a different setup.")
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		return true, err
	}
	defer tx.Rollback()
	eventID, err := savePrivateSetup(tx, s)
	if err != nil {
		return true, err
	}
	raw, err := json.Marshal(s.Setup)
	if err != nil {
		return true, err
	}
	if _, err = tx.Exec("UPDATE driving_session SET setup=?,event_id=?,revision=revision+1 WHERE id=?", string(raw), eventID, s.ID); err != nil {
		return true, err
	}
	if _, err = tx.Exec("UPDATE server_event SET user_event_id=? WHERE id=?", eventID, *inst.Cr.serverEvent.Id); err != nil {
		return true, err
	}
	return true, tx.Commit()
}
