package cmd

import (
	"context"
	"fmt"
	"net"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
	"github.com/naiba/nb/singleton"
)

var scpCmd = &cli.Command{
	Name:            "scp",
	Usage:           "Enhanced scp command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var args []string

		proxyName := cmd.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return fmt.Errorf("proxy server not found: " + proxyName)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			args = append(args, "-o", "ProxyCommand=nc -X 5 -x "+fmt.Sprintf("%s:%s", socksHost, socksPort)+" %h %p")
		}

		var extArgs = cmd.Args().Slice()

		sshServerName := cmd.String("ssh-server")
		if sshServerName != "" {
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return fmt.Errorf("ssh server not found: " + sshServerName)
			}
			args = append(args, "-i", server.Prikey)
			args = append(args, "-P", server.GetPort())
			if err := ReplaceRemotePath(extArgs, server); err != nil {
				return err
			}
		}

		return internal.ExecuteInHost(nil, "scp", append(args, extArgs...)...)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, scpCmd)
}
