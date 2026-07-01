package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type resultFileSummary struct {
	File       string `json:"file"`
	InstanceId int    `json:"instance_id"`
	ModifiedAt int64  `json:"modified_at"`
	Type       string `json:"type"`
	Track      string `json:"track"`
	Winner     string `json:"winner"`
	Entries    int    `json:"entries"`
	BestLapMs  int    `json:"best_lap_ms"`
}

func apiResultFiles(c *gin.Context) {
	items := make([]resultFileSummary, 0)
	for _, inst := range Instances.All() {
		for _, p := range resultJSONFiles(inst.Dir()) {
			if s, ok := parseResultFile(p, inst.Id()); ok {
				items = append(items, s)
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModifiedAt > items[j].ModifiedAt })
	if len(items) > 200 {
		items = items[:200]
	}
	c.PureJSON(200, gin.H{"items": items})
}

func resultJSONFiles(dir string) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, d := range []string{filepath.Join(dir, "results"), filepath.Join(dir, "result"), filepath.Join(dir, "out")} {
		_ = filepath.WalkDir(d, func(p string, de os.DirEntry, err error) error {
			if err != nil || de == nil || de.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(p), ".json") {
				add(p)
			}
			return nil
		})
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "*result*.json"))
	for _, p := range matches {
		add(p)
	}
	return out
}

func parseResultFile(path string, instanceId int) (resultFileSummary, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return resultFileSummary{}, false
	}
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return resultFileSummary{}, false
	}
	st, err := os.Stat(path)
	if err != nil {
		return resultFileSummary{}, false
	}
	cars := firstMapArray(raw, "cars")
	results := firstMapArray(raw, "result", "results")
	if len(cars) == 0 && len(results) == 0 {
		return resultFileSummary{}, false
	}
	s := resultFileSummary{
		File:       filepath.Base(path),
		InstanceId: instanceId,
		ModifiedAt: st.ModTime().UnixMilli(),
		Type:       firstString(raw, "type", "sessiontype", "session"),
		Track:      firstString(raw, "track", "trackname", "trackconfig"),
	}
	if s.Type == "" {
		s.Type = "Result"
	}
	s.Entries = len(cars)
	if s.Entries == 0 {
		s.Entries = len(results)
	}
	best := 0
	if len(cars) > 0 {
		winnerCar := cars[0]
		winnerPos := 1<<31 - 1
		for _, row := range results {
			pos := firstInt(row, "position", "pos")
			if pos <= 0 || pos >= winnerPos {
				continue
			}
			if car := carById(cars, firstInt(row, "carid", "car_id")); car != nil {
				winnerCar = car
				winnerPos = pos
			}
		}
		s.Winner = firstString(winnerCar, "drivername", "driver", "name")
	}
	for _, car := range append(cars, results...) {
		if lap := firstInt(car, "bestlap", "bestlapms", "best_lap_ms", "laptime", "lap_time"); lap > 0 && (best == 0 || lap < best) {
			best = lap
		}
	}
	s.BestLapMs = best
	if s.ModifiedAt == 0 {
		s.ModifiedAt = time.Now().UnixMilli()
	}
	return s, true
}

func normKey(k string) string {
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(k))
}

func carById(cars []map[string]any, id int) map[string]any {
	for _, car := range cars {
		if firstInt(car, "carid", "car_id") == id {
			return car
		}
	}
	return nil
}

func firstString(v any, keys ...string) string {
	want := map[string]bool{}
	for _, k := range keys {
		want[normKey(k)] = true
	}
	var found string
	var walk func(any)
	walk = func(x any) {
		if found != "" {
			return
		}
		switch t := x.(type) {
		case map[string]any:
			for k, v := range t {
				if want[normKey(k)] {
					if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
						found = strings.TrimSpace(s)
						return
					}
				}
			}
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	return found
}

func firstInt(v any, keys ...string) int {
	want := map[string]bool{}
	for _, k := range keys {
		want[normKey(k)] = true
	}
	var found int
	var walk func(any)
	walk = func(x any) {
		if found > 0 {
			return
		}
		if m, ok := x.(map[string]any); ok {
			for k, v := range m {
				if !want[normKey(k)] {
					continue
				}
				switch n := v.(type) {
				case float64:
					found = int(n)
				case string:
					found, _ = strconv.Atoi(n)
				}
				return
			}
		}
	}
	walk(v)
	return found
}

func firstMapArray(v any, keys ...string) []map[string]any {
	var found []map[string]any
	var walk func(any)
	walk = func(x any) {
		if found != nil {
			return
		}
		switch t := x.(type) {
		case map[string]any:
			for _, wantKey := range keys {
				for k, v := range t {
					if normKey(k) != normKey(wantKey) {
						continue
					}
					if arr, ok := v.([]any); ok {
						for _, item := range arr {
							if m, ok := item.(map[string]any); ok {
								found = append(found, m)
							}
						}
						return
					}
				}
			}
			for _, v := range t {
				walk(v)
			}
		case []any:
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	if found == nil {
		return []map[string]any{}
	}
	return found
}
