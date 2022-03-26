package cmd

import (
	"fmt"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, rsyncCmd)
}

var rsyncCmd = &cli.Command{
	Name:            "rsync",
	Usage:           "Enhanced rsync workflow.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		var proxyCommand string

		proxyName := c.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return cli.Exit("proxy server not found: "+proxyName, 1)
			}
			proxyCommand = " -o ProxyCommand=\"nc -X 5 -x " + fmt.Sprintf("%s:%s", server.Host, server.Port) + " %h %p\""
		}

		var extArgs = c.Args().Slice()
		var args []string

		sshServerName := c.String("ssh-server")
		if sshServerName != "" {
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return cli.Exit("ssh server not found: "+sshServerName, 1)
			}
			args = append(args, "-e", fmt.Sprintf("ssh -i %s -p %s%s", server.Prikey, server.Port, proxyCommand))
			if err := replaceRemotePath(extArgs, server); err != nil {
				return err
			}
		}

		return ExecuteInHost(nil, "rsync", append(args, extArgs...)...)
	},
}
