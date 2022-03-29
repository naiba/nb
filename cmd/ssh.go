package cmd

import (
	"fmt"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, sshCmd)
}

var sshCmd = &cli.Command{
	Name:  "ssh",
	Usage: "Enhanced ssh command.",
	Action: func(c *cli.Context) error {
		var args []string

		proxyName := c.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return cli.Exit("proxy server not found: "+proxyName, 1)
			}
			args = append(args, "-o", "ProxyCommand=nc -X 5 -x "+fmt.Sprintf("%s:%s", server.Host, server.Port)+" %h %p")
		}

		sshServerName := c.String("ssh-server")
		if sshServerName != "" {
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return cli.Exit("ssh server not found: "+sshServerName, 1)
			}
			args = append(args, "-i", server.Prikey)
			args = append(args, "-p", server.GetPort())
			args = append(args, server.Login+"@"+server.Host)
		}

		return ExecuteInHost(nil, "ssh", append(args, c.Args().Slice()...)...)
	},
}
