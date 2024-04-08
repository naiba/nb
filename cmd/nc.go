package cmd

import (
	"fmt"
	"net"

	"github.com/urfave/cli/v2"

	"github.com/naiba/nb/singleton"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, ncCmd)
}

var ncCmd = &cli.Command{
	Name:            "nc",
	Usage:           "Enhanced nc command.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		var args []string

		proxyName := c.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return cli.Exit("proxy server not found: "+proxyName, 1)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			args = append(args, "-x", fmt.Sprintf("%s:%s", socksHost, socksPort))
		}

		return ExecuteInHost(nil, "nc", append(args, c.Args().Slice()...)...)
	},
}
