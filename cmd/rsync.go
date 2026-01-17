package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
)

var rsyncCmd = &cli.Command{
	Name:            "rsync",
	Usage:           "Enhanced rsync command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var proxyCommand string

		proxyConfig, err := GetProxyConfig(cmd.String("proxy"))
		if err != nil {
			return err
		}
		if proxyConfig != nil {
			proxyCommand = fmt.Sprintf(" -o ProxyCommand=\"nc -X 5 -x %s:%s %%h %%p\"", proxyConfig.SocksHost, proxyConfig.SocksPort)
		}

		var extArgs = cmd.Args().Slice()
		var args []string

		server, err := GetSSHServerConfig(cmd.String("ssh-server"))
		if err != nil {
			return err
		}
		if server != nil {
			args = append(args, "-e", fmt.Sprintf("ssh -i %s -p %s%s", server.Prikey, server.GetPort(), proxyCommand))
			if err := ReplaceRemotePath(extArgs, *server); err != nil {
				return err
			}
		}

		return internal.ExecuteInHost(nil, "rsync", append(args, extArgs...)...)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, rsyncCmd)
}
