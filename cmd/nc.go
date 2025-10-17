package cmd

import (
	"context"
	"fmt"
	"net"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
	"github.com/naiba/nb/singleton"
)

var ncCmd = &cli.Command{
	Name:            "nc",
	Usage:           "Enhanced nc command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var args []string

		proxyName := cmd.String("proxy")
		if proxyName != "" {
			if singleton.Config == nil || singleton.Config.Proxy == nil {
				return fmt.Errorf("proxy configuration not available. Please create a config file at ~/.config/nb.yaml")
			}
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return fmt.Errorf("proxy server not found: %s", proxyName)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			args = append(args, "-x", fmt.Sprintf("%s:%s", socksHost, socksPort))
		}

		return internal.ExecuteInHost(nil, "nc", append(args, cmd.Args().Slice()...)...)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, ncCmd)
}
