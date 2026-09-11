package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ContentMetadata struct {
	Kind        string   `json:"kind"`
	Key         string   `json:"key"`
	Layout      string   `json:"layout"`
	Revision    int      `json:"revision"`
	DisplayName string   `json:"display_name"`
	Tags        []string `json:"tags"`
	Notes       string   `json:"notes"`
	Archived    bool     `json:"archived"`
}

const metadataColumns = "kind,key,layout,revision,display_name,tags,notes,archived"

func scanMetadata(row interface{ Scan(...any) error }) (ContentMetadata, error) {
	var m ContentMetadata
	var tags string
	err := row.Scan(&m.Kind, &m.Key, &m.Layout, &m.Revision, &m.DisplayName, &tags, &m.Notes, &m.Archived)
	if err == nil {
		err = json.Unmarshal([]byte(tags), &m.Tags)
	}
	return m, err
}
func metadataIdentity(c *gin.Context) (ContentMetadata, bool) {
	m := ContentMetadata{Kind: c.Param("kind"), Key: c.Param("key"), Layout: c.Query("layout"), Tags: []string{}}
	if (m.Kind != "car" && m.Kind != "track" && m.Kind != "weather") || !safeSegment(m.Key) || (m.Layout != "" && !safeSegment(m.Layout)) || (m.Kind != "track" && m.Layout != "") {
		apiBadRequest(c, "Invalid content identity")
		return m, false
	}
	return m, true
}
func apiContentMetadataList(c *gin.Context) {
	rows, err := Dba.db.Query("SELECT " + metadataColumns + " FROM content_metadata ORDER BY kind,key,layout")
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer rows.Close()
	items := []ContentMetadata{}
	for rows.Next() {
		m, e := scanMetadata(rows)
		if e != nil {
			apiDbError(c, e)
			return
		}
		items = append(items, m)
	}
	if err = rows.Err(); err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func apiContentMetadata(c *gin.Context) {
	m, ok := metadataIdentity(c)
	if !ok {
		return
	}
	saved, err := scanMetadata(Dba.db.QueryRow("SELECT "+metadataColumns+" FROM content_metadata WHERE kind=? AND key=? AND layout=?", m.Kind, m.Key, m.Layout))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		apiDbError(c, err)
		return
	}
	if err == nil {
		m = saved
	}
	usage, err := contentUsage(m.Kind, m.Key, m.Layout, false)
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"metadata": m, "usage": usage})
}
func apiContentMetadataSave(c *gin.Context) {
	identity, ok := metadataIdentity(c)
	if !ok {
		return
	}
	var m ContentMetadata
	if c.ShouldBindJSON(&m) != nil {
		apiBadRequest(c, "Invalid metadata")
		return
	}
	m.Kind, m.Key, m.Layout = identity.Kind, identity.Key, identity.Layout
	if len(m.DisplayName) > 160 || len(m.Notes) > 10000 || len(m.Tags) > 40 {
		apiBadRequest(c, "Use a name up to 160 characters, notes up to 10,000 characters and up to 40 tags")
		return
	}
	for i, t := range m.Tags {
		m.Tags[i] = strings.TrimSpace(t)
		if len(m.Tags[i]) < 1 || len(m.Tags[i]) > 60 {
			apiBadRequest(c, "Each tag needs 1–60 characters")
			return
		}
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer tx.Rollback()
	before, e := scanMetadata(tx.QueryRow("SELECT "+metadataColumns+" FROM content_metadata WHERE kind=? AND key=? AND layout=?", m.Kind, m.Key, m.Layout))
	if errors.Is(e, sql.ErrNoRows) {
		before = identity
	} else if e != nil {
		apiDbError(c, e)
		return
	}
	if before.Revision != m.Revision {
		apiError(c, 409, "revision_conflict", "Metadata changed. Reload before saving; your edit has not overwritten it.")
		return
	}
	m.Revision++
	tags, _ := json.Marshal(m.Tags)
	_, err = tx.Exec("INSERT INTO content_metadata("+metadataColumns+") VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(kind,key,layout) DO UPDATE SET revision=excluded.revision,display_name=excluded.display_name,tags=excluded.tags,notes=excluded.notes,archived=excluded.archived", m.Kind, m.Key, m.Layout, m.Revision, m.DisplayName, string(tags), m.Notes, m.Archived)
	if err == nil {
		old, _ := json.Marshal(before)
		next, _ := json.Marshal(m)
		_, err = tx.Exec("INSERT INTO metadata_change(kind,key,layout,before_json,after_json,created_at) VALUES(?,?,?,?,?,?)", m.Kind, m.Key, m.Layout, string(old), string(next), time.Now().UnixMilli())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, m)
}
func contentArchived(kind, key, layout string) bool {
	var archived bool
	_ = Dba.db.QueryRow("SELECT archived FROM content_metadata WHERE kind=? AND key=? AND layout=?", kind, key, layout).Scan(&archived)
	return archived
}

// Usage is exact foreign-key membership in saved setups, never display-name matching.
// All layouts is used by disk deletion because that operation removes the track folder.
func contentUsage(kind, key, layout string, allLayouts bool) ([]map[string]any, error) {
	condition := ""
	args := []any{key}
	switch kind {
	case "track":
		condition = "e.cache_track_key=?"
		if !allLayouts {
			condition += " AND COALESCE(e.cache_track_config,'')=?"
			args = append(args, layout)
		}
	case "car":
		condition = "EXISTS(SELECT 1 FROM user_class_entry g WHERE g.user_class_id=e.class_id AND g.cache_car_key=?)"
	case "weather":
		condition = "EXISTS(SELECT 1 FROM user_time_weather w WHERE w.user_time_id=e.time_id AND w.graphics=?)"
	default:
		return nil, errors.New("Invalid content kind")
	}
	rows, err := Dba.db.Query("SELECT e.id,COALESCE(e.name,''),s.id,COALESCE(s.lifecycle,'reusable'),(SELECT count(*) FROM server_event q WHERE q.user_event_id=e.id AND q.finished=0) FROM user_event e LEFT JOIN driving_session s ON s.event_id=e.id WHERE "+condition+" ORDER BY e.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var event, pending int
		var name, life string
		var session *int
		if err = rows.Scan(&event, &name, &session, &life, &pending); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"event_id": event, "session_id": session, "name": name, "lifecycle": life, "pending_queue_items": pending})
	}
	return out, rows.Err()
}
func apiMetadataChanges(c *gin.Context) {
	rows, err := Dba.db.Query("SELECT id,kind,key,layout,before_json,after_json,created_at,undone_at FROM metadata_change ORDER BY id DESC LIMIT 100")
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int
		var kind, key, layout, before, after string
		var created int64
		var undone *int64
		if err = rows.Scan(&id, &kind, &key, &layout, &before, &after, &created, &undone); err != nil {
			apiDbError(c, err)
			return
		}
		items = append(items, map[string]any{"id": id, "kind": kind, "key": key, "layout": layout, "before": json.RawMessage(before), "after": json.RawMessage(after), "created_at": created, "undone_at": undone})
	}
	c.JSON(200, gin.H{"items": items})
}
func apiMetadataUndo(c *gin.Context) {
	id, ok := pathId(c)
	if !ok {
		return
	}
	tx, err := Dba.db.Begin()
	if err != nil {
		apiDbError(c, err)
		return
	}
	defer tx.Rollback()
	var beforeJSON, afterJSON string
	var undone *int64
	if err = tx.QueryRow("SELECT before_json,after_json,undone_at FROM metadata_change WHERE id=?", id).Scan(&beforeJSON, &afterJSON, &undone); err != nil {
		apiNotFound(c)
		return
	}
	if undone != nil {
		apiError(c, 409, "already_undone", "This change has already been undone")
		return
	}
	var before, after ContentMetadata
	if json.Unmarshal([]byte(beforeJSON), &before) != nil || json.Unmarshal([]byte(afterJSON), &after) != nil {
		apiDbError(c, errors.New("Invalid stored change"))
		return
	}
	current, err := scanMetadata(tx.QueryRow("SELECT "+metadataColumns+" FROM content_metadata WHERE kind=? AND key=? AND layout=?", after.Kind, after.Key, after.Layout))
	if err != nil || current.Revision != after.Revision {
		apiError(c, 409, "later_changes", "Later edits exist. Undo would overwrite them; review the current metadata instead.")
		return
	}
	tags, _ := json.Marshal(before.Tags)
	_, err = tx.Exec("UPDATE content_metadata SET revision=revision+1,display_name=?,tags=?,notes=?,archived=? WHERE kind=? AND key=? AND layout=?", before.DisplayName, string(tags), before.Notes, before.Archived, after.Kind, after.Key, after.Layout)
	if err == nil {
		_, err = tx.Exec("UPDATE metadata_change SET undone_at=? WHERE id=?", time.Now().UnixMilli(), id)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": fmt.Sprintf("Reversed metadata change %d", id)})
}
