package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

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
