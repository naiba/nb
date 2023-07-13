package cmd

import (
	"fmt"
	"net"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, rsyncCmd)
}

var rsyncCmd = &cli.Command{
	Name:            "rsync",
	Usage:           "Enhanced rsync command.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		var proxyCommand string

		proxyName := c.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return cli.Exit("proxy server not found: "+proxyName, 1)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			proxyCommand = " -o ProxyCommand=\"nc -X 5 -x " + fmt.Sprintf("%s:%s", socksHost, socksPort) + " %h %p\""
		}

		var extArgs = c.Args().Slice()
		var args []string

		sshServerName := c.String("ssh-server")
		if sshServerName != "" {
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return cli.Exit("ssh server not found: "+sshServerName, 1)
			}
			args = append(args, "-e", fmt.Sprintf("ssh -i %s -p %s%s", server.Prikey, server.GetPort(), proxyCommand))
			if err := ReplaceRemotePath(extArgs, server); err != nil {
				return err
			}
		}

		return ExecuteInHost(nil, "rsync", append(args, extArgs...)...)
	},
}
