package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

const sqliteMagic = "SQLite format 3\x00"

// applyStagedRestore swaps in a database uploaded via the maintenance page.
// It runs at boot, before the DB is opened, so the live *sql.DB never has its
// file replaced underneath it. The previous DB is kept as smdata.db.prev.
func applyStagedRestore(dbpath string) {
	staged := dbpath + ".restore"
	if _, err := os.Stat(staged); err != nil {
		return
	}

	backup := dbpath + ".prev"
	_ = os.Remove(backup)
	if _, err := os.Stat(dbpath); err == nil {
		if err := os.Rename(dbpath, backup); err != nil {
			log.Print("Could not back up current database before restore: ", err)
			return
		}
	}
	if err := os.Rename(staged, dbpath); err != nil {
		log.Print("Could not apply staged database restore: ", err)
		return
	}
	log.Print("Applied staged database restore; previous database kept at ", backup)
}

// looksLikeSqlite checks the 16-byte SQLite file header so we never stage a
// bogus upload that would brick the next boot.
func looksLikeSqlite(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, len(sqliteMagic))
	if _, err := f.Read(header); err != nil {
		return false
	}
	return string(header) == sqliteMagic
}

// apiMaintenanceRestore stages an uploaded database for the next boot. All
// servers must be stopped; the file is validated as SQLite before staging.
func apiMaintenanceRestore(c *gin.Context) {
	for _, inst := range Instances.All() {
		if inst.isRunning() {
			apiError(c, http.StatusConflict, "running", "Stop every server before restoring a database.")
			return
		}
	}

	file, err := c.FormFile("database")
	if err != nil {
		apiBadRequest(c, "Choose a database file (smdata.db).")
		return
	}

	staged := filepath.Join(ConfigFolder, "smdata.db.restore")
	if err := c.SaveUploadedFile(file, staged); err != nil {
		apiError(c, http.StatusInternalServerError, "io_error", "Could not save the uploaded database.")
		return
	}
	if !looksLikeSqlite(staged) {
		_ = os.Remove(staged)
		apiBadRequest(c, "That file is not a SQLite database. Upload the smdata.db downloaded from Backup.")
		return
	}

	c.PureJSON(http.StatusOK, gin.H{
		"staged":  true,
		"message": "Restore staged. Restart Server Manager to apply it; the current database is kept as smdata.db.prev.",
	})
}
