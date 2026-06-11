package main

import (
	"net/http"
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

func apiTimeUpdate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	var tm UserTime
	if err := c.ShouldBindJSON(&tm); err != nil {
		apiBadRequest(c, "Invalid time payload: "+err.Error())
		return
	}
	tm.Id = &id
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
	EventCategoryId int    `json:"event_category_id"`
	TrackKey        string `json:"track_key"`
	TrackConfig     string `json:"track_config"`
	DifficultyId    int    `json:"difficulty_id"`
	SessionId       int    `json:"session_id"`
	ClassId         int    `json:"class_id"`
	TimeId          int    `json:"time_id"`
	RaceLaps        int    `json:"race_laps"`
	Strategy        int    `json:"strategy"`
}

func (req eventRequest) toUserEvent() (UserEvent, string) {
	if req.TrackKey == "" {
		return UserEvent{}, "track_key is required"
	}
	if req.EventCategoryId <= 0 || req.DifficultyId <= 0 || req.SessionId <= 0 || req.ClassId <= 0 || req.TimeId <= 0 {
		return UserEvent{}, "event_category_id, difficulty_id, session_id, class_id and time_id are required"
	}
	evt := UserEvent{
		EventCategoryId:  &req.EventCategoryId,
		CacheTrackKey:    &req.TrackKey,
		CacheTrackConfig: &req.TrackConfig,
		DifficultyId:     &req.DifficultyId,
		SessionId:        &req.SessionId,
		ClassId:          &req.ClassId,
		TimeId:           &req.TimeId,
		RaceLaps:         &req.RaceLaps,
		Strategy:         &req.Strategy,
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
	if _, err := Dba.insertEvent(evt); err != nil {
		apiDbError(c, err)
		return
	}
	c.PureJSON(http.StatusOK, gin.H{"success": true})
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
