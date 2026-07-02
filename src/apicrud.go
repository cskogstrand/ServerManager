package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// JSON CRUD endpoints (Phase 0 of the SPA migration). All of them reuse the
// existing Dbaccess functions the HTML routes use, so both UIs stay in sync.

type nameRequest struct {
	Name string `json:"name"`
}

// bindName reads a {"name": "..."} create payload.
func bindName(c *gin.Context) (string, bool) {
	var req nameRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		apiBadRequest(c, "A non-empty \"name\" is required")
		return "", false
	}
	return strings.TrimSpace(req.Name), true
}

func listFilled(c *gin.Context) bool {
	return c.Query("filled") == "1"
}

// --- Difficulty ---

func apiDifficultyList(c *gin.Context) {
	list, err := Dba.selectDifficultyList(listFilled(c))
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiDifficultyCreate(c *gin.Context) {
	name, ok := bindName(c)
	if !ok {
		return
	}
	id, err := Dba.insertDifficulty(name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiDifficultyUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var dif UserDifficulty
	if err := c.ShouldBindJSON(&dif); err != nil {
		apiBadRequest(c, "Invalid difficulty payload: "+err.Error())
		return
	}
	dif.Id = &id
	if _, err := Dba.updateDifficulty(dif); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiDifficultyDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteDifficulty(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Drift scoring modes ---

func apiDriftScoringModeList(c *gin.Context) {
	list, err := Dba.selectDriftScoringModeList(listFilled(c))
	if err != nil {
		apiDbError(c, err)
		return
	}
	usage, err := Dba.driftScoringModeUsageCounts()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list, "usage": usage})
}

func apiDriftScoringModeGet(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	mode, err := Dba.selectDriftScoringMode(id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"data": mode})
}

func apiDriftScoringModeCreate(c *gin.Context) {
	name, ok := bindName(c)
	if !ok {
		return
	}
	id, err := Dba.insertDriftScoringMode(name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiDriftScoringModeUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var mode DriftScoringMode
	if err := c.ShouldBindJSON(&mode); err != nil {
		apiBadRequest(c, "Invalid drift scoring mode payload: "+err.Error())
		return
	}
	mode.Id = &id
	if _, err := Dba.updateDriftScoringMode(mode); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiDriftScoringModeDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteDriftScoringMode(id); err != nil {
		apiError(c, http.StatusConflict, "in_use", err.Error())
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Session ---

func apiSessionList(c *gin.Context) {
	list, err := Dba.selectSessionList(listFilled(c))
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiSessionCreate(c *gin.Context) {
	name, ok := bindName(c)
	if !ok {
		return
	}
	id, err := Dba.insertSession(name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiSessionUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var ses UserSession
	if err := c.ShouldBindJSON(&ses); err != nil {
		apiBadRequest(c, "Invalid session payload: "+err.Error())
		return
	}
	ses.Id = &id
	if _, err := Dba.updateSession(ses); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiSessionDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteSession(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Time & weather ---

func apiTimeList(c *gin.Context) {
	list, err := Dba.selectTimeList(listFilled(c))
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiTimeCreate(c *gin.Context) {
	name, ok := bindName(c)
	if !ok {
		return
	}
	id, err := Dba.insertTime(name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// timeUpdateRequest mirrors UserTime/UserTimeWeather with plain JSON numbers.
// The model structs carry ",string" tags the old form UI depends on, which
// reject properly-typed JSON — so the SPA endpoint binds this DTO instead.
type timeWeatherRequest struct {
	Graphics               *string `json:"graphics"`
	BaseTemperatureAmbient *int    `json:"base_temperature_ambient"`
	BaseTemperatureRoad    *int    `json:"base_temperature_road"`
	VariationAmbient       *int    `json:"variation_ambient"`
	VariationRoad          *int    `json:"variation_road"`
	WindBaseSpeedMin       *int    `json:"wind_base_speed_min"`
	WindBaseSpeedMax       *int    `json:"wind_base_speed_max"`
	WindBaseDirection      *int    `json:"wind_base_direction"`
	WindVariationDirection *int    `json:"wind_variation_direction"`
	CspTime                *string `json:"csp_time"`
	CspTimeOfDayMulti      *int    `json:"csp_time_of_day_multi"`
	CspDate                *string `json:"csp_date"`
}

type timeUpdateRequest struct {
	Name           *string              `json:"name"`
	Time           *string              `json:"time"`
	TimeOfDayMulti *int                 `json:"time_of_day_multi"`
	CspEnabled     *int                 `json:"csp_enabled"`
	Weathers       []timeWeatherRequest `json:"weathers"`
}

func apiTimeUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var req timeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid time payload: "+err.Error())
		return
	}

	tm := UserTime{
		Id:             &id,
		Name:           req.Name,
		Time:           req.Time,
		TimeOfDayMulti: req.TimeOfDayMulti,
		CspEnabled:     req.CspEnabled,
	}
	for _, w := range req.Weathers {
		tm.Weathers = append(tm.Weathers, UserTimeWeather{
			Graphics:               w.Graphics,
			BaseTemperatureAmbient: w.BaseTemperatureAmbient,
			BaseTemperatureRoad:    w.BaseTemperatureRoad,
			VariationAmbient:       w.VariationAmbient,
			VariationRoad:          w.VariationRoad,
			WindBaseSpeedMin:       w.WindBaseSpeedMin,
			WindBaseSpeedMax:       w.WindBaseSpeedMax,
			WindBaseDirection:      w.WindBaseDirection,
			WindVariationDirection: w.WindVariationDirection,
			CspTime:                w.CspTime,
			CspTimeOfDayMulti:      w.CspTimeOfDayMulti,
			CspDate:                w.CspDate,
		})
	}

	if _, err := Dba.updateTime(tm); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiTimeDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteTime(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Class ---

func apiClassList(c *gin.Context) {
	list, err := Dba.selectClassList(listFilled(c))
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiClassCreate(c *gin.Context) {
	name, ok := bindName(c)
	if !ok {
		return
	}
	id, err := Dba.insertClass(name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiClassUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var cls UserClass
	if err := c.ShouldBindJSON(&cls); err != nil {
		apiBadRequest(c, "Invalid class payload: "+err.Error())
		return
	}
	cls.Id = &id
	if _, err := Dba.updateClass(cls); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiClassDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteClass(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Event category (incl. nested events, same shape the old UI posts) ---

func apiCategoryList(c *gin.Context) {
	list, err := Dba.selectEventCategoryList(listFilled(c))
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiCategoryGet(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	cat, err := Dba.selectCategoryEvents(id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	if cat.Id == nil {
		apiNotFound(c)
		return
	}
	c.PureJSON(http.StatusOK, cat)
}

func apiCategoryCreate(c *gin.Context) {
	name, ok := bindName(c)
	if !ok {
		return
	}
	id, err := Dba.insertEventCategory(name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiCategoryDuplicate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	src, err := Dba.selectEventCategory(id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	name := "Copy"
	if src.Name != nil {
		name = *src.Name + " (copy)"
	}
	newId, err := Dba.duplicateEventCategory(id, name)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": newId})
}

func apiCategoryUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var cat UserEventCategory
	if err := c.ShouldBindJSON(&cat); err != nil {
		apiBadRequest(c, "Invalid category payload: "+err.Error())
		return
	}
	cat.Id = &id
	if _, err := Dba.updateEventCategory(cat); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// apiCategoryRename changes only the name. The full PUT replaces the nested
// event list (anything not sent gets deleted), which the SPA avoids — it
// manages events through /api/events instead.
func apiCategoryRename(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	name, ok := bindName(c)
	if !ok {
		return
	}
	if _, err := Dba.updateEventCategoryName(id, name); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiCategoryDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteEventCategory(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Events ---

// eventRequest is a clean DTO: plain numbers, separate track key/config.
// (UserEvent's json tags carry ",string" quirks the old dashboard relies on.)
type eventRequest struct {
	EventCategoryId    int    `json:"event_category_id"`
	Name               string `json:"name"`
	TrackKey           string `json:"track_key"`
	TrackConfig        string `json:"track_config"`
	DifficultyId       int    `json:"difficulty_id"`
	SessionId          int    `json:"session_id"`
	ClassId            int    `json:"class_id"`
	TimeId             int    `json:"time_id"`
	DriftScoringModeId *int   `json:"drift_scoring_mode_id"`
	RaceLaps           int    `json:"race_laps"`
	Strategy           int    `json:"strategy"`
}

func (req eventRequest) toUserEvent() (UserEvent, string) {
	if req.TrackKey == "" {
		return UserEvent{}, "track_key is required"
	}
	if req.EventCategoryId <= 0 || req.DifficultyId <= 0 || req.SessionId <= 0 || req.ClassId <= 0 || req.TimeId <= 0 {
		return UserEvent{}, "event_category_id, difficulty_id, session_id, class_id and time_id are required"
	}
	evt := UserEvent{
		EventCategoryId:    &req.EventCategoryId,
		CacheTrackKey:      &req.TrackKey,
		CacheTrackConfig:   &req.TrackConfig,
		DifficultyId:       &req.DifficultyId,
		SessionId:          &req.SessionId,
		ClassId:            &req.ClassId,
		TimeId:             &req.TimeId,
		DriftScoringModeId: req.DriftScoringModeId,
		RaceLaps:           &req.RaceLaps,
		Strategy:           &req.Strategy,
	}
	if evt.DriftScoringModeId != nil {
		if *evt.DriftScoringModeId <= 0 {
			evt.DriftScoringModeId = nil
		} else if _, err := Dba.selectDriftScoringMode(*evt.DriftScoringModeId); err != nil {
			return UserEvent{}, "drift_scoring_mode_id does not exist"
		}
	}
	// Empty name stays NULL so the lobby title falls back to the category name.
	if name := strings.TrimSpace(req.Name); name != "" {
		evt.Name = &name
	}
	return evt, ""
}

func apiEventList(c *gin.Context) {
	list, err := Dba.selectEventList()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": list})
}

func apiEventGet(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	evt, err := Dba.selectEvent(id)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, evt)
}

func apiEventCreate(c *gin.Context) {
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid event payload: "+err.Error())
		return
	}
	evt, problem := req.toUserEvent()
	if problem != "" {
		apiBadRequest(c, problem)
		return
	}
	id, err := Dba.insertEvent(evt)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"success": true, "id": id})
}

func apiEventUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var req eventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid event payload: "+err.Error())
		return
	}
	evt, problem := req.toUserEvent()
	if problem != "" {
		apiBadRequest(c, problem)
		return
	}
	evt.Id = &id
	if _, err := Dba.updateEvent(evt); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

func apiEventDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if _, err := Dba.deleteEvent(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// apiPresetUsage reports how many events reference each preset, keyed by the
// list resource name (classes/sessions/times/difficulties) → {presetId: count}.
// The preset pages use it to show "used by N events" and to warn before editing
// a shared preset. Column names come from a fixed whitelist, never user input.
func apiPresetUsage(c *gin.Context) {
	cols := map[string]string{
		"classes":             "class_id",
		"sessions":            "session_id",
		"times":               "time_id",
		"difficulties":        "difficulty_id",
		"drift-scoring-modes": "drift_scoring_mode_id",
	}
	out := gin.H{}
	for key, col := range cols {
		counts, err := Dba.presetUsageCounts(col)
		if err != nil {
			apiDbError(c, err)
			return
		}
		out[key] = counts
	}
	c.PureJSON(http.StatusOK, out)
}

// --- Config ---

func apiConfigGet(c *gin.Context) {
	cfg, err := Dba.selectConfig()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, cfg)
}

func apiConfigUpdate(c *gin.Context) {
	var cfg UserConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		apiBadRequest(c, "Invalid config payload: "+err.Error())
		return
	}
	if _, err := Dba.updateConfig(cfg); err != nil {
		apiDbError(c, err)
		return
	}
	// Pick up changed auto-capture tunables without a restart.
	Captures.refresh()
	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

// apiConfigContentUpdate covers the content/CSP half of the config row
// (install path + CSP flags), mirroring the old content page split.
func apiConfigContentUpdate(c *gin.Context) {
	var cfg UserConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		apiBadRequest(c, "Invalid config payload: "+err.Error())
		return
	}
	if _, err := Dba.updateContent(cfg); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

// apiServerEngine reports the selected dedicated-server engine and, for
// AssettoServer, whether its binary is already downloaded.
func apiServerEngine(c *gin.Context) {
	engine := engineKunos
	if cfg, err := Dba.selectConfig(); err == nil && cfg.ServerEngine != nil && *cfg.ServerEngine != "" {
		engine = *cfg.ServerEngine
	}
	c.PureJSON(http.StatusOK, gin.H{
		"engine":                  engine,
		"assettoserver_version":   assettoServerVersion,
		"assettoserver_installed": assettoServerInstalled(),
	})
}

// apiAssettoServerInstall downloads + extracts the AssettoServer release so the
// first server start doesn't block on it (and so download errors surface in the
// UI rather than only the logs).
func apiAssettoServerInstall(c *gin.Context) {
	bin, err := ensureAssettoServerInstalled()
	if err != nil {
		apiError(c, http.StatusBadGateway, "download_failed", err.Error())
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"installed": true, "path": bin, "version": assettoServerVersion})
}

// --- Queue (SPA) ---

// eventDurationMinutes estimates how long a queued event occupies the server:
// the sum of its enabled timed sessions. A lap-based race has no fixed minute
// count, so it contributes 0 (ETA is best-effort, hence "where possible").
func eventDurationMinutes(ue UserEvent) int {
	mins := 0
	add := func(enabled, t *int) {
		if enabled != nil && *enabled == 1 && t != nil {
			mins += *t
		}
	}
	add(ue.BookingEnabled, ue.BookingTime)
	add(ue.PracticeEnabled, ue.PracticeTime)
	add(ue.QualifyEnabled, ue.QualifyTime)
	if ue.RaceEnabled != nil && *ue.RaceEnabled == 1 {
		laps := 0
		if ue.RaceLaps != nil {
			laps = *ue.RaceLaps
		}
		if laps == 0 && ue.RaceTime != nil {
			mins += *ue.RaceTime
		}
	}
	return mins
}

// apiQueueList returns the queue with display names; ?instance=N filters.
func apiQueueList(c *gin.Context) {
	instanceId, _ := strconv.Atoi(c.Query("instance"))
	events, err := Dba.selectServerEvents(false, instanceId)
	if err != nil {
		apiDbError(c, err)
		return
	}

	items := make([]gin.H, 0, len(events))
	for _, se := range events {
		items = append(items, gin.H{
			"id":           se.Id,
			"event_id":     se.UserEvent.Id,
			"instance_id":  se.InstanceId,
			"name":         se.UserEvent.Name,
			"category":     se.UserEvent.CategoryName,
			"track":        se.UserEvent.TrackName,
			"track_key":    se.UserEvent.CacheTrackKey,
			"track_config": se.UserEvent.CacheTrackConfig,
			"difficulty":   se.UserEvent.DifficultyName,
			"session":      se.UserEvent.SessionName,
			"class":        se.UserEvent.ClassName,
			"time":         se.UserEvent.TimeName,
			"duration_min": eventDurationMinutes(se.UserEvent),
			"started_at":   se.StartedAt,
			"finished":     se.Finished,
		})
	}
	c.PureJSON(http.StatusOK, gin.H{"items": items})
}

func apiQueueDelete(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	if queueRowRepeatLocked(id) {
		apiError(c, http.StatusConflict, "repeat_locked", "This instance is in repeat mode. Switch it back to manual queue first.")
		return
	}
	if _, err := Dba.deleteServerEvent(id); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"id": id})
}

// --- Content caches (SPA pickers & library) ---

func apiCarsList(c *gin.Context) {
	cars, err := Dba.selectCacheCars()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": cars})
}

func apiTracksList(c *gin.Context) {
	tracks, err := Dba.selectCacheTracks()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": tracks})
}

func apiWeathersList(c *gin.Context) {
	weathers, err := Dba.selectCacheWeathers()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": weathers})
}

// --- Auth & meta (SPA) ---

type loginRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// apiLogin is the JSON twin of routeLogin; registered outside the auth group.
func apiLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid login payload")
		return
	}

	user, err := Dba.selectUser(req.Name)
	if err != nil || user.Name == nil || user.Password == nil {
		apiError(c, http.StatusUnauthorized, "bad_credentials", "Wrong username or password")
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(req.Password)) != nil {
		apiError(c, http.StatusUnauthorized, "bad_credentials", "Wrong username or password")
		return
	}

	if err := issueAuthCookies(c, *user.Name, "admin"); err != nil {
		apiError(c, http.StatusInternalServerError, "token_error", err.Error())
		return
	}

	role := roleAdmin
	if user.Role != nil && *user.Role != "" {
		role = *user.Role
	}
	c.PureJSON(http.StatusOK, gin.H{"success": true, "name": *user.Name, "role": role})
}

func apiLogout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.SetCookie(csrfCookieName, "", -1, "/", "", false, false)
	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

func apiAbout(c *gin.Context) {
	c.PureJSON(http.StatusOK, gin.H{
		"version":       Version,
		"config_folder": ConfigFolder,
		"temp_folder":   TempFolder,
	})
}

// --- Current user ---

func apiUserGet(c *gin.Context) {
	username, exists := c.Get("user")
	if !exists {
		apiError(c, http.StatusUnauthorized, "unauthorized", "Not logged in")
		return
	}
	usr, err := Dba.selectUser(username.(string))
	if err != nil {
		apiDbError(c, err)
		return
	}
	usr.Password = nil
	c.PureJSON(http.StatusOK, usr)
}

func apiUserUpdate(c *gin.Context) {
	username, exists := c.Get("user")
	if !exists {
		apiError(c, http.StatusUnauthorized, "unauthorized", "Not logged in")
		return
	}

	current, err := Dba.selectUser(username.(string))
	if err != nil {
		apiDbError(c, err)
		return
	}

	var req Users
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid user payload: "+err.Error())
		return
	}

	// Username changes are not supported; password only changes when a new
	// one is provided, and it is stored bcrypt-hashed.
	req.Name = current.Name
	if req.Password != nil && *req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			apiError(c, http.StatusInternalServerError, "hash_error", err.Error())
			return
		}
		hashStr := string(hash)
		req.Password = &hashStr
	} else {
		req.Password = current.Password
	}

	if _, err := Dba.updateUser(req); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"success": true})
}

// --- User management (admin only; enforced by RoleMiddleware) ---

func validRole(role string) bool {
	return role == roleAdmin || role == roleSteward || role == roleViewer
}

func apiUsersList(c *gin.Context) {
	users, err := Dba.selectUsers()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"items": users})
}

func apiUserCreate(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiBadRequest(c, "Invalid user payload")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Password == "" {
		apiBadRequest(c, "Name and password are required.")
		return
	}
	if !validRole(req.Role) {
		apiBadRequest(c, "Role must be admin, steward or viewer.")
		return
	}
	if existing, err := Dba.selectUser(req.Name); err == nil && existing.Name != nil {
		apiError(c, http.StatusConflict, "exists", "A user with that name already exists.")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "hash_error", err.Error())
		return
	}
	if _, err := Dba.insertUser(req.Name, string(hash), req.Role); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"name": req.Name})
}

func apiUserSetRole(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Role string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !validRole(req.Role) {
		apiBadRequest(c, "Role must be admin, steward or viewer.")
		return
	}
	target, err := Dba.selectUser(name)
	if err != nil || target.Name == nil {
		apiNotFound(c)
		return
	}
	// Don't demote the last admin.
	if target.Role != nil && *target.Role == roleAdmin && req.Role != roleAdmin {
		admins, _ := Dba.countAdmins()
		if admins <= 1 {
			apiError(c, http.StatusConflict, "last_admin", "Cannot demote the last admin.")
			return
		}
	}
	if _, err := Dba.updateUserRole(name, req.Role); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"name": name, "role": req.Role})
}

func apiUserResetPassword(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		apiBadRequest(c, "A new password is required.")
		return
	}
	if target, err := Dba.selectUser(name); err != nil || target.Name == nil {
		apiNotFound(c)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "hash_error", err.Error())
		return
	}
	if _, err := Dba.updateUserPassword(name, string(hash)); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"name": name})
}

func apiUserDelete(c *gin.Context) {
	name := c.Param("name")
	if self, ok := c.Get("user"); ok && self.(string) == name {
		apiError(c, http.StatusConflict, "self", "You cannot delete your own account.")
		return
	}
	target, err := Dba.selectUser(name)
	if err != nil || target.Name == nil {
		apiNotFound(c)
		return
	}
	if target.Role != nil && *target.Role == roleAdmin {
		admins, _ := Dba.countAdmins()
		if admins <= 1 {
			apiError(c, http.StatusConflict, "last_admin", "Cannot delete the last admin.")
			return
		}
	}
	if _, err := Dba.deleteUser(name); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"name": name})
}
