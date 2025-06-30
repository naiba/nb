package cmd

import (
	"context"
	"fmt"
	"net"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/singleton"
)

var rsyncCmd = &cli.Command{
	Name:            "rsync",
	Usage:           "Enhanced rsync command.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var proxyCommand string

		proxyName := cmd.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return fmt.Errorf("proxy server not found: " + proxyName)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			proxyCommand = " -o ProxyCommand=\"nc -X 5 -x " + fmt.Sprintf("%s:%s", socksHost, socksPort) + " %h %p\""
		}

		var extArgs = cmd.Args().Slice()
		var args []string

		sshServerName := cmd.String("ssh-server")
		if sshServerName != "" {
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return fmt.Errorf("ssh server not found: " + sshServerName)
			}
			args = append(args, "-e", fmt.Sprintf("ssh -i %s -p %s%s", server.Prikey, server.GetPort(), proxyCommand))
			if err := ReplaceRemotePath(extArgs, server); err != nil {
				return err
			}
		}

		return ExecuteInHost(nil, "rsync", append(args, extArgs...)...)
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, rsyncCmd)
}
