package internal

import (
	"os"
	"os/exec"
	"strings"
)

func ExecuteInHost(env []string, name string, args ...string) error {
	command := BuildCommand(env, name, args...)
	return command.Run()
}

func ExecuteInHostWithOutput(env []string, name string, args ...string) ([]byte, error) {
	cmdStr := name + " " + strings.Join(args, " ")
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Env = append(os.Environ(), env...)
	return cmd.Output()
}

func BashScriptExecuteInHost(line string) error {
	command := BuildCommand(nil, "bash", "-c", line)
	return command.Run()
}
