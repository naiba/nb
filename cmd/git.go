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
		gitCleanHistoryCommand,
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

var getGitDirectoryName = regexp.MustCompile(`(?:^|[/:])([^/:]+?)(?:\.git)?$`)

var gitSalonCommand = &cli.Command{
	Name:            "salon",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		args := []string{"clone"}
		_, env, err := GetGitSSHCommandEnv(cmd.String("git-user"), cmd.String("proxy"))
		if err != nil {
			return err
		}

		cmdArgs := cmd.Args().Slice()
		args = append(args, cmdArgs...)

		var dirName string
		var nonFlagArgs []string
		for _, arg := range cmdArgs {
			if !strings.HasPrefix(arg, "-") {
				nonFlagArgs = append(nonFlagArgs, arg)
			}
		}
		if len(nonFlagArgs) >= 2 {
			dirName = nonFlagArgs[len(nonFlagArgs)-1]
		} else if len(nonFlagArgs) == 1 {
			matched := getGitDirectoryName.FindStringSubmatch(nonFlagArgs[0])
			if len(matched) < 2 {
				return fmt.Errorf("failed to parse git directory name from %s", nonFlagArgs[0])
			}
			dirName = matched[1]
		} else {
			return fmt.Errorf("missing git repository URL")
		}

		if err := internal.ExecuteInHost(env, "git", args...); err != nil {
			return err
		}

		setupArgs := []string{"cd", dirName, "&&", "nb"}
		if cmd.String("proxy") != "" {
			setupArgs = append(setupArgs, "-p "+cmd.String("proxy"))
		}
		setupArgs = append(setupArgs, "-gu "+cmd.String("git-user"))
		setupArgs = append(setupArgs, "git setup")

		return internal.BashScriptExecuteInHost(strings.Join(setupArgs, " "))
	},
}

var gitWhoCommand = &cli.Command{
	Name: "whoami",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		return internal.BashScriptExecuteInHost("git config --local --list|grep \"user.email\\|user.name\\|core.sshcommand\\|gpg.format\\|user.signingkey\"")
	},
}

var gitCleanHistoryCommand = &cli.Command{
	Name:  "clean-history",
	Usage: "Clean all commit history, keeping only current files.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "message",
			Aliases: []string{"m"},
			Value:   "Initial commit",
			Usage:   "Commit message for the new initial commit",
		},
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "Skip confirmation prompt",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if !cmd.Bool("force") {
			fmt.Print("WARNING: This will permanently delete all commit history. Continue? [y/N]: ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		// Get current branch name
		branchBytes, err := internal.ExecuteInHostWithOutput(nil, "git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branch := strings.TrimSpace(string(branchBytes))

		message := cmd.String("message")
		script := fmt.Sprintf(`
git checkout --orphan temp_clean_history_branch && \
git add -A && \
git commit -m "%s" && \
git branch -D %s && \
git branch -m %s
`, message, branch, branch)

		if err := internal.BashScriptExecuteInHost(script); err != nil {
			return fmt.Errorf("failed to clean history: %w", err)
		}

		fmt.Println("Successfully cleaned commit history.")
		fmt.Println("To push changes, run: git push -f origin " + branch)
		return nil
	},
}
