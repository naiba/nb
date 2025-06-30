package cmd

import (
	"context"

	"github.com/urfave/cli/v3"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, flutterCmd)
}

var flutterCmd = &cli.Command{
	Name:            "flutter",
	Usage:           "Enhanced flutter command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		_, env, err := GetGitSSHCommandEnv(cmd.String("git-user"), cmd.String("proxy"))
		if err != nil {
			return err
		}
		return ExecuteInHost(env, "flutter", cmd.Args().Slice()...)
	},
}
