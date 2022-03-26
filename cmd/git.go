package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/naiba/nb/model"
	"github.com/naiba/nb/singleton"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, gitCmd)
}

var gitCmd = &cli.Command{
	Name:  "git",
	Usage: "Enhanced git workflow.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
			Usage:   "Choose a git account to execute commands.",
		},
	},
	Subcommands: []*cli.Command{
		gitCommitCommand,
		gitWhoCommand,
		gitSetupCommand,
	},
	Action: func(c *cli.Context) error {
		_, env, err := getGitEnvForUser(c)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		return ExecuteInHost(env, "git", c.Args().Slice()...)
	},
}

var gitCommitCommand = &cli.Command{
	Name: "commit",
	Action: func(c *cli.Context) error {
		account, env, err := getGitEnvForUser(c)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		args := []string{"commit"}
		if account != nil {
			args = append(args, "--author=\""+account.Name+" <"+account.Email+">\"")
		}
		args = append(args, c.Args().Slice()...)
		return ExecuteInHost(env, "git", args...)
	},
}

var gitWhoCommand = &cli.Command{
	Name: "whoami",
	Action: func(c *cli.Context) error {
		return ExecuteLineInHost("git config --local --list|grep \"user.email\\|user.name\\|core.sshcommand\"")
	},
}

var gitSetupCommand = &cli.Command{
	Name:  "setup",
	Usage: "Setup or tear-down the git account config locally.",
	Action: func(c *cli.Context) error {
		account, env, err := getGitEnvForUser(c)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		if account == nil {
			if err := ExecuteLineInHost("git config --local --unset core.sshCommand && git config --local --unset user.name && git config --local --unset user.email"); err != nil {
				return err
			}
		} else {
			_, command, _ := strings.Cut(env[0], "=")
			if err := ExecuteLineInHost("git config --local core.sshCommand '" + command + "' && git config --local user.name " + account.Name + " && git config --local user.email " + account.Email); err != nil {
				return err
			}
		}
		return gitWhoCommand.Action(c)
	},
}

func getGitEnvForUser(c *cli.Context) (*model.GitAccount, []string, error) {
	user := c.String("user")
	if user == "" {
		return nil, nil, nil
	}
	account, exists := singleton.Config.Git[user]
	if !exists {
		return nil, nil, errors.New("git user not exists: " + user)
	}
	proxyName := c.String("proxy")
	if proxyName == "" {
		return &account, []string{
			"GIT_SSH_COMMAND=ssh -i \"" + account.SSHPrikey + "\" -o IdentitiesOnly=yes",
		}, nil
	}
	server, exists := singleton.Config.Proxy[proxyName]
	if !exists {
		return nil, nil, errors.New("proxy server not found: " + proxyName)
	}
	return &account, []string{
		"GIT_SSH_COMMAND=ssh -i \"" + account.SSHPrikey + "\" -o ProxyCommand=\"nc -X 5 -x " + fmt.Sprintf("%s:%s", server.Host, server.Port) + " %h %p\" -o IdentitiesOnly=yes",
	}, nil
}
