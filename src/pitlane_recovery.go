package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type cleanupTarget struct {
	Name     string `json:"name"`
	Size     int64  `json:"bytes"`
	Modified int64  `json:"modified_at"`
}

func cleanupTargets() ([]cleanupTarget, string, error) {
	// Only old top-level upload archives owned by this application. No directories,
	// symlinks, game files, restore files, results, logs, or recordings are eligible.
	entries, err := os.ReadDir(TempFolder)
	if err != nil {
		return nil, "", err
	}
	out := []cleanupTarget{}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "sm-upload-") || e.Type()&os.ModeSymlink != 0 || e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, "", err
		}
		if !info.Mode().IsRegular() || time.Since(info.ModTime()) < 24*time.Hour {
			continue
		}
		out = append(out, cleanupTarget{e.Name(), info.Size(), info.ModTime().UnixNano()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	raw, _ := json.Marshal(out)
	hash := sha256.Sum256(raw)
	return out, hex.EncodeToString(hash[:]), nil
}
func apiCleanupPreview(c *gin.Context) {
	targets, key, err := cleanupTargets()
	if err != nil {
		apiDbError(c, err)
		return
	}
	c.JSON(200, gin.H{"targets": targets, "token": key, "checked_at": time.Now().UnixMilli(), "exclusions": []string{"Uploads modified in the last 24 hours", "All directories and symlinks", "Installed content and server run files", "Recordings and media", "Databases and staged restores"}})
}
func apiCleanupApply(c *gin.Context) {
	var request struct {
		Token string `json:"token"`
	}
	if c.ShouldBindJSON(&request) != nil {
		apiBadRequest(c, "Preview cleanup first")
		return
	}
	targets, key, err := cleanupTargets()
	if err != nil {
		apiDbError(c, err)
		return
	}
	if request.Token != key {
		apiError(c, 409, "preview_changed", "Cleanup targets changed. Review a fresh preview.")
		return
	}
	if ContentJobs != nil {
		for _, job := range ContentJobs.ListVisible() {
			if job.Status == "queued" || job.Status == "running" {
				apiError(c, 409, "import_active", "Wait for active imports to finish before cleaning up")
				return
			}
		}
	}
	removed := []string{}
	for _, target := range targets {
		path := filepath.Join(TempFolder, target.Name)
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() || info.Size() != target.Size || info.ModTime().UnixNano() != target.Modified {
			c.JSON(409, gin.H{"removed": removed, "error": gin.H{"message": "A cleanup target changed. Refresh the preview."}})
			return
		}
		if e = os.Remove(path); e != nil {
			c.JSON(500, gin.H{"removed": removed, "error": gin.H{"message": "Could not remove an upload archive. Refresh the preview."}})
			return
		}
		removed = append(removed, target.Name)
	}
	c.JSON(200, gin.H{"removed": removed, "completed_at": time.Now().UnixMilli()})
}
