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

func (inst *Instance) start() {
	binary := "acServer"
	if runtime.GOOS == "windows" {
		binary = "acServer.exe"
	}

	dir := inst.Dir()
	fpath := filepath.Join(dir, binary)
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
	defer inst.mu.Unlock()

	if inst.cmd != nil && inst.cmd.Process != nil {
		err := inst.cmd.Process.Kill()

		if err != nil {
			log.Print("Error killing acServer process: ", err)
		}
		inst.cmd = nil
		inst.Udp.online = false
		inst.Status.Players = 0
	}
}
