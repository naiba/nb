package cmd

import (
	"fmt"
	"strings"

	"github.com/naiba/nb/model"
	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, scpCmd)
}

var scpCmd = &cli.Command{
	Name:  "scp",
	Usage: "Enhanced scp workflow.",
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
			if err := replaceRemotePath(extArgs, server); err != nil {
				return err
			}
		}

		return ExecuteInHost(nil, "scp", append(args, extArgs...)...)
	},
}

func replaceRemotePath(slice []string, server model.SSHAccount) error {
	var replaced bool
	for i := 0; i < len(slice); i++ {
		if strings.HasPrefix(slice[i], "remote:") {
			slice[i] = strings.Replace(slice[i], "remote:", fmt.Sprintf("%s@%s:", server.Login, server.Host), 1)
			replaced = true
		}
	}
	if !replaced {
		return fmt.Errorf("Remote path (remote:) not found in args: %v", slice)
	}
	return nil
}
