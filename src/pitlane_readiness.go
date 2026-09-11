package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Protect on-disk provisioning and launch from asynchronous content replacement.
var contentLifecycleMu sync.RWMutex

func validateDrivingInstallation(s DrivingSession, inst *Instance) error {
	cfg, err := Dba.selectConfig()
	if err != nil {
		return err
	}
	if !isEnabled(cfg.CfgFilled) || !isEnabled(cfg.ModFilled) {
		return errors.New("Finish installation and server configuration in Garage before planning a start")
	}
	engine := derefOrEmpty(cfg.ServerEngine)
	if engine == "" {
		engine = engineKunos
	}
	if engine != engineKunos && engine != engineAssettoServer {
		return errors.New("Select a supported server engine in Garage")
	}
	if s.Experience == "drift" && engine != engineAssettoServer {
		return errors.New("Drift scoring needs AssettoServer. Choose that engine in Garage, or create a Practice session")
	}
	if !isEnabled(inst.Conf.Enabled) {
		return errors.New("Enable this server before starting a session")
	}
	for _, p := range []*int{inst.Conf.UdpPort, inst.Conf.TcpPort, inst.Conf.HttpPort, inst.Conf.PluginPort, inst.Conf.PluginListenPort} {
		if p == nil || *p < 1 || *p > 65535 {
			return errors.New("Configure valid game, HTTP and telemetry ports for this server")
		}
		for _, other := range Instances.All() {
			if other.Id() == inst.Id() || !isEnabled(other.Conf.Enabled) {
				continue
			}
			for _, q := range []*int{other.Conf.UdpPort, other.Conf.TcpPort, other.Conf.HttpPort, other.Conf.PluginPort, other.Conf.PluginListenPort} {
				if q != nil && *p == *q {
					return fmt.Errorf("Port %d is also used by %s. Resolve the conflict in Your servers", *p, other.Name())
				}
			}
		}
	}
	if engine == engineAssettoServer {
		if !assettoServerInstalled() {
			return errors.New("AssettoServer is not installed. Use Garage → Advanced → Server engine to install it")
		}
		base, err := Dba.basepath()
		if err != nil {
			return err
		}
		paths := []string{filepath.Join(base, "content", "tracks", s.Setup.TrackKey)}
		for _, g := range s.Setup.Grid.Entries {
			paths = append(paths, filepath.Join(base, "content", "cars", derefOrEmpty(g.CacheCarKey)), filepath.Join(base, "content", "cars", derefOrEmpty(g.CacheCarKey), "skins", derefOrEmpty(g.SkinKey)))
		}
		if s.Setup.TrackConfig != "" {
			paths = append(paths, filepath.Join(base, "content", "tracks", s.Setup.TrackKey, s.Setup.TrackConfig))
		}
		for _, p := range paths {
			if st, e := os.Stat(p); e != nil || !st.IsDir() {
				return errors.New("Installed content changed on disk. Rescan Cars & tracks and review the selected layout, cars and skins")
			}
		}
		return nil
	}
	archive, err := zip.OpenReader(filepath.Join(ConfigFolder, "smcontent.zip"))
	if err != nil {
		return errors.New("The server content archive is missing or unreadable. Rebuild the content cache in Garage")
	}
	defer archive.Close()
	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
	}
	binary := "acServer"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if !names[binary] {
		return errors.New("The server binary is missing from the content archive. Check installation and rebuild the cache")
	}
	models := "models.ini"
	if s.Setup.TrackConfig != "" {
		models = "models_" + s.Setup.TrackConfig + ".ini"
	}
	if !names["tracks/"+s.Setup.TrackKey+"/"+models] {
		return errors.New("The exact track layout is missing from the server content archive. Rebuild the cache")
	}
	for _, g := range s.Setup.Grid.Entries {
		found := false
		for name := range names {
			if strings.HasPrefix(name, "cars/"+derefOrEmpty(g.CacheCarKey)+"/") {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Car %s is missing from the server content archive. Rebuild the cache", derefOrEmpty(g.CacheCarKey))
		}
	}
	return nil
}
