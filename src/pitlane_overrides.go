package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Overrides are session-owned inputs to the shared renderer. Only known driving
// controls are accepted; ports, paths, credentials, capacity and content identity
// remain governed by their validated models.
func applyDrivingOverrides(setup DrivingSetup) (DrivingSetup, error) {
	if len(setup.CustomINI) > 16000 {
		return setup, fmt.Errorf("Custom INI is limited to 16,000 characters")
	}
	type control struct {
		dest     **int
		min, max int
	}
	controls := map[string]control{
		"PRACTICE.TIME": {&setup.Phases.PracticeTime, 1, 1440}, "PRACTICE.IS_OPEN": {&setup.Phases.PracticeIsOpen, 0, 1},
		"QUALIFY.TIME": {&setup.Phases.QualifyTime, 1, 1440}, "QUALIFY.IS_OPEN": {&setup.Phases.QualifyIsOpen, 0, 1},
		"RACE.TIME": {&setup.Phases.RaceTime, 0, 1440}, "RACE.WAIT_TIME": {&setup.Phases.RaceWaitTime, 0, 3600}, "RACE.IS_OPEN": {&setup.Phases.RaceIsOpen, 0, 2},
		"DYNAMIC_TRACK.SESSION_START": {&setup.Difficulty.SessionStart, 0, 100}, "DYNAMIC_TRACK.SESSION_TRANSFER": {&setup.Difficulty.SessionTransfer, 0, 100}, "DYNAMIC_TRACK.LAP_GAIN": {&setup.Difficulty.LapGain, 0, 1000},
		"SERVER.ABS_ALLOWED": {&setup.Difficulty.AbsAllowed, 0, 2}, "SERVER.TC_ALLOWED": {&setup.Difficulty.TcAllowed, 0, 2},
		"SERVER.AUTOCLUTCH_ALLOWED": {&setup.Difficulty.AutoclutchAllowed, 0, 1}, "SERVER.TYRE_BLANKETS_ALLOWED": {&setup.Difficulty.TyreBlanketsAllowed, 0, 1},
		"SERVER.FUEL_RATE": {&setup.Difficulty.FuelRate, 0, 500}, "SERVER.TYRE_WEAR_RATE": {&setup.Difficulty.TyreWearRate, 0, 500}, "SERVER.DAMAGE_MULTIPLIER": {&setup.Difficulty.DamageMultiplier, 0, 100},
	}
	scanner := bufio.NewScanner(strings.NewReader(setup.CustomINI))
	section := ""
	seen := map[string]bool{}
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, ";") || strings.HasPrefix(text, "#") {
			continue
		}
		if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
			section = strings.ToUpper(strings.TrimSpace(text[1 : len(text)-1]))
			continue
		}
		pair := strings.SplitN(text, "=", 2)
		if len(pair) != 2 {
			return setup, fmt.Errorf("Custom INI line %d needs KEY=value", line)
		}
		key := section + "." + strings.ToUpper(strings.TrimSpace(pair[0]))
		rule, ok := controls[key]
		if !ok {
			return setup, fmt.Errorf("Custom INI %s is not a supported driving control. Use the dedicated server or content settings", key)
		}
		if seen[key] {
			return setup, fmt.Errorf("Custom INI repeats %s", key)
		}
		seen[key] = true
		value, err := strconv.Atoi(strings.TrimSpace(pair[1]))
		if err != nil || value < rule.min || value > rule.max {
			return setup, fmt.Errorf("Custom INI %s needs a whole number from %d to %d", key, rule.min, rule.max)
		}
		*rule.dest = intPtr(value)
	}
	return setup, scanner.Err()
}
func apiDrivingTemplate(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	event, err := Dba.selectEvent(id)
	if err != nil {
		apiNotFound(c)
		return
	}
	s := DrivingSession{Name: derefOrEmpty(event.Name), Experience: "practice", Lifecycle: "draft", TimeZone: "UTC"}
	if s.Name == "" {
		s.Name = "Driving session"
	}
	s.Setup.TrackKey = derefOrEmpty(event.CacheTrackKey)
	s.Setup.TrackConfig = derefOrEmpty(event.CacheTrackConfig)
	if event.RaceLaps != nil {
		s.Setup.RaceLaps = *event.RaceLaps
	}
	if owned, e := ownedDrivingSetup(id); e != nil {
		apiDbError(c, e)
		return
	} else if owned != nil {
		s.Name, s.Experience, s.Setup = owned.Name, owned.Experience, owned.Setup
		c.JSON(200, s)
		return
	}
	if event.DifficultyId == nil || event.SessionId == nil || event.ClassId == nil || event.TimeId == nil {
		apiBadRequest(c, "Choose the missing presets in this reusable setup before creating a session.")
		return
	}
	cr := ConfigRenderer{}
	if s.Setup.Difficulty, err = cr.sessionDifficulty(event); err != nil {
		apiBadRequest(c, "The reusable setup has no valid assists preset")
		return
	}
	if s.Setup.Phases, err = cr.sessionPhases(event); err != nil {
		apiBadRequest(c, "The reusable setup has no valid phases preset")
		return
	}
	if s.Setup.Grid, err = cr.sessionGrid(event); err != nil {
		apiBadRequest(c, "The reusable setup has no valid grid preset")
		return
	}
	conditions, err := cr.sessionTime(event)
	if err != nil {
		apiBadRequest(c, "The reusable setup has no valid weather preset")
		return
	}
	raw, _ := json.Marshal(conditions)
	if err = json.Unmarshal(raw, &s.Setup.Conditions); err != nil {
		apiDbError(c, err)
		return
	}
	s.Setup.Scoring = Dba.activeDriftScoringMode(event.DriftScoringModeId, nil)
	if isEnabled(s.Setup.Phases.RaceEnabled) {
		s.Experience = "race"
	}
	c.JSON(200, s)
}
