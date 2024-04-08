package cmd

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"

	"github.com/naiba/nb/model"
	"github.com/naiba/nb/singleton"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, sshCmd)
}

var sshCmd = &cli.Command{
	Name:        "ssh",
	Usage:       "Enhanced ssh command.",
	Subcommands: []*cli.Command{sshInsecureCmd},
	Action: func(c *cli.Context) error {
		var args []string

		proxyName := c.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return cli.Exit("proxy server not found: "+proxyName, 1)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			args = append(args, "-o", "ProxyCommand=nc -X 5 -x "+fmt.Sprintf("%s:%s", socksHost, socksPort)+" %h %p")
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

var sshInsecureCmd = &cli.Command{
	Name:    "insecure",
	Aliases: []string{"in"},
	Usage:   "Scan insecure ssh server.",
	Action: func(c *cli.Context) error {
		var wg sync.WaitGroup
		wg.Add(len(singleton.Config.SSH))
		for _, item := range singleton.Config.SSH {
			go func(s model.SSHAccount) {
				defer wg.Done()
				config := &ssh.ClientConfig{
					User: s.Login,
					Auth: []ssh.AuthMethod{
						ssh.Password("your_password"),
					},
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				}
				addr := fmt.Sprintf("%s:%s", s.Host, s.GetPort())
				conn, err := ssh.Dial("tcp", addr, config)
				if err == nil {
					conn.Close()
					log.Println("SSH Server:", addr, "is insecure")
				} else if !strings.Contains(err.Error(), "attempted methods [none]") {
					log.Println("SSH Server:", addr, "is insecure, ", err.Error())
				}
			}(item)
		}
		wg.Wait()
		return nil
	},
}
