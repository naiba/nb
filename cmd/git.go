package cmd

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, gitCmd)
}

var gitCmd = &cli.Command{
	Name:  "git",
	Usage: "Enhanced git command.",
	Commands: []*cli.Command{
		gitCommitCommand,
		gitWhoCommand,
		gitSetupCommand,
		gitSalonCommand,
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		_, env, err := GetGitSSHCommandEnv(cmd.String("git-user"), cmd.String("proxy"))
		if err != nil {
			return err
		}
		return internal.ExecuteInHost(env, "git", cmd.Args().Slice()...)
	},
}

var gitSetupCommand = &cli.Command{
	Name:  "setup",
	Usage: "Setup or tear-down the git account config locally.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		account, env, err := GetGitSSHCommandEnv(cmd.String("git-user"), cmd.String("proxy"))
		if err != nil {
			return err
		}
		if account == nil {
			if err := internal.BashScriptExecuteInHost("git config --local --unset core.sshCommand && git config --local --unset user.name && git config --local --unset user.email"); err != nil {
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
			if err := internal.BashScriptExecuteInHost(command); err != nil {
				return err
			}
		}
		return gitWhoCommand.Action(ctx, cmd)
	},
}

var gitCommitCommand = &cli.Command{
	Name:            "commit",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		account, env, err := GetGitSSHCommandEnv(cmd.String("git-user"), cmd.String("proxy"))
		if err != nil {
			return err
		}
		args := []string{"commit"}
		if account != nil {
			args = append(args, "--author=\""+account.Name+" <"+account.Email+">\"")
		}
		args = append(args, cmd.Args().Slice()...)
		return internal.ExecuteInHost(env, "git", args...)
	},
}

var getGitDirectoryName = regexp.MustCompile(`\/([^\/]*)\.git`)

var gitSalonCommand = &cli.Command{
	Name:            "salon",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		args := []string{"clone"}
		_, env, err := GetGitSSHCommandEnv(cmd.String("git-user"), cmd.String("proxy"))
		if err != nil {
			return err
		}
		args = append(args, cmd.Args().Slice()...)

		matched := getGitDirectoryName.FindAllStringSubmatch(strings.Join(args, " "), 1)
		if len(matched[0]) != 2 {
			return fmt.Errorf("failed to parse git directory name from %s", args)
		}
		if err := internal.ExecuteInHost(env, "git", args...); err != nil {
			return err
		}

		args = []string{"cd", matched[0][1], "&&", "nb"}
		if cmd.String("proxy") != "" {
			args = append(args, "-p "+cmd.String("proxy"))
		}
		args = append(args, "-gu "+cmd.String("git-user"))
		args = append(args, "git setup")

		return internal.BashScriptExecuteInHost(strings.Join(args, " "))
	},
}

var gitWhoCommand = &cli.Command{
	Name: "whoami",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return internal.BashScriptExecuteInHost("git config --local --list|grep \"user.email\\|user.name\\|core.sshcommand\\|gpg.format\\|user.signingkey\"")
	},
}
