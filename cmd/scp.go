package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
)

var scpCmd = &cli.Command{
	Name:            "scp",
	Usage:           "Enhanced scp command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var args []string

		proxyConfig, err := GetProxyConfig(cmd.String("proxy"))
		if err != nil {
			return err
		}
		if proxyConfig != nil {
			args = append(args, "-o", fmt.Sprintf("ProxyCommand=nc -X 5 -x %s:%s %%h %%p", proxyConfig.SocksHost, proxyConfig.SocksPort))
		}

		var extArgs = cmd.Args().Slice()

		server, err := GetSSHServerConfig(cmd.String("ssh-server"))
		if err != nil {
			return err
		}
		if server != nil {
			args = append(args, "-i", server.Prikey)
			args = append(args, "-P", server.GetPort())
			if err := ReplaceRemotePath(extArgs, *server); err != nil {
				return err
			}
		}

		return internal.ExecuteInHost(nil, "scp", append(args, extArgs...)...)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, scpCmd)
}
