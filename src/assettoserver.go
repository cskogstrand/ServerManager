package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// IgnoreConfigurationErrors keys relaxed together by the "allow mod content"
// toggle. Both are "missing metadata" errors common to community/drift mods:
// cars without a packed data.acd, and tracks without lat/lon/timezone params.
var relaxableConfigErrors = []string{"MissingCarChecksums", "MissingTrackParams"}

// assettoServerMu serialises the one-time download/extract of the shared
// install dir across concurrently-starting instances.
var assettoServerMu sync.Mutex

// AssettoServer is a drop-in replacement for the stock Kunos acServer. It reads
// the same cfg/server_cfg.ini + cfg/entry_list.ini + cfg/welcome.txt that the
// renderer already produces, and natively serves the Content Manager
// /api/details "content" field from cfg/cm_content/content.json — which is what
// makes the "Install missing content" button appear. SM does not ship the
// binary (it's AGPL and ~45 MB); it is fetched on demand from GitHub releases
// and cached under ConfigFolder.

const assettoServerVersion = "v0.0.54"

// assettoServerArchive returns the release asset URL and its archive kind
// ("tar.gz" or "zip") for the OS the dedicated server runs on.
func assettoServerArchive() (url string, kind string) {
	base := "https://github.com/compujuckel/AssettoServer/releases/download/" + assettoServerVersion + "/"
	if runtime.GOOS == "windows" {
		return base + "assetto-server-win-x64.zip", "zip"
	}
	return base + "assetto-server-linux-x64.tar.gz", "tar.gz"
}

// randPassword returns an n-char alphanumeric string from a crypto source.
func randPassword(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Print("Could not read random bytes for password: ", err)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// ensureAssettoServerAdminPassword guarantees a non-empty ADMIN_PASSWORD, which
// AssettoServer requires (stock acServer tolerates an empty one). If unset, a
// random password is generated and persisted so the operator can see/change it
// in Settings → Access. Returns the (possibly updated) config.
func ensureAssettoServerAdminPassword(cfg UserConfig) UserConfig {
	if cfg.AdminPassword != nil && strings.TrimSpace(*cfg.AdminPassword) != "" {
		return cfg
	}
	pw := randPassword(12)
	cfg.AdminPassword = &pw
	if _, err := Dba.updateConfig(cfg); err != nil {
		log.Print("Could not persist generated admin password: ", err)
	} else {
		log.Print("AssettoServer requires an admin password; generated one (Settings → Access)")
	}
	return cfg
}

func assettoServerBinaryName() string {
	if runtime.GOOS == "windows" {
		return "AssettoServer.exe"
	}
	return "AssettoServer"
}

// assettoServerInstallDir is the cached, extracted release tree, shared by all
// instances.
func assettoServerInstallDir() string {
	return filepath.Join(ConfigFolder, "assettoserver", assettoServerVersion)
}

func assettoServerInstalled() bool {
	bin := filepath.Join(assettoServerInstallDir(), assettoServerBinaryName())
	st, err := os.Stat(bin)
	return err == nil && !st.IsDir()
}

// ensureAssettoServerInstalled downloads and extracts the release into the
// shared install dir if the binary is not already present, then returns its
// path. Concurrent callers are serialised so two instances starting at once
// don't both download.
func ensureAssettoServerInstalled() (string, error) {
	assettoServerMu.Lock()
	defer assettoServerMu.Unlock()

	dir := assettoServerInstallDir()
	bin := filepath.Join(dir, assettoServerBinaryName())
	if st, err := os.Stat(bin); err == nil && !st.IsDir() {
		return bin, nil
	}

	if err := downloadAssettoServer(dir); err != nil {
		return "", err
	}

	if st, err := os.Stat(bin); err != nil || st.IsDir() {
		return "", fmt.Errorf("AssettoServer binary missing after extract: %s", bin)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(bin, 0755); err != nil {
			log.Print("Could not chmod AssettoServer binary: ", err)
		}
		// FastLaneUtils is invoked by the server for AI splines.
		_ = os.Chmod(filepath.Join(dir, "utils", "FastLaneUtils"), 0755)
	}
	return bin, nil
}

func downloadAssettoServer(destDir string) error {
	url, kind := assettoServerArchive()
	log.Print("Downloading AssettoServer ", assettoServerVersion, " from ", url)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AssettoServer download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "assettoserver-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	log.Print("Extracting AssettoServer to ", destDir)
	if kind == "zip" {
		return extractZip(tmp.Name(), destDir)
	}
	return extractTarGz(tmp.Name(), destDir)
}

// safeJoin joins base+name and rejects entries that would escape base (zip-slip
// / tar traversal guard).
func safeJoin(base, name string) (string, error) {
	target := filepath.Join(base, name)
	cleanBase := filepath.Clean(base)
	if target != cleanBase && !strings.HasPrefix(target, cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive path: %s", name)
	}
	return target, nil
}

func extractTarGz(srcFile, destDir string) error {
	f, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}

		target, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeSymlink:
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func extractZip(srcFile, destDir string) error {
	zr, err := zip.OpenReader(srcFile)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, zf := range zr.File {
		target, err := safeJoin(destDir, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, zf.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		out.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// provisionAssettoServerRunDir lays out an instance working directory so the
// cached AssettoServer can run from it: the executable and its native libs are
// copied in (so the dynamic loader and AppContext.BaseDirectory resolve inside
// the run dir), while the large read-only content/system trees and the optional
// plugins/utils folders are symlinked from the install. The cfg/ tree (written
// by the renderer + writeModLinks) is left untouched.
func provisionAssettoServerRunDir(dir string) error {
	installDir := assettoServerInstallDir()

	// Executable + native libs must be real files alongside each other.
	bin := assettoServerBinaryName()
	for _, name := range []string{bin, "libcsp_xxhash3.so", "libsteam_api.so", "steam_appid.txt"} {
		src := filepath.Join(installDir, name)
		if _, err := os.Stat(src); err != nil {
			continue // win-x64 / future layouts may not ship every file
		}
		if err := copyFileIfNewer(src, filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("copy %s: %w", name, err)
		}
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(filepath.Join(dir, bin), 0755)
	}

	// Support trees can be symlinked (read-only at runtime).
	for _, name := range []string{"plugins", "utils"} {
		src := filepath.Join(installDir, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := linkInto(src, filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("link %s: %w", name, err)
		}
	}

	// Content + system come from the live install so AssettoServer sees the
	// full car/track data (checksums, surfaces) without re-extraction.
	base, err := Dba.basepath()
	if err != nil {
		return err
	}
	if err := linkInto(filepath.Join(base, "content"), filepath.Join(dir, "content")); err != nil {
		return fmt.Errorf("link content: %w", err)
	}
	if err := linkInto(filepath.Join(base, "system"), filepath.Join(dir, "system")); err != nil {
		return fmt.Errorf("link system: %w", err)
	}
	return nil
}

// linkInto points dst at src via a symlink, replacing whatever non-matching
// thing is already there (e.g. a real content/ dir left by a previous Kunos
// run). Falls back to nothing on Windows symlink-permission errors — callers
// treat that as fatal via the returned error.
func linkInto(src, dst string) error {
	if existing, err := os.Readlink(dst); err == nil {
		if existing == src {
			return nil
		}
		os.Remove(dst)
	} else if _, err := os.Lstat(dst); err == nil {
		// Exists but is not a symlink (real dir/file) — drop it.
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	return os.Symlink(src, dst)
}

// trackParamsURL is the same source AssettoServer pulls its default track
// params (lat/lon/timezone) from.
const trackParamsURL = "https://raw.githubusercontent.com/ac-custom-shaders-patch/acc-extension-config/master/config/data_track_params.ini"

// ensureAssettoServerTrackParams guarantees cfg/data_track_params.ini has an
// entry for the event's track. AssettoServer's WeatherManager aborts startup if
// the track is missing ("No track params found"), which is common for mod
// tracks not in the official list. Stock tracks keep their real coordinates
// (from the official file AssettoServer downloads, or that we fetch here);
// only genuinely-missing tracks get a fallback entry, which the operator can
// correct in the file afterwards.
func ensureAssettoServerTrackParams(dir, trackKey, trackName string) {
	if trackKey == "" {
		return
	}
	path := filepath.Join(dir, "cfg", "data_track_params.ini")

	existing, _ := os.ReadFile(path)
	content := string(existing)
	hadFile := len(existing) > 0

	// Fresh run dir: mirror AssettoServer and pull the official list so stock
	// tracks keep correct coordinates instead of our fallback.
	if !hadFile {
		if b := fetchTrackParams(); b != "" {
			content = b
		}
	}

	if iniHasSection(content, trackKey) {
		if !hadFile && content != "" {
			writeTrackParams(path, content)
		}
		return
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	name := trackName
	if name == "" {
		name = trackKey
	}
	content += fmt.Sprintf("\n[%s]\nNAME=%s\nLATITUDE=51.48\nLONGITUDE=0\nTIMEZONE=Europe/London\n", trackKey, name)
	writeTrackParams(path, content)
	log.Print("Added fallback track params for ", trackKey, " — edit cfg/data_track_params.ini for the real location/timezone")
}

func writeTrackParams(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Print("Could not create cfg dir for data_track_params.ini: ", err)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Print("Could not write data_track_params.ini: ", err)
	}
}

func fetchTrackParams() string {
	resp, err := http.Get(trackParamsURL)
	if err != nil {
		log.Print("Could not fetch track params list: ", err)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Print("Could not fetch track params list: ", resp.Status)
		return ""
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(b)
}

// iniHasSection reports whether content contains a [section] header (matched
// case-insensitively, like AssettoServer's track lookup).
func iniHasSection(content, section string) bool {
	target := "[" + strings.ToLower(section) + "]"
	for _, line := range strings.Split(content, "\n") {
		if strings.ToLower(strings.TrimSpace(line)) == target {
			return true
		}
	}
	return false
}

// extraCfgMinSize is the floor below which an extra_cfg.yml can't be a real
// AssettoServer config (its default dump is several KB). Anything smaller is a
// stub — e.g. one an older Server Manager build wrote — which AssettoServer's
// YAML parser rejects ("unresolved tag '!extra_cfg.yml'").
const extraCfgMinSize = 400

// ensureAssettoServerExtraCfg reconciles cfg/extra_cfg.yml so the relaxable
// IgnoreConfigurationErrors keys match the operator's choice. Many
// community/drift mods ship cars without a packed data.acd and tracks without
// params; without relaxing, AssettoServer refuses to start ("No data.acd
// found", "No track params found"). Relaxing weakens that validation — intended
// for trusted/LAN servers.
//
// Server Manager never authors the file from scratch — a partial YAML document
// is not accepted by AssettoServer. It only patches a full, valid config:
// AssettoServer generates one on first run, and a fresh instance is seeded from
// the default instance's working config so the relax keys apply on the first
// start. If neither is available the (possibly stub) file is removed so
// AssettoServer regenerates its own valid default.
func ensureAssettoServerExtraCfg(dir string, relax bool) {
	path := filepath.Join(dir, "cfg", "extra_cfg.yml")
	desired := "false"
	if relax {
		desired = "true"
	}

	data, _ := os.ReadFile(path)
	if len(data) < extraCfgMinSize {
		// Not a real config (missing, or a stub). Seed from the default
		// instance's valid config if we have one; otherwise drop it and let
		// AssettoServer create a valid default on startup.
		seed := seedExtraCfgFromDefault(dir)
		if seed == "" {
			if len(data) > 0 {
				os.Remove(path)
			}
			return
		}
		data = []byte(seed)
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			log.Print("Could not create cfg dir for extra_cfg.yml: ", mkErr)
			return
		}
		if wErr := os.WriteFile(path, data, 0644); wErr != nil {
			log.Print("Could not seed extra_cfg.yml: ", wErr)
			return
		}
	}

	content := string(data)
	for _, k := range relaxableConfigErrors {
		content = setIgnoreConfigError(content, k, desired)
	}
	// The Kunos-compatible UDP plugin interface is what feeds Server Manager's
	// telemetry (players, drivers, live positions). AssettoServer leaves it off
	// by default, so always enable it.
	content = setYamlTopLevelBool(content, "EnableLegacyPluginInterface", true)
	if content == string(data) {
		return
	}
	if wErr := os.WriteFile(path, []byte(content), 0644); wErr != nil {
		log.Print("Could not update extra_cfg.yml: ", wErr)
	}
}

// setYamlTopLevelBool ensures a top-level `key: value` boolean exists, patching
// it in place if present (matching only an inline scalar, never a nested
// mapping) or appending it otherwise.
func setYamlTopLevelBool(content, key string, value bool) string {
	v := "false"
	if value {
		v = "true"
	}
	re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(key) + `:[ \t]*).*$`)
	if re.MatchString(content) {
		return re.ReplaceAllString(content, "${1}"+v)
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + key + ": " + v + "\n"
}

// seedExtraCfgFromDefault returns the default instance's extra_cfg.yml (a real
// AssettoServer-generated config) to seed a fresh instance, or "" if this is
// the default instance or no valid source exists.
func seedExtraCfgFromDefault(dir string) string {
	if filepath.Clean(dir) == filepath.Clean(TempFolder) {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(TempFolder, "cfg", "extra_cfg.yml"))
	if err != nil || len(data) < extraCfgMinSize {
		return ""
	}
	return string(data)
}

// setIgnoreConfigError ensures `key: value` exists under the
// IgnoreConfigurationErrors mapping, patching the value in place if present,
// inserting it under an existing section, or appending the section otherwise.
func setIgnoreConfigError(content, key, value string) string {
	re := regexp.MustCompile(`(?m)^(\s*` + key + `:\s*).*$`)
	if re.MatchString(content) {
		return re.ReplaceAllString(content, "${1}"+value)
	}
	line := "  " + key + ": " + value
	if strings.Contains(content, "IgnoreConfigurationErrors:") {
		return strings.Replace(content, "IgnoreConfigurationErrors:",
			"IgnoreConfigurationErrors:\n"+line, 1)
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + "IgnoreConfigurationErrors:\n" + line + "\n"
}

// Sentinels delimiting the csp_extra_options.ini block Server Manager owns.
// Everything between them is rewritten/removed by the drift-score toggle;
// operator-authored content outside the block is never touched.
const (
	driftCspBlockStart = "; >>> Server Manager: drift score (managed)"
	driftCspBlockEnd   = "; <<< Server Manager: drift score (managed)"
)

// driftScriptURL is the public URL CSP clients fetch the drift-score HUD/feeder
// from. It points at the rendered endpoint (not the raw /static file) so the
// per-instance telemetry ingest URL+token is injected into the script body. It
// reuses the same public base players already use for mod downloads
// (admin-configured value, else the detected public IP on the web port), so it
// carries the same reachability requirement: SM's web port must be reachable by
// joining clients — which the WebSocket ingest shares (same host:port).
func driftScriptURL(cfg UserConfig, instanceId int) string {
	// {SessionID} is a CSP script-URL placeholder: "0-based index of entry list
	// used by the player" — i.e. the ACSP car id. CSP substitutes it per client
	// when fetching the script, so the server learns each client's car id
	// without the script having to figure out its own session slot.
	return modDownloadBaseURL(cfg) + "/sm/lua/driftscore?instance=" + strconv.Itoa(instanceId) + "&sid={SessionID}"
}

// stripDriftCspBlock removes any previously-written managed block (including a
// leading blank-line run and trailing newline) so re-runs never duplicate it.
func stripDriftCspBlock(content string) string {
	re := regexp.MustCompile(`(?s)\n*` + regexp.QuoteMeta(driftCspBlockStart) + `.*?` + regexp.QuoteMeta(driftCspBlockEnd) + `[ \t]*\n?`)
	return re.ReplaceAllString(content, "")
}

// nextCspScriptIndex returns one past the highest [SCRIPT_n] index already in
// content, so the managed block never collides with operator scripts. Defaults
// to 1 when there are none.
func nextCspScriptIndex(content string) int {
	re := regexp.MustCompile(`(?mi)^\s*\[SCRIPT_(\d+)\]`)
	max := 0
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max + 1
}

// ensureAssettoServerCspExtraOptions reconciles cfg/csp_extra_options.ini so the
// drift-score CSP Lua script is served to clients when enabled. AssettoServer
// encodes this file into the handshake and pushes it to connecting CSP clients;
// the [SCRIPT_n] entry tells CSP to load the HUD from scriptURL. When disabled
// the managed block is stripped; if nothing else remains the file is removed.
// Only meaningful on the AssettoServer engine — stock acServer ignores it.
func ensureAssettoServerCspExtraOptions(dir string, enable bool, scriptURL string) {
	path := filepath.Join(dir, "cfg", "csp_extra_options.ini")
	data, _ := os.ReadFile(path)
	orig := string(data)

	content := stripDriftCspBlock(orig)
	if enable {
		block := driftCspBlockStart + "\n" +
			fmt.Sprintf("[SCRIPT_%d]\nSCRIPT = '%s'\n", nextCspScriptIndex(content), scriptURL) +
			driftCspBlockEnd + "\n"
		if base := strings.TrimRight(content, "\n"); base != "" {
			content = base + "\n\n" + block
		} else {
			content = block
		}
	}

	if strings.TrimSpace(content) == "" {
		if len(data) > 0 {
			os.Remove(path)
		}
		return
	}
	if content == orig {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Print("Could not create cfg dir for csp_extra_options.ini: ", err)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Print("Could not write csp_extra_options.ini: ", err)
	}
}

// Sentinels delimiting the [EXTRA_RULES] block Server Manager owns in
// csp_extra_options.ini. Independent of the drift-score [SCRIPT_n] block — both
// can coexist; each function only rewrites/removes its own sentinel range.
const (
	rulesCspBlockStart = "; >>> Server Manager: extra rules (managed)"
	rulesCspBlockEnd   = "; <<< Server Manager: extra rules (managed)"
)

// stripRulesCspBlock removes any previously-written managed extra-rules block
// (including a leading blank-line run and trailing newline) so re-runs never
// duplicate it.
func stripRulesCspBlock(content string) string {
	re := regexp.MustCompile(`(?s)\n*` + regexp.QuoteMeta(rulesCspBlockStart) + `.*?` + regexp.QuoteMeta(rulesCspBlockEnd) + `[ \t]*\n?`)
	return re.ReplaceAllString(content, "")
}

// ensureAssettoServerExtraRules reconciles the [EXTRA_RULES] block in
// cfg/csp_extra_options.ini. When allowWrongWay is set, CSP lets drivers go the
// wrong way without the "back to pits" teleport/penalty (ALLOW_WRONG_WAY=1).
// AssettoServer encodes this file into the CSP handshake and pushes it to
// connecting clients; when disabled the managed block is stripped and, if the
// file is left empty, removed. Only meaningful on the AssettoServer engine —
// stock acServer ignores it. Coexists with the drift-score block.
func ensureAssettoServerExtraRules(dir string, allowWrongWay bool) {
	path := filepath.Join(dir, "cfg", "csp_extra_options.ini")
	data, _ := os.ReadFile(path)
	orig := string(data)

	content := stripRulesCspBlock(orig)
	if allowWrongWay {
		block := rulesCspBlockStart + "\n" +
			"[EXTRA_RULES]\nALLOW_WRONG_WAY = 1\n" +
			rulesCspBlockEnd + "\n"
		if base := strings.TrimRight(content, "\n"); base != "" {
			content = base + "\n\n" + block
		} else {
			content = block
		}
	}

	if strings.TrimSpace(content) == "" {
		if len(data) > 0 {
			os.Remove(path)
		}
		return
	}
	if content == orig {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Print("Could not create cfg dir for csp_extra_options.ini: ", err)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Print("Could not write csp_extra_options.ini: ", err)
	}
}

// unlinkIfSymlink removes path only if it is a symlink, so the Kunos extraction
// path recreates it as a real directory instead of writing through a leftover
// AssettoServer symlink into the live install tree.
func unlinkIfSymlink(path string) {
	if _, err := os.Readlink(path); err == nil {
		os.Remove(path)
	}
}

// copyFileIfNewer copies src to dst unless dst already exists with the same
// size (cheap staleness check — release artifacts are immutable per version).
func copyFileIfNewer(src, dst string) error {
	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if di, err := os.Stat(dst); err == nil && di.Size() == si.Size() {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, si.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
