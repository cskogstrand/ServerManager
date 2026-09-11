package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func pitlaneTestDB(t *testing.T) Dbaccess {
	t.Helper()
	old := Dba
	dba := open(filepath.Join(t.TempDir(), "fixture.db"))
	dba.applySchema("/schema.sql")
	Dba = dba
	t.Cleanup(func() { dba.db.Close(); Dba = old })
	return dba
}
func pitlaneRequest(t *testing.T, method, path string, body any, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("role", roleAdmin) })
	route := path
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if _, err := strconv.Atoi(part); err == nil {
			parts[i] = ":id"
			break
		}
	}
	route = strings.Join(parts, "/")
	if strings.HasPrefix(path, "/content/metadata/") {
		route = "/content/metadata/:kind/:key"
	}

	r.Handle(method, route, handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}
func seedPitlaneContent(t *testing.T, dba Dbaccess) {
	t.Helper()
	for _, q := range []string{
		`UPDATE user_config SET max_clients=12`,
		`INSERT INTO cache_track(key,config,name,pitboxes,tags) VALUES('fixture_track','','Fixture circuit',12,'[]')`,
		`INSERT INTO cache_car(key,name,skins,tags,specs,torque,power) VALUES('fixture_car','Fixture car','[{"key":"red","name":"Red"}]','[]','{}','[]','[]')`,
		`INSERT INTO cache_weather(key,name) VALUES('clear','Clear')`,
	} {
		if _, err := dba.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}
func TestPitlaneInventoryUpgradeAndRevision(t *testing.T) {
	dba := pitlaneTestDB(t)
	r := Rig{Name: "Shared seat", Display: "triple", Gear: []RigGear{{ID: "wheel", Type: "Wheel", Name: "Club wheel"}}}
	w := pitlaneRequest(t, "POST", "/rigs", r, apiRigSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	dba.applySchema("/schema.sql")
	rigs, err := dba.rigs()
	if err != nil || len(rigs) != 1 || rigs[0].Gear[0].ID != "wheel" {
		t.Fatal(rigs, err)
	}
	stale := rigs[0]
	r = stale
	r.Location = "Upstairs"
	w = pitlaneRequest(t, "PUT", "/rigs/1", r, apiRigSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = pitlaneRequest(t, "PUT", "/rigs/1", stale, apiRigSave)
	if w.Code != 409 {
		t.Fatal("stale edit must conflict", w.Code, w.Body.String())
	}
	w = pitlaneRequest(t, "POST", "/rigs", Rig{Name: "SHARED SEAT", Display: "vr"}, apiRigSave)
	if w.Code != 409 {
		t.Fatal("duplicate name must conflict", w.Code)
	}
	r.Display = "regenerated"
	w = pitlaneRequest(t, "POST", "/rigs", r, apiRigSave)
	if w.Code != 400 {
		t.Fatal("unknown display accepted")
	}
}
func TestPitlaneSourceWithoutDriver(t *testing.T) {
	dba := pitlaneTestDB(t)
	status := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer status.Close()
	s := CameraSource{Name: "Idle seat", Enabled: true, PlayerURL: status.URL + "/whep", StatusURL: status.URL, CaptureURL: "rtsp://fixture.invalid/camera"}
	w := pitlaneRequest(t, "POST", "/sources", s, apiCameraSourceSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	sources, err := dba.cameraSources()
	if err != nil || len(sources) != 1 {
		t.Fatal(sources, err)
	}
	w = pitlaneRequest(t, "GET", "/sources/status", nil, apiCameraSourceHealth)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"status":"live"`)) {
		t.Fatal(w.Code, w.Body.String())
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("role", roleViewer) })
	r.GET("/sources", apiCameraSources)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/sources", nil))
	if bytes.Contains(w.Body.Bytes(), []byte("rtsp://")) || bytes.Contains(w.Body.Bytes(), []byte("status_url")) {
		t.Fatal("source credentials leaked", w.Body.String())
	}
}
func TestPitlaneWholeDraftAndPreview(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	old := Instances
	Instances = NewInstanceManager()
	i := 1
	Instances.instances[1] = &Instance{Conf: ServerInstance{Id: &i, Enabled: &i, Name: textPtr("Fixture server")}}
	t.Cleanup(func() { Instances = old })
	w := pitlaneRequest(t, "GET", "/defaults", nil, apiDrivingDefaults)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var s DrivingSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	s.Experience = "practice"
	w = pitlaneRequest(t, "POST", "/preview", s, apiDrivingPreview)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var presets int
	dba.db.QueryRow("SELECT count(*) FROM user_class").Scan(&presets)
	if presets != 0 {
		t.Fatal("preview wrote a preset")
	}
	w = pitlaneRequest(t, "POST", "/driving-sessions", s, apiDrivingSessionSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	firstEvent := *s.EventID
	s.Setup.Grid.Entries[0].Count = intPtr(3)
	w = pitlaneRequest(t, "PUT", "/driving-sessions/1", s, apiDrivingSessionSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var oldCount int
	err := dba.db.QueryRow("SELECT car_count FROM user_class_entry WHERE user_class_id=(SELECT class_id FROM user_event WHERE id=?)", firstEvent).Scan(&oldCount)
	if err != nil || oldCount != 8 {
		t.Fatal("prior configuration was changed", oldCount, err)
	}
	w = pitlaneRequest(t, "PUT", "/driving-sessions/1", s, apiDrivingSessionSave)
	if w.Code != 409 {
		t.Fatal("stale revision accepted", w.Code, w.Body.String())
	}
	s.Setup.Grid.Entries[0].Count = intPtr(99)
	w = pitlaneRequest(t, "POST", "/driving-sessions", s, apiDrivingSessionSave)
	if w.Code != 400 {
		t.Fatal("oversize grid accepted", w.Code, w.Body.String())
	}
	dba.applySchema("/schema.sql")
	var count int
	dba.db.QueryRow("SELECT count(*) FROM driving_session").Scan(&count)
	if count != 1 {
		t.Fatal("upgrade lost session")
	}
}
func TestPitlaneLegacyQuery(t *testing.T) {
	r := gin.New()
	r.GET("/app/*path", routeLegacyApp)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/app/sessions?q=drift%20night&tag=tandem&instance=2", nil))
	if got := w.Header().Get("Location"); got != "/sessions?q=drift%20night&tag=tandem&instance=2" {
		t.Fatal(got)
	}
}

func pitlaneFixtureInstance(t *testing.T, dba Dbaccess, binary string) *Instance {
	t.Helper()
	oldInstances, oldConfig, oldTemp, oldCaptures := Instances, ConfigFolder, TempFolder, Captures
	ConfigFolder = t.TempDir()
	TempFolder = t.TempDir()
	Captures = nil
	Instances = NewInstanceManager()
	confs, err := dba.selectServerInstances()
	if err != nil || len(confs) == 0 {
		t.Fatal(err)
	}
	inst := &Instance{Conf: confs[0], Udp: &UdpPlugin{}, drivers: map[int]*DriverState{}, positions: map[int]*CarPositionState{}, driftScorers: map[int]*driftScorer{}}
	Instances.instances[inst.Id()] = inst
	archive, err := os.Create(filepath.Join(ConfigFolder, "smcontent.zip"))
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for name, data := range map[string]string{"acServer": binary, "tracks/fixture_track/models.ini": "[MODEL_0]\nFILE=fixture.kn5", "cars/fixture_car/data.acd": "fixture"} {
		file, e := writer.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = file.Write([]byte(data)); e != nil {
			t.Fatal(e)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	archive.Close()
	if _, err = dba.db.Exec("UPDATE user_config SET cfg_filled=1,mod_filled=1,server_engine='kunos'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		inst.stop()
		Instances, ConfigFolder, TempFolder, Captures = oldInstances, oldConfig, oldTemp, oldCaptures
	})
	return inst
}
func pitlaneDraft(t *testing.T) DrivingSession {
	t.Helper()
	w := pitlaneRequest(t, "GET", "/defaults", nil, apiDrivingDefaults)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var s DrivingSession
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	s.Experience = "practice"
	w = pitlaneRequest(t, "POST", "/driving-sessions", s, apiDrivingSessionSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestPitlaneExactLaunchRetryAndCancellation(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	inst := pitlaneFixtureInstance(t, dba, "#!/bin/sh\nexec sleep 60\n")
	first, second := pitlaneDraft(t), pitlaneDraft(t)
	req := LaunchRequest{Key: "durable-operation-0001", Revision: first.Revision, InstanceID: inst.Id(), Mode: "now", TimeZone: "Europe/Oslo"}
	path := fmt.Sprintf("/driving-sessions/%d/start", first.ID)
	w := pitlaneRequest(t, "POST", path, req, apiDrivingSessionLaunch)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !inst.isRunning() || inst.Cr.serverEvent.UserEvent.Id == nil || *inst.Cr.serverEvent.UserEvent.Id != *first.EventID {
		t.Fatal("Started another queue item")
	}
	var initial map[string]any
	json.Unmarshal(w.Body.Bytes(), &initial)
	w = pitlaneRequest(t, "POST", path, req, apiDrivingSessionLaunch)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var retry map[string]any
	json.Unmarshal(w.Body.Bytes(), &retry)
	if retry["execution_id"] != initial["execution_id"] {
		t.Fatal("Retry created another execution")
	}
	req.Mode = "after"
	w = pitlaneRequest(t, "POST", path, req, apiDrivingSessionLaunch)
	if w.Code != 409 {
		t.Fatal("Mismatched idempotency body accepted", w.Code)
	}
	req.Key = "durable-operation-0002"
	req.Revision = second.Revision
	w = pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/start", second.ID), req, apiDrivingSessionLaunch)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/finish", second.ID), nil, apiDrivingFinish)
	if w.Code != 200 || !inst.isRunning() {
		t.Fatal("Cancelling queued session affected running session", w.Code, w.Body.String())
	}
	w = pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/finish", first.ID), nil, apiDrivingFinish)
	if w.Code != 200 || inst.isRunning() {
		t.Fatal("Finish did not stop exact execution", w.Code, w.Body.String())
	}
	var count int
	dba.db.QueryRow("SELECT count(*) FROM driving_execution WHERE session_id=?", first.ID).Scan(&count)
	if count != 1 {
		t.Fatal("Duplicate execution", count)
	}
}
func TestPitlaneQueueConflictAndFailedStartRecovery(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	inst := pitlaneFixtureInstance(t, dba, "not an executable")
	earlier, draft := pitlaneDraft(t), pitlaneDraft(t)
	if _, err := dba.db.Exec("INSERT INTO server_event(user_event_id,instance_id,orderby) VALUES(?,?,1)", earlier.EventID, inst.Id()); err != nil {
		t.Fatal(err)
	}
	req := LaunchRequest{Key: "failed-start-operation", Revision: draft.Revision, InstanceID: inst.Id(), Mode: "now"}
	path := fmt.Sprintf("/driving-sessions/%d/start", draft.ID)
	w := pitlaneRequest(t, "POST", path, req, apiDrivingSessionLaunch)
	if w.Code != 409 {
		t.Fatal("Queue conflict accepted", w.Code, w.Body.String())
	}
	var count int
	dba.db.QueryRow("SELECT count(*) FROM driving_execution").Scan(&count)
	if count != 0 {
		t.Fatal("Conflicting request persisted a launch")
	}
	if _, err := dba.db.Exec("UPDATE server_event SET finished=1"); err != nil {
		t.Fatal(err)
	}
	w = pitlaneRequest(t, "POST", path, req, apiDrivingSessionLaunch)
	if w.Code != 409 {
		t.Fatal("Broken executable reported success", w.Code, w.Body.String())
	}
	var state string
	dba.db.QueryRow("SELECT state FROM driving_execution WHERE session_id=?", draft.ID).Scan(&state)
	if state != "failed" {
		t.Fatal(state)
	}
	dba.db.QueryRow("SELECT count(*) FROM server_event WHERE finished=0").Scan(&count)
	if count != 0 {
		t.Fatal("Failed execution left a runnable queue entry")
	}
	w = pitlaneRequest(t, "POST", path, req, apiDrivingSessionLaunch)
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte(`"state":"failed"`)) {
		t.Fatal("Failed outcome was not replayed", w.Code, w.Body.String())
	}
	if err := reconcileDrivingExecutions(); err != nil {
		t.Fatal(err)
	}
}
func TestPitlaneMetadataUndoAndCleanup(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	m := ContentMetadata{Revision: 0, DisplayName: "Club car", Tags: []string{"club"}, Archived: true}
	path := "/content/metadata/car/fixture_car"
	w := pitlaneRequest(t, "PUT", path, m, apiContentMetadataSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &m)
	dba.applySchema("/schema.sql")
	if !contentArchived("car", "fixture_car", "") {
		t.Fatal("Archive did not survive upgrade")
	}
	stale := m
	m.Notes = "newer"
	w = pitlaneRequest(t, "PUT", path, m, apiContentMetadataSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = pitlaneRequest(t, "POST", "/recovery/changes/1/undo", nil, apiMetadataUndo)
	if w.Code != 409 {
		t.Fatal("Undo overwrote later changes", w.Code, w.Body.String())
	}
	w = pitlaneRequest(t, "PUT", path, stale, apiContentMetadataSave)
	if w.Code != 409 {
		t.Fatal("Stale metadata edit accepted")
	}
	w = pitlaneRequest(t, "POST", "/recovery/changes/2/undo", nil, apiMetadataUndo)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	oldTemp := TempFolder
	TempFolder = t.TempDir()
	t.Cleanup(func() { TempFolder = oldTemp })
	target := filepath.Join(TempFolder, "sm-upload-abandoned.zip")
	os.WriteFile(target, []byte("old upload"), 0600)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(target, past, past)
	os.WriteFile(filepath.Join(TempFolder, "smdata.db.restore"), []byte("keep"), 0600)
	targets, token, err := cleanupTargets()
	if err != nil || len(targets) != 1 {
		t.Fatal(targets, err)
	}
	os.WriteFile(target, []byte("changed upload"), 0600)
	w = pitlaneRequest(t, "POST", "/recovery/cleanup", map[string]string{"token": token}, apiCleanupApply)
	if w.Code != 409 {
		t.Fatal("Stale cleanup preview applied", w.Code)
	}
	if _, err = os.Stat(target); err != nil {
		t.Fatal("Changed file deleted")
	}
}
func TestPitlaneHandoverPreservesPriorOwnership(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	inst := pitlaneFixtureInstance(t, dba, "fixture")
	for _, q := range []string{"INSERT INTO guest_driver(name,created_at) VALUES('Previous',1),('Next',1)", "INSERT INTO driver(guid,name,first_seen,last_seen) VALUES('shared','Account',1,1)"} {
		if _, err := dba.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	inst.drivers[0] = &DriverState{Connected: true, Guid: "shared", Name: "Previous", accountName: "Account", GuestDriverId: 1, Laps: 2, BestLapMs: 90000, joinedAt: 1, sessTrack: "fixture_track", Car: "fixture_car", executionID: 12}
	inst.positions[0] = &CarPositionState{VelocityX: 5}
	if err := inst.handoverDriver(0, 2); err == nil {
		t.Fatal("Moving handover accepted")
	}
	inst.positions[0].VelocityX = 0
	if err := inst.handoverDriver(0, 2); err != nil {
		t.Fatal(err)
	}
	var guest, laps int
	dba.db.QueryRow("SELECT guest_driver_id,laps FROM driver_session").Scan(&guest, &laps)
	if guest != 1 || laps != 2 {
		t.Fatal("Old result ownership changed", guest, laps)
	}
	if inst.drivers[0].GuestDriverId != 2 || inst.drivers[0].Laps != 0 {
		t.Fatal("Future segment not reset")
	}
	if err := dba.insertDriverMedia("source:7", "clip", "previous.mp4", "Pinned recording", 1, 0, 0, 0, 0, 0, 1, 12, "shared"); err != nil {
		t.Fatal(err)
	}
	var source, guid string
	dba.db.QueryRow("SELECT guest_driver_id,source_id,driver_guid FROM driver_media").Scan(&guest, &source, &guid)
	if guest != 1 || source != "source:7" || guid != "shared" {
		t.Fatal("In-flight recording reassigned", guest, source, guid)
	}
}
func TestPitlaneOverridesAndBodyIdentity(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	s := pitlaneDraft(t)
	s.Setup.CustomINI = "[SERVER]\nUDP_PORT=1234"
	if validateDrivingSetup(s, nil) == nil {
		t.Fatal("Port override accepted")
	}
	s.Setup.CustomINI = "[PRACTICE]\nTIME=35"
	effective, err := applyDrivingOverrides(s.Setup)
	if err != nil || *effective.Phases.PracticeTime != 35 {
		t.Fatal(effective, err)
	}
	w := pitlaneRequest(t, "POST", "/driving-sessions", s, apiDrivingSessionSave)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var copy DrivingSession
	json.Unmarshal(w.Body.Bytes(), &copy)
	if copy.ID == s.ID {
		t.Fatal("Body ID changed existing session")
	}
}

func TestPitlaneConcurrentOperatorsAndLiveVersion(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	inst := pitlaneFixtureInstance(t, dba, "#!/bin/sh\nexec sleep 60\n")
	s := pitlaneDraft(t)
	path := fmt.Sprintf("/driving-sessions/%d/start", s.ID)
	results := make(chan *httptest.ResponseRecorder, 6)
	for i := 0; i < 6; i++ {
		go func() {
			results <- pitlaneRequest(t, "POST", path, LaunchRequest{Key: "concurrent-same-operation", Revision: s.Revision, InstanceID: inst.Id(), Mode: "now"}, apiDrivingSessionLaunch)
		}()
	}
	var execution float64
	for i := 0; i < 6; i++ {
		w := <-results
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		if i == 0 {
			execution = result["execution_id"].(float64)
		} else if result["execution_id"] != execution {
			t.Fatal("Concurrent retry produced another execution")
		}
	}
	handled, err := applyOwnedLiveSetup(inst, DashboardEventUpdate{EventID: *s.EventID, ClassEntries: []DashboardClassEntryUpdate{{CacheCarKey: "fixture_car", SkinKey: "red", Count: 2}}})
	if err != nil || !handled {
		t.Fatal(handled, err)
	}
	var snapshot string
	dba.db.QueryRow("SELECT setup_snapshot FROM driving_execution WHERE id=?", execution).Scan(&snapshot)
	var oldSetup DrivingSetup
	json.Unmarshal([]byte(snapshot), &oldSetup)
	if *oldSetup.Grid.Entries[0].Count == 2 {
		t.Fatal("Live edit rewrote prior execution")
	}
	if err = restartCurrentServerEvent(inst); err != nil {
		t.Fatal(err)
	}
	var count int
	dba.db.QueryRow("SELECT count(*) FROM driving_execution WHERE session_id=?", s.ID).Scan(&count)
	if count != 2 {
		t.Fatal("Restart did not create a distinct execution", count)
	}
	if !strings.Contains(inst.Cr.entryListResult, "[CAR_1]") || strings.Contains(inst.Cr.entryListResult, "[CAR_2]") {
		t.Fatal("Restart did not use private edited grid")
	}
	w := pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/finish", s.ID), nil, apiDrivingFinish)
	if w.Code != 200 || inst.isRunning() {
		t.Fatal("Versioned session finish lost execution ownership", w.Code, w.Body.String())
	}
}

func TestPitlanePlannedOverrunAndTimezone(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	inst := pitlaneFixtureInstance(t, dba, "#!/bin/sh\nexec sleep 60\n")
	first, second := pitlaneDraft(t), pitlaneDraft(t)
	future := time.Now().Add(time.Hour).UnixMilli()
	for i, s := range []DrivingSession{first, second} {
		req := LaunchRequest{Key: fmt.Sprintf("planned-operation-%d", i), Revision: s.Revision, InstanceID: inst.Id(), Mode: "later", ScheduledAt: &future, TimeZone: "Europe/Oslo"}
		w := pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/start", s.ID), req, apiDrivingSessionLaunch)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if _, err := dba.db.Exec("UPDATE driving_session SET scheduled_at=?", time.Now().Add(-time.Minute).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	checkDrivingSchedules()
	if !inst.isRunning() {
		t.Fatal("Due planned session did not start")
	}
	var secondState string
	dba.db.QueryRow("SELECT lifecycle FROM driving_session WHERE id=?", second.ID).Scan(&secondState)
	if secondState != "planned" {
		t.Fatal("Overrun displaced current session", secondState)
	}
	if w := pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/finish", first.ID), nil, apiDrivingFinish); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	checkDrivingSchedules()
	if !inst.isRunning() || *inst.Cr.serverEvent.UserEvent.Id != *second.EventID {
		t.Fatal("Next due session did not start when free")
	}
	third := pitlaneDraft(t)
	req := LaunchRequest{Key: "invalid-timezone-request", Revision: third.Revision, InstanceID: inst.Id(), Mode: "later", ScheduledAt: &future, TimeZone: "Europe/NotAZone"}
	if w := pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/start", third.ID), req, apiDrivingSessionLaunch); w.Code != 400 {
		t.Fatal("Invalid timezone accepted", w.Code)
	}
}

func TestPitlaneSourceMediaRolesAndScope(t *testing.T) {
	dba := pitlaneTestDB(t)
	if _, err := dba.db.Exec("INSERT INTO guest_driver(name,created_at) VALUES('Guest',1)"); err != nil {
		t.Fatal(err)
	}
	if err := dba.insertDriverMedia("source:7", "clip", "moment.mp4", "Moment", 1, 0, 0, 0, 0, 0, 0, 0, "shared"); err != nil {
		t.Fatal(err)
	}
	request := func(role, method, path, body string) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("role", role) }, RoleMiddleware)
		r.POST("/api/sources/:key/media/:file/assign", apiSourceMediaMutate)
		r.DELETE("/api/sources/:key/media/:file", apiSourceMediaMutate)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	if w := request(roleViewer, "DELETE", "/api/sources/source:7/media/moment.mp4", ""); w.Code != 403 {
		t.Fatal("Viewer deleted media", w.Code)
	}
	if w := request(roleSteward, "POST", "/api/sources/source:8/media/moment.mp4/assign", `{"guest_driver_id":1}`); w.Code != 404 {
		t.Fatal("Cross-source attribution accepted", w.Code)
	}
	if w := request(roleSteward, "POST", "/api/sources/source:7/media/moment.mp4/assign", `{"guest_driver_id":1}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request(roleSteward, "DELETE", "/api/sources/source:7/media/moment.mp4", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestPitlaneArchiveMissingPlatformFileAndReplacement(t *testing.T) {
	old := ConfigFolder
	ConfigFolder = t.TempDir()
	t.Cleanup(func() { ConfigFolder = old })
	source := filepath.Join(t.TempDir(), "acServer")
	if err := os.WriteFile(source, []byte("fixture executable"), 0600); err != nil {
		t.Fatal(err)
	}
	z := ZipFile{}
	defer z.Close()
	if err := z.UpdateZipfile(map[string]string{source: "acServer", source + ".exe": "acServer.exe"}); err != nil {
		t.Fatal(err)
	}
	if z.FindZipFile("acServer") == nil || z.FindZipFile("acServer.exe") != nil {
		t.Fatal("Platform archive membership incorrect")
	}
	if err := z.UpdateZipfile(map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if z.FindZipFile("acServer") != nil {
		t.Fatal("Recache retained removed content")
	}
}

func TestPitlaneDrivingValueBounds(t *testing.T) {
	for _, setup := range []DrivingSetup{
		{Difficulty: UserDifficulty{AbsAllowed: intPtr(3)}},
		{Conditions: timeUpdateRequest{Weathers: []timeWeatherRequest{{WindBaseSpeedMin: intPtr(20), WindBaseSpeedMax: intPtr(5)}}}},
		{Conditions: timeUpdateRequest{CspEnabled: intPtr(1), Weathers: []timeWeatherRequest{{CspTime: textPtr("29:00")}}}},
		{Phases: UserSession{PracticeEnabled: intPtr(2)}},
	} {
		if err := validateDrivingValues(setup); err == nil {
			t.Fatal("Invalid hidden driving value accepted", setup)
		}
	}
}

func TestPitlaneConsistentBackupAndStagedRestore(t *testing.T) {
	dba := pitlaneTestDB(t)
	oldTemp, oldConfig, oldInstances := TempFolder, ConfigFolder, Instances
	TempFolder = t.TempDir()
	ConfigFolder = t.TempDir()
	Instances = &InstanceManager{instances: map[int]*Instance{}}
	t.Cleanup(func() { TempFolder, ConfigFolder, Instances = oldTemp, oldConfig, oldInstances })
	dba.db.Exec("UPDATE user_config SET name='Before backup'")
	w := pitlaneRequest(t, "GET", "/backup", nil, apiServerSmdata)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	snapshot := filepath.Join(t.TempDir(), "backup.db")
	os.WriteFile(snapshot, w.Body.Bytes(), 0600)
	if !validRestoreDatabase(snapshot) {
		t.Fatal("Backup is not a complete, consistent ServerManager database")
	}
	// Reject a corrupt upload without destroying a previously valid staged backup.
	staged := filepath.Join(ConfigFolder, "smdata.db.restore")
	os.WriteFile(staged, w.Body.Bytes(), 0600)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("database", "corrupt.db")
	part.Write([]byte(sqliteMagic + "corrupt"))
	writer.Close()
	r := gin.New()
	r.POST("/restore", apiMaintenanceRestore)
	req := httptest.NewRequest("POST", "/restore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 400 || !validRestoreDatabase(staged) {
		t.Fatal("Invalid upload replaced staged backup", rec.Code, rec.Body.String())
	}
	target := filepath.Join(ConfigFolder, "smdata.db")
	os.WriteFile(target, []byte("previous database fixture"), 0600)
	applyStagedRestore(target)
	if !validRestoreDatabase(target) {
		t.Fatal("Staged restore failed")
	}
	previous, _ := os.ReadFile(target + ".prev")
	if string(previous) != "previous database fixture" {
		t.Fatal("Rollback database lost")
	}
}

func TestPitlaneTwoRunningInstancesStayIsolated(t *testing.T) {
	dba := pitlaneTestDB(t)
	seedPitlaneContent(t, dba)
	first := pitlaneFixtureInstance(t, dba, "#!/bin/sh\nexec sleep 60\n")
	conf := first.Conf
	conf.Id = nil
	conf.Name = textPtr("Second fixture")
	conf.UdpPort = intPtr(41001)
	conf.TcpPort = intPtr(41001)
	conf.HttpPort = intPtr(41002)
	conf.PluginPort = intPtr(41003)
	conf.PluginListenPort = intPtr(41004)
	id, err := dba.insertServerInstance(conf)
	if err != nil {
		t.Fatal(err)
	}
	conf.Id = intPtr(int(id))
	second := &Instance{Conf: conf, Udp: &UdpPlugin{}, drivers: map[int]*DriverState{}, positions: map[int]*CarPositionState{}, driftScorers: map[int]*driftScorer{}}
	Instances.instances[second.Id()] = second
	t.Cleanup(func() { second.stop() })
	a, b := pitlaneDraft(t), pitlaneDraft(t)
	for index, pair := range []struct {
		s DrivingSession
		i *Instance
	}{{a, first}, {b, second}} {
		req := LaunchRequest{Key: fmt.Sprintf("isolated-execution-%d", index), Revision: pair.s.Revision, InstanceID: pair.i.Id(), Mode: "now"}
		w := pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/start", pair.s.ID), req, apiDrivingSessionLaunch)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if !first.isRunning() || !second.isRunning() || first.Dir() == second.Dir() {
		t.Fatal("Independent servers did not start separately")
	}
	if w := pitlaneRequest(t, "POST", fmt.Sprintf("/driving-sessions/%d/finish", a.ID), nil, apiDrivingFinish); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if first.isRunning() || !second.isRunning() || *second.Cr.serverEvent.UserEvent.Id != *b.EventID {
		t.Fatal("Finishing one session changed the other instance")
	}
}

// Set PITLANE_LEGACY_SCHEMA to `git show <pre-cutover>:src/embed/schema.sql`
// when checking compatibility against a particular release.
func TestPitlaneLegacySchemaUpgradeAndReadCompatibility(t *testing.T) {
	path := os.Getenv("PITLANE_LEGACY_SCHEMA")
	if path == "" {
		t.Skip("Set PITLANE_LEGACY_SCHEMA for release compatibility verification")
	}
	legacy, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := Dba
	dba := open(filepath.Join(t.TempDir(), "legacy.db"))
	Dba = dba
	t.Cleanup(func() { dba.db.Close(); Dba = old })
	if _, err = dba.db.Exec(string(legacy)); err != nil {
		t.Fatal(err)
	}
	if _, err = dba.db.Exec("UPDATE user_config SET name='Preserved legacy club'; INSERT INTO guest_driver(name,created_at) VALUES('Preserved guest',1)"); err != nil {
		t.Fatal(err)
	}
	dba.applySchema("/schema.sql")
	seedPitlaneContent(t, dba)
	session := pitlaneDraft(t)
	dba.applySchema("/schema.sql")
	if _, err = dba.db.Exec(string(legacy)); err != nil {
		t.Fatal("Previous schema cannot open upgraded data", err)
	}
	var name string
	var count int
	if err = dba.db.QueryRow("SELECT name FROM user_config WHERE id=1").Scan(&name); err != nil || name != "Preserved legacy club" {
		t.Fatal(name, err)
	}
	dba.db.QueryRow("SELECT count(*) FROM guest_driver WHERE name='Preserved guest'").Scan(&count)
	if count != 1 {
		t.Fatal("Legacy guest lost")
	}
	dba.db.QueryRow("SELECT count(*) FROM driving_session WHERE id=?", session.ID).Scan(&count)
	if count != 1 {
		t.Fatal("Rollback schema erased new session")
	}
}
