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

var lines string
var cmd *exec.Cmd

func appendOutput(pipe io.ReadCloser, label string) {
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
			lines += string(b) + "\n"
		}
	}
}

func start() {
	binary := "acServer"
	if runtime.GOOS == "windows" {
		binary = "acServer.exe"
	}

	fpath := filepath.Join(TempFolder, binary)
	if _, err := os.Stat(fpath); errors.Is(err, os.ErrNotExist) {
		log.Print("Could not find executable: ", fpath, err)
	}
	cmd = exec.Command(fpath)

	if runtime.GOOS != "windows" {
		err := os.Chmod(fpath, 0755)

		if err != nil {
			log.Print("Could not chmod executable: ", fpath, err)
		}
	}

	cmd.Dir = TempFolder
	Udp.online = false
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
		log.Print("Could not start executable: ", fpath, err)
		return
	}

	lines = ""
	go appendOutput(stdOut, "stdout")
	go appendOutput(stdErr, "stderr")
}

func getContent() string {
	return lines
}

func isRunning() bool {
	return cmd != nil
}

func stop() {
	if cmd != nil && cmd.Process != nil {
		err := cmd.Process.Kill()

		if err != nil {
			log.Print("Error killing acServer process: ", err)
		}
		cmd = nil
		Udp.online = false
		Status.Players = 0
	}
}
