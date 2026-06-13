package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"
)

type ConfigRenderer struct {
	serverCfgIni    *ttemplate.Template
	entryListIni    *ttemplate.Template
	serverCfgResult string
	entryListResult string
	class           UserClass
	track           CacheTrack
	cspRequired     bool
	cspVersion      string
	cspLetter       string
	maxClients      int
	serverEvent     ServerEvent
	renderErr       error
}

type EntryListTemplateData struct {
	Entries []EntryListEntry
}

type EntryListEntry struct {
	CacheCarKey   string
	SkinKey       string
	SpectatorMode int
	DriverName    string
	Team          string
	Guid          string
	Ballast       int
	Restrictor    int
}

// 8:00 AM = -80
// 18:00 PM = 80
// increment of 8 every 30 minutes
func (cr *ConfigRenderer) timeToSunAngle(timeStr *string) int {
	// 13:00 maps to a sun angle of 0 — a safe default when no time is set.
	s := "13:00"
	if timeStr != nil && *timeStr != "" {
		s = *timeStr
	}
	time, err := time.Parse("15:04", s)
	if err != nil {
		log.Print("Could not parse time: ", s, err)
	}

	angle := -80 + (16 * (time.Hour() - 8))
	angle = angle + int(math.Round(float64(time.Minute())/15))*4

	return angle
}

func (cr *ConfigRenderer) writeIni(dir string) {
	cfgfolder := filepath.Join(dir, "cfg")
	err := os.MkdirAll(cfgfolder, os.ModePerm)
	if err != nil {
		log.Print("Could not create temp folder: ", cfgfolder, err)
	}

	err = os.WriteFile(filepath.Join(cfgfolder, "server_cfg.ini"), []byte(cr.serverCfgResult), 0644)
	if err != nil {
		log.Print("Could not write server_cfg.ini: ", err)
	}

	err = os.WriteFile(filepath.Join(cfgfolder, "entry_list.ini"), []byte(cr.entryListResult), 0644)
	if err != nil {
		log.Print("Could not write entry_list.ini: ", err)
	}
}

func reserveSpectatorSlots(entryCount int, capacity int, spectator bool) (normalSlots int, totalClients int, err error) {
	if capacity < 0 {
		capacity = 0
	}
	normalSlots = capacity
	if spectator {
		if capacity <= 1 {
			return 0, 0, fmt.Errorf("spectator mode needs at least 2 available slots")
		}
		normalSlots = capacity - 1
	}
	if entryCount < normalSlots {
		normalSlots = entryCount
	}
	if spectator && normalSlots <= 0 {
		return 0, 0, fmt.Errorf("spectator mode needs at least 1 normal race slot")
	}
	totalClients = normalSlots
	if spectator {
		totalClients++
	}
	return normalSlots, totalClients, nil
}

func spectatorSlotConfigured(instance ServerInstance) bool {
	return isEnabled(instance.SpectatorEnabled) &&
		trimmedStringPtr(instance.SpectatorCarKey) != nil &&
		trimmedStringPtr(instance.SpectatorSkinKey) != nil &&
		trimmedStringPtr(instance.SpectatorName) != nil &&
		trimmedStringPtr(instance.SpectatorGuid) != nil
}

func entryListData(class UserClass, instance ServerInstance) (EntryListTemplateData, UserClass) {
	entries := make([]EntryListEntry, 0, len(class.Entries)+1)
	for _, ent := range class.Entries {
		car := derefOrEmpty(ent.CacheCarKey)
		skin := derefOrEmpty(ent.SkinKey)
		if car == "" {
			continue
		}
		entries = append(entries, EntryListEntry{
			CacheCarKey: car,
			SkinKey:     skin,
			Ballast:     0,
			Restrictor:  0,
		})
	}

	serverClass := class
	serverClass.Entries = append([]UserClassEntry(nil), class.Entries...)
	if spectatorSlotConfigured(instance) {
		car := strings.TrimSpace(*instance.SpectatorCarKey)
		skin := strings.TrimSpace(*instance.SpectatorSkinKey)
		name := strings.TrimSpace(*instance.SpectatorName)
		guid := strings.TrimSpace(*instance.SpectatorGuid)
		entries = append(entries, EntryListEntry{
			CacheCarKey:   car,
			SkinKey:       skin,
			SpectatorMode: 1,
			DriverName:    name,
			Guid:          guid,
			Ballast:       0,
			Restrictor:    0,
		})
		serverClass.Entries = append(serverClass.Entries, UserClassEntry{
			CacheCarKey: &car,
			SkinKey:     &skin,
			Count:       intPtr(1),
		})
	}

	return EntryListTemplateData{Entries: entries}, serverClass
}

func (cr *ConfigRenderer) renderIni(eventId int, instance ServerInstance) {
	cr.renderErr = nil
	r := regexp.MustCompile(`\d{1,3}`)

	event, err := Dba.selectEvent(eventId)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	eventcat, err := Dba.selectEventCategory(*event.EventCategoryId)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	tm, err := Dba.selectTimeWeather(*event.TimeId)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	diff, err := Dba.selectDifficulty(*event.DifficultyId)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	cfg, err := Dba.selectConfig()
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	session, err := Dba.selectSession(*event.SessionId)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	class, err := Dba.selectClassEntries(*event.ClassId)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	// Expand each entry by its car count so one list row can yield N grid slots
	expanded := make([]UserClassEntry, 0, len(class.Entries))
	for _, ent := range class.Entries {
		count := 1
		if ent.Count != nil && *ent.Count > 0 {
			count = *ent.Count
		}
		for i := 0; i < count; i++ {
			expanded = append(expanded, ent)
		}
	}
	class.Entries = expanded

	track, err := Dba.selectCacheTrack(*event.CacheTrackKey, *event.CacheTrackConfig)
	if err != nil {
		log.Print("Database error: ", err)
		return
	}

	if session.QualifyMaxWaitPerc == nil {
		var val = 120
		session.QualifyMaxWaitPerc = &val
	}

	// csp required? build a cspstr to be concat with the track name
	cspstr := ""
	cr.cspVersion = ""
	cr.cspLetter = ""
	if cfg.CspRequired != nil && *cfg.CspRequired > 0 && cfg.CspVersion != nil && *cfg.CspVersion > 0 {
		cr.cspRequired = true
		cspLetter := ""

		cspPhycars := cfg.CspPhycars != nil && *cfg.CspPhycars > 0
		cspPhytracks := cfg.CspPhytracks != nil && *cfg.CspPhytracks > 0
		cspHidepit := cfg.CspHidepit != nil && *cfg.CspHidepit > 0

		if cspPhycars && cspPhytracks && cspHidepit {
			cspLetter = "/../H"
		} else if cspPhycars && cspPhytracks {
			cspLetter = "/../D"
		} else if cspPhycars && cspHidepit {
			cspLetter = "/../F"
		} else if cspPhytracks && cspHidepit {
			cspLetter = "/../G"
		} else if cspPhycars {
			cspLetter = "/../B"
		} else if cspPhytracks {
			cspLetter = "/../C"
		} else if cspHidepit {
			cspLetter = "/../E"
		}

		cr.cspVersion = strconv.Itoa(*cfg.CspVersion)
		cr.cspLetter = strings.TrimPrefix(cspLetter, "/../")
		cspstr = "csp/" + cr.cspVersion + cspLetter + "/../"
	} else {
		cr.cspRequired = false
	}

	// Weather CSP? build new graphics string
	if tm.CspEnabled != nil && *tm.CspEnabled == 1 {
		t := "13:00" // sets the sun angle to zero; a "nice" default/backup value
		tm.Time = &t
		todm := 1
		tm.TimeOfDayMulti = &todm
		for _, wt := range tm.Weathers {
			cspTime, err := time.Parse("15:04", *wt.CspTime)
			if err != nil {
				log.Print("Could not parse csp time:", *wt.CspTime, err)
			}
			cspTimeInt := (cspTime.Hour() * 3600) + (cspTime.Minute() * 60) + cspTime.Second()
			seconds := strconv.Itoa(cspTimeInt)
			mult := strconv.Itoa(*wt.CspTimeOfDayMulti)

			dateStr := ""
			if wt.CspDate != nil && *wt.CspDate != "" {
				cspDate, err := time.Parse("2006-01-02", *wt.CspDate)
				if err != nil {
					log.Print("Could not parse csp date:", *wt.CspDate, err)
				}

				dateStr = "_start=" + strconv.FormatInt(cspDate.Unix(), 10)
			}

			matches := r.FindStringSubmatch(*wt.Graphics)
			*wt.Graphics = matches[0] + "_time=" + seconds + "_mult=" + mult + dateStr
		}
	}

	// Maximum clients defined as the minimum between maxclients, pitboxes and vehicles in class.
	// Spectator mode consumes one slot, so the normal grid is reduced before
	// appending the locked spectator entry.
	stratneeded := false
	slotCapacity := *cfg.MaxClients

	if *track.Pitboxes < slotCapacity {
		stratneeded = true
		slotCapacity = *track.Pitboxes
	}
	normalSlots, maxclients, err := reserveSpectatorSlots(len(class.Entries), slotCapacity, spectatorSlotConfigured(instance))
	if err != nil {
		cr.renderErr = err
		log.Print("Could not render server config: ", err)
		return
	}
	if len(class.Entries) < normalSlots {
		stratneeded = true
	}
	if len(class.Entries) > normalSlots {
		stratneeded = true
	}

	// Strategy needed? re-order cars in the entry list as per selected strategy
	if stratneeded {
		// Random
		if *event.Strategy == 2 {
			rand.Shuffle(len(class.Entries), func(i, j int) { class.Entries[i], class.Entries[j] = class.Entries[j], class.Entries[i] })
		}
		// Cut the list by the max number of clients
		class.Entries = class.Entries[:normalSlots]
	}
	entryData, serverClass := entryListData(class, instance)

	funcMap := ttemplate.FuncMap{
		"derefInt": func(i *int) int {
			if i == nil {
				return 0
			}
			return *i
		},
		"defaultInt": func(i *int, def int) int {
			if i == nil {
				return def
			}
			return *i
		},
		"defaultStr": func(s *string, def string) string {
			if s == nil {
				return def
			}
			return *s
		},
	}

	if cr.serverCfgIni == nil {
		file, err := OpenAsset("/ini/server_cfg.ini")
		if err != nil {
			log.Print("Could not open template file server_cfg.ini: ", err)
		} else {
			defer file.Close()
			tmplStr, err := io.ReadAll(file)
			if err != nil {
				log.Print("Could not read template file server_cfg.ini: ", err)
			}
			cr.serverCfgIni, err = ttemplate.New("server_cfg.ini").Funcs(funcMap).Parse(string(tmplStr))
			if err != nil {
				log.Print("Error parsing server_cfg.ini template: ", err)
			}
		}
	}

	// The lobby title suffix uses the event's custom name when set, otherwise
	// the event category name (legacy behavior).
	eventName := eventcat.Name
	if event.Name != nil && strings.TrimSpace(*event.Name) != "" {
		eventName = event.Name
	}

	data := map[string]any{
		"event":       event,
		"config":      cfg,
		"instance":    instance,
		"diff":        diff,
		"session":     session,
		"time":        tm,
		"class":       serverClass,
		"track":       track,
		"max_clients": maxclients,
		"sunangle":    cr.timeToSunAngle(tm.Time),
		"cspstr":      cspstr,
		"name":        eventName,
	}

	var b bytes.Buffer
	err = cr.serverCfgIni.Execute(&b, data)
	if err != nil {
		log.Print("Error executing server_cfg.ini template: ", err)
	}

	if cr.entryListIni == nil {
		file, err := OpenAsset("/ini/entry_list.ini")
		if err != nil {
			log.Print("Could not open template file entry_list.ini: ", err)
		} else {
			defer file.Close()
			tmplStr, err := io.ReadAll(file)
			if err != nil {
				log.Print("Could not read template file entry_list.ini: ", err)
			}
			cr.entryListIni, err = ttemplate.New("entry_list.ini").Funcs(funcMap).Parse(string(tmplStr))
			if err != nil {
				log.Print("Error parsing entry_list.ini template: ", err)
			}
		}
	}

	var b2 bytes.Buffer
	err = cr.entryListIni.Execute(&b2, entryData)
	if err != nil {
		log.Print("Error executing entry_list.ini template: ", err)
	}

	cr.serverCfgResult = b.String()
	cr.entryListResult = b2.String()
	cr.class = serverClass
	cr.track = track
	cr.maxClients = maxclients
}
