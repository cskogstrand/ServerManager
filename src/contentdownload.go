package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// Mod links let players grab missing cars/tracks straight from this server.
// Two delivery paths are produced when "Append mod links" is enabled:
//   - cfg/cm_content/content.json  — Content Manager auto-download manifest
//   - welcome.txt                  — human-readable links shown on join
// Both point at the public /dl/* endpoints below, which stream a zip built
// live from the install content tree.

// modDownloadBaseURL returns the public base URL players use to reach this
// server's download endpoints. The admin-configured value wins (needed when
// the web port is remapped behind Docker/NAT); otherwise we fall back to the
// detected public IP on the default web port.
func modDownloadBaseURL(cfg UserConfig) string {
	if cfg.ModDownloadUrl != nil {
		if u := strings.TrimRight(strings.TrimSpace(*cfg.ModDownloadUrl), "/"); u != "" {
			return u
		}
	}
	ip := publicIp
	if ip == "" {
		ip = "127.0.0.1"
	}
	return "http://" + ip + ":3030"
}

func carDownloadURL(cfg UserConfig, key string) string {
	return modDownloadBaseURL(cfg) + "/dl/car/" + key + ".zip"
}

func trackDownloadURL(cfg UserConfig, key string) string {
	return modDownloadBaseURL(cfg) + "/dl/track/" + key + ".zip"
}

// streamContentZip zips a content subfolder (e.g. "cars/<key>") to the response
// as <root>.zip, with archive entries rooted at <root>/ so the archive extracts
// directly into the player's content/<cars|tracks> folder. Entries are stored
// uncompressed — mod payloads (kn5/dds/acd) are already compressed, so deflate
// would only burn CPU for no size win.
func streamContentZip(c *gin.Context, contentSub string, root string) {
	base, err := Dba.basepath()
	if err != nil {
		log.Print("download: install path not configured: ", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	srcDir := filepath.Join(base, "content", contentSub)
	if info, err := os.Stat(srcDir); err != nil || !info.IsDir() {
		// Mod folders sometimes drift in case from the cached key; on a
		// case-sensitive filesystem the exact stat misses. Fall back to a
		// case-insensitive scan of the parent before giving up.
		if alt := findDirCaseInsensitive(filepath.Dir(srcDir), filepath.Base(srcDir)); alt != "" {
			srcDir = alt
		} else {
			log.Printf("download 404: no content directory at %q (key %q)", srcDir, root)
			c.Status(http.StatusNotFound)
			return
		}
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+root+`.zip"`)

	zw := zip.NewWriter(c.Writer)
	defer zw.Close()

	err = filepath.Walk(srcDir, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil || fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, p)
		if err != nil {
			return nil
		}

		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:     root + "/" + filepath.ToSlash(rel),
			Method:   zip.Store,
			Modified: fi.ModTime(),
		})
		if err != nil {
			return nil
		}

		src, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer src.Close()

		_, err = io.Copy(w, src)
		return err
	})
	if err != nil {
		log.Print("Error streaming content zip for ", root, ": ", err)
	}
}

// findDirCaseInsensitive returns the absolute path of a subdirectory of parent
// whose name matches `name` ignoring case, or "" if none. Handles mod folders
// whose on-disk casing differs from the cached content key.
func findDirCaseInsensitive(parent, name string) string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(parent, e.Name())
		}
	}
	return ""
}

// apiDownloadCar streams content/cars/<key> as a zip. Public (unauthenticated):
// joining players are not logged into Server Manager. Only a single,
// separator-free path segment is accepted, so it cannot escape the content tree.
func apiDownloadCar(c *gin.Context) {
	key := strings.TrimSuffix(c.Param("key"), ".zip")
	if !safeSegment(key) {
		c.Status(http.StatusBadRequest)
		return
	}
	streamContentZip(c, filepath.Join("cars", key), key)
}

// apiDownloadTrack streams content/tracks/<key> as a zip. See apiDownloadCar.
func apiDownloadTrack(c *gin.Context) {
	key := strings.TrimSuffix(c.Param("key"), ".zip")
	if !safeSegment(key) {
		c.Status(http.StatusBadRequest)
		return
	}
	streamContentZip(c, filepath.Join("tracks", key), key)
}

// cmContentEntry / cmContent model the Content Manager content.json manifest.
type cmContentEntry struct {
	URL string `json:"url"`
}

type cmContent struct {
	Cars  map[string]cmContentEntry `json:"cars,omitempty"`
	Track *cmContentEntry           `json:"track,omitempty"`
}

// writeModLinks reconciles the welcome message and download manifest for the
// event the renderer just prepared. With "Append mod links" enabled it writes
// the CM content.json and appends per-mod download links to welcome.txt;
// otherwise it clears any stale manifest and keeps welcome.txt in sync with the
// plain welcome message (server_cfg.ini points WELCOME_PATH at it). dir is the
// instance working directory acServer runs from.
func writeModLinks(dir string, cfg UserConfig, cr *ConfigRenderer) {
	modlinks := cfg.AppendModlinks != nil && *cfg.AppendModlinks == 1

	if !modlinks {
		// Drop a manifest left over from a previous run so Content Manager
		// can't keep offering links the admin has since turned off.
		os.Remove(filepath.Join(dir, "cfg", "cm_content", "content.json"))
		writeWelcomeFile(dir, cfg, nil, nil, "", "")
		return
	}

	// Only advertise content that actually exists on disk, so Content Manager
	// never gets a 404 download URL (one missing car can abort the whole
	// "install missing content" flow). A car/track in the cache but absent from
	// the live content tree is skipped and logged.
	base, _ := Dba.basepath()
	have := func(sub string) bool {
		if base == "" {
			return true // can't verify — don't drop links
		}
		p := filepath.Join(base, "content", sub)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return true
		}
		return findDirCaseInsensitive(filepath.Dir(p), filepath.Base(p)) != ""
	}

	// Collect unique car keys from the rendered entry list.
	carURLs := map[string]string{}
	carOrder := make([]string, 0)
	seen := map[string]bool{}
	for _, e := range cr.class.Entries {
		if e.CacheCarKey == nil || *e.CacheCarKey == "" {
			continue
		}
		k := *e.CacheCarKey
		if seen[k] {
			continue
		}
		seen[k] = true
		if !have(filepath.Join("cars", k)) {
			log.Printf("mod links: skipping car %q — not found under %s/content/cars", k, base)
			continue
		}
		carURLs[k] = carDownloadURL(cfg, k)
		carOrder = append(carOrder, k)
	}

	trackKey := ""
	trackURL := ""
	if cr.track.Key != nil && *cr.track.Key != "" && have(filepath.Join("tracks", *cr.track.Key)) {
		trackKey = *cr.track.Key
		trackURL = trackDownloadURL(cfg, trackKey)
	}

	writeContentManifest(dir, carURLs, trackURL)
	writeWelcomeFile(dir, cfg, carOrder, carURLs, trackKey, trackURL)
}

func writeContentManifest(dir string, carURLs map[string]string, trackURL string) {
	manifest := cmContent{}
	if len(carURLs) > 0 {
		manifest.Cars = map[string]cmContentEntry{}
		for k, u := range carURLs {
			manifest.Cars[k] = cmContentEntry{URL: u}
		}
	}
	if trackURL != "" {
		manifest.Track = &cmContentEntry{URL: trackURL}
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.Print("Could not marshal content.json: ", err)
		return
	}

	cmDir := filepath.Join(dir, "cfg", "cm_content")
	if err := os.MkdirAll(cmDir, os.ModePerm); err != nil {
		log.Print("Could not create cm_content dir: ", err)
		return
	}
	if err := os.WriteFile(filepath.Join(cmDir, "content.json"), data, 0644); err != nil {
		log.Print("Could not write content.json: ", err)
	}
}

// writeWelcomeFile builds welcome.txt = the admin welcome message, optionally
// followed by a download-links block. server_cfg.ini points WELCOME_PATH here,
// so this becomes the message shown to every player regardless of client. With
// no welcome message and no links there is nothing to show, so the (possibly
// stale) file is removed instead.
func writeWelcomeFile(dir string, cfg UserConfig, carOrder []string, carURLs map[string]string, trackKey string, trackURL string) {
	// server_cfg.ini sets WELCOME_MESSAGE=cfg/welcome.txt (a path, resolved
	// against the working dir), so the file lives next to server_cfg.ini.
	path := filepath.Join(dir, "cfg", "welcome.txt")

	hasLinks := trackURL != "" || len(carOrder) > 0
	welcome := ""
	if cfg.WelcomeMessage != nil {
		welcome = *cfg.WelcomeMessage
	}

	if welcome == "" && !hasLinks {
		os.Remove(path)
		return
	}

	var sb strings.Builder
	if welcome != "" {
		sb.WriteString(welcome)
	}
	if hasLinks {
		if welcome != "" {
			sb.WriteString("\n\n")
		}
		sb.WriteString("=== Missing content? Download it from this server ===\n")
		if trackURL != "" {
			sb.WriteString("Track " + trackKey + ": " + trackURL + "\n")
		}
		for _, k := range carOrder {
			sb.WriteString("Car " + k + ": " + carURLs[k] + "\n")
		}
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		log.Print("Could not write welcome.txt: ", err)
	}
}
