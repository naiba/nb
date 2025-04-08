//go:build !windows
// +build !windows

package internal

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

var pid, gid int

func init() {
	var err error
	pid = os.Getpid()
	gid, err = syscall.Getpgid(pid)
	if err != nil {
		log.Fatalf("Failed to get process group ID: %v", err)
	}
}

func BuildCommand(env []string, name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: gid}
	command.Env = append(command.Env, os.Environ()...)
	command.Env = append(command.Env, env...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command
}

func CleanupChildProcesses(bySignal bool) {
	if pid == gid {
		syscall.Kill(-gid, syscall.SIGTERM)
	}
}
