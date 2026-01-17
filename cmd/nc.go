package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
)

var ncCmd = &cli.Command{
	Name:            "nc",
	Usage:           "Enhanced nc command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var args []string

		proxyConfig, err := GetProxyConfig(cmd.String("proxy"))
		if err != nil {
			return err
		}
		if proxyConfig != nil {
			args = append(args, "-x", fmt.Sprintf("%s:%s", proxyConfig.SocksHost, proxyConfig.SocksPort))
		}

		return internal.ExecuteInHost(nil, "nc", append(args, cmd.Args().Slice()...)...)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, ncCmd)
}
