package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
		gitCoauthoredByCommand,
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
		return ExecuteLineInHost("git config --local --list|grep \"user.email\\|user.name\\|core.sshcommand\"")
	},
}

var gitCoauthoredByCommand = &cli.Command{
	Name:    "co-authored-by",
	Aliases: []string{"cab"},
	Action: func(c *cli.Context) error {
		users := c.Args().Slice()
		for i := 0; i < len(users); i++ {
			resp, err := http.Get("https://api.github.com/users/" + users[i])
			if err != nil {
				panic(err)
			}
			body, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				panic(err)
			}
			var u GitHubUserInfoResponse
			err = json.Unmarshal(body, &u)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Co-authored-by: %s <%d+%s@users.noreply.github.com>\n", u.Login, u.ID, u.Login)
		}
		return nil
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
			if err := ExecuteLineInHost("git config --local core.sshCommand '" + command + "' && git config --local user.name " + account.Name + " && git config --local user.email " + account.Email); err != nil {
				return err
			}
		}
		return gitWhoCommand.Action(c)
	},
}

type GitHubUserInfoResponse struct {
	ID      uint64
	Login   string
	Blog    *string
	Name    *string
	Company *string
	Email   *string
}
