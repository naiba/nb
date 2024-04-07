package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, gitCmd)
}

var gitCmd = &cli.Command{
	Name:            "git",
	Usage:           "Enhanced git command.",
	SkipFlagParsing: true,
	Subcommands: []*cli.Command{
		gitCommitCommand,
		gitWhoCommand,
		gitSetupCommand,
		gitSalonCommand,
	},
	Action: func(c *cli.Context) error {
		_, env, err := GetGitSSHCommandEnv(c.String("git-user"), c.String("proxy"))
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		return ExecuteInHost(env, "git", c.Args().Slice()...)
	},
}

var gitSetupCommand = &cli.Command{
	Name:  "setup",
	Usage: "Setup or tear-down the git account config locally.",
	Action: func(c *cli.Context) error {
		account, env, err := GetGitSSHCommandEnv(c.String("git-user"), c.String("proxy"))
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		if account == nil {
			if err := ExecuteLineInHost("git config --local --unset core.sshCommand && git config --local --unset user.name && git config --local --unset user.email"); err != nil {
				return err
			}
		} else {
			_, command, _ := strings.Cut(env[0], "=")
			command = "git config --local core.sshCommand '" + command +
				"' && git config --local user.name " + account.Name +
				" && git config --local user.email " + account.Email
			if account.SSHSignKey != "" {
				command += " && git config --local gpg.format ssh && git config --local user.signingkey " + account.SSHSignKey
			}
			if err := ExecuteLineInHost(command); err != nil {
				return err
			}
		}
		return gitWhoCommand.Action(c)
	},
}

var gitCommitCommand = &cli.Command{
	Name:            "commit",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		account, env, err := GetGitSSHCommandEnv(c.String("git-user"), c.String("proxy"))
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

var getGitDirectoryName = regexp.MustCompile(`\/([^\/]*)\.git`)

var gitSalonCommand = &cli.Command{
	Name:            "salon",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		args := []string{"clone"}
		_, env, err := GetGitSSHCommandEnv(c.String("git-user"), c.String("proxy"))
		if err != nil {
			return err
		}
		args = append(args, c.Args().Slice()...)

		matched := getGitDirectoryName.FindAllStringSubmatch(strings.Join(args, " "), 1)
		if len(matched[0]) != 2 {
			return fmt.Errorf("failed to parse git directory name from %s", args)
		}
		if err := ExecuteInHost(env, "git", args...); err != nil {
			return err
		}

		args = []string{"cd", matched[0][1], "&&", "nb"}
		if c.String("proxy") != "" {
			args = append(args, "-p "+c.String("proxy"))
		}
		args = append(args, "-gu "+c.String("git-user"))
		args = append(args, "git setup")

		return ExecuteLineInHost(strings.Join(args, " "))
	},
}

var gitWhoCommand = &cli.Command{
	Name: "whoami",
	Action: func(c *cli.Context) error {
		return ExecuteLineInHost("git config --local --list|grep \"user.email\\|user.name\\|core.sshcommand\\|gpg.format\\|user.signingkey\"")
	},
}
