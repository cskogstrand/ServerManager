package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (inst *Instance) appendOutput(pipe io.ReadCloser, label string) {
	out := bufio.NewReader(pipe)

	for {
		b, _, err := out.ReadLine()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("Error acServer process %s: %v", label, err)
			}
			return
		}

		if len(b) > 0 {
			inst.mu.Lock()
			inst.lines += string(b) + "\n"
			inst.mu.Unlock()
		}
	}
}

// serverExecutable returns the path to the dedicated-server binary to spawn for
// this instance, honouring the configured engine. The AssettoServer binary is
// copied into the run dir by provisionAssettoServerRunDir so it sits next to
// its native libs and the cfg/content trees.
func (inst *Instance) serverExecutable(dir string) string {
	if cfg, err := Dba.selectConfig(); err == nil &&
		cfg.ServerEngine != nil && *cfg.ServerEngine == engineAssettoServer {
		return filepath.Join(dir, assettoServerBinaryName())
	}
	binary := "acServer"
	if runtime.GOOS == "windows" {
		binary = "acServer.exe"
	}
	return filepath.Join(dir, binary)
}

func (inst *Instance) start() {
	dir := inst.Dir()
	fpath := inst.serverExecutable(dir)
	if _, err := os.Stat(fpath); errors.Is(err, os.ErrNotExist) {
		log.Print("Could not find executable: ", fpath, err)
	}

	if runtime.GOOS != "windows" {
		err := os.Chmod(fpath, 0755)

		if err != nil {
			log.Print("Could not chmod executable: ", fpath, err)
		}
	}

	inst.mu.Lock()
	if inst.cmd != nil {
		inst.mu.Unlock()
		log.Print("Instance already running: ", inst.Name())
		return
	}

	cmd := exec.Command(fpath)
	cmd.Dir = dir
	inst.Udp.online = false
	inst.tel = telemetryHealth{}
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		log.Print("Could not capture acServer stdout: ", err)
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		log.Print("Could not capture acServer stderr: ", err)
	}
	err = cmd.Start()
	if err != nil {
		inst.mu.Unlock()
		log.Print("Could not start executable: ", fpath, err)
		return
	}

	inst.cmd = cmd
	inst.lines = ""
	inst.mu.Unlock()

	go inst.appendOutput(stdOut, "stdout")
	go inst.appendOutput(stdErr, "stderr")
	go inst.waitForExit(cmd)

	inst.publishRunning(true)
}

// waitForExit reaps the server process. Without this the OS process becomes a
// zombie on exit and, worse, the UI keeps showing "running" after a crash
// because inst.cmd is never cleared. When the process we started exits, we
// clear runtime state and publish running=false — unless stop() already swapped
// inst.cmd (a deliberate stop/restart), in which case it owns the cleanup.
func (inst *Instance) waitForExit(cmd *exec.Cmd) {
	err := cmd.Wait()

	inst.mu.Lock()
	if inst.cmd != cmd {
		// A stop() or restart already replaced/cleared this process; that path
		// handles its own cleanup.
		inst.mu.Unlock()
		return
	}
	inst.cmd = nil
	inst.Udp.online = false
	inst.tel.udpOnline = false
	inst.Status.Players = 0
	inst.mu.Unlock()

	if err != nil {
		log.Printf("acServer process for %q exited: %v", inst.Name(), err)
	} else {
		log.Printf("acServer process for %q exited", inst.Name())
	}

	inst.clearDrivers()
	inst.publishRunning(false)
}

func (inst *Instance) logContent() string {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.lines
}

func (inst *Instance) isRunning() bool {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.cmd != nil
}

func (inst *Instance) stop() {
	inst.mu.Lock()

	stopped := false
	if inst.cmd != nil && inst.cmd.Process != nil {
		err := inst.cmd.Process.Kill()

		if err != nil {
			log.Print("Error killing acServer process: ", err)
		}
		inst.cmd = nil
		inst.Udp.online = false
		inst.Status.Players = 0
		stopped = true
	}
	inst.mu.Unlock()

	if stopped {
		inst.clearDrivers()
		inst.publishRunning(false)
	}
}
