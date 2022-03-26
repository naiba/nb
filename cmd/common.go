package cmd

import (
	"os"
	"os/exec"
)

func ExecuteInHost(env []string, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Env = append(command.Env, os.Environ()...)
	command.Env = append(command.Env, env...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func ExecuteLineInHost(line string) error {
	command := exec.Command("bash", "-c", line)
	command.Env = append(command.Env, os.Environ()...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
