package cmd

import (
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, flutterCmd)
}

var flutterCmd = &cli.Command{
	Name:            "flutter",
	Usage:           "Enhanced flutter command.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		var args []string
		if c.Args().Len() > 0 {
			args = append(args, c.Args().Slice()[1:]...)
		}
		_, env, err := GetGitSSHCommandEnv(c.String("git-user"), c.String("proxy"))
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}
		return ExecuteInHost(env, "flutter", args...)
	},
}
