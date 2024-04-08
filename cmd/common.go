package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/naiba/nb/model"
	"github.com/naiba/nb/singleton"
)

func ExecuteInHost(env []string, name string, args ...string) error {
	command := BuildCommand(env, name, args...)
	return command.Run()
}

func BashScriptExecuteInHost(line string) error {
	command := BuildCommand(nil, "bash", "-c", line)
	return command.Run()
}

func BuildCommand(env []string, name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)
	command.Env = append(command.Env, os.Environ()...)
	command.Env = append(command.Env, env...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command
}

func GetGitSSHCommandEnv(user string, proxyName string) (*model.GitAccount, []string, error) {
	if user == "" {
		return nil, nil, nil
	}
	account, exists := singleton.Config.Git[user]
	if !exists {
		return nil, nil, errors.New("git user not exists: " + user)
	}
	if proxyName == "" {
		return &account, []string{
			"GIT_SSH_COMMAND=ssh -i \"" + account.SSHPrikey + "\" -o IdentitiesOnly=yes",
		}, nil
	}
	server, exists := singleton.Config.Proxy[proxyName]
	if !exists {
		return nil, nil, errors.New("proxy server not found: " + proxyName)
	}
	socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
	return &account, []string{
		"GIT_SSH_COMMAND=ssh -i \"" + account.SSHPrikey + "\" -o ProxyCommand=\"nc -X 5 -x " + fmt.Sprintf("%s:%s", socksHost, socksPort) + " %h %p\" -o IdentitiesOnly=yes",
	}, nil
}

func ReplaceRemotePath(slice []string, server model.SSHAccount) error {
	var replaced bool
	for i := 0; i < len(slice); i++ {
		if strings.HasPrefix(slice[i], "remote:") {
			slice[i] = strings.Replace(slice[i], "remote:", fmt.Sprintf("%s@%s:", server.Login, server.Host), 1)
			replaced = true
		}
	}
	if !replaced {
		return fmt.Errorf("remote path (remote:) not found in args: %v", slice)
	}
	return nil
}
