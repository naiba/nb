package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
