package cmd

import (
	"fmt"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, scpCmd)
}

var scpCmd = &cli.Command{
	Name:            "scp",
	Usage:           "Enhanced scp command.",
	SkipFlagParsing: true,
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

		var extArgs = c.Args().Slice()

		sshServerName := c.String("ssh-server")
		if sshServerName != "" {
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return cli.Exit("ssh server not found: "+sshServerName, 1)
			}
			args = append(args, "-i", server.Prikey)
			args = append(args, "-P", server.GetPort())
			if err := ReplaceRemotePath(extArgs, server); err != nil {
				return err
			}
		}

		return ExecuteInHost(nil, "scp", append(args, extArgs...)...)
	},
}
