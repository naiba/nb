package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"

	"github.com/naiba/nb/internal"
	"github.com/naiba/nb/model"
	"github.com/naiba/nb/singleton"
)

var sshCmd = &cli.Command{
	Name:     "ssh",
	Usage:    "Enhanced ssh command.",
	Commands: []*cli.Command{sshInsecureCmd},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		var args []string

		proxyName := cmd.String("proxy")
		if proxyName != "" {
			if singleton.Config == nil || singleton.Config.Proxy == nil {
				return fmt.Errorf("proxy configuration not available. Please create a config file at ~/.config/nb.yaml")
			}
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return fmt.Errorf("proxy server not found: %s", proxyName)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			args = append(args, "-o", "ProxyCommand=nc -X 5 -x "+fmt.Sprintf("%s:%s", socksHost, socksPort)+" %h %p")
		}

		sshServerName := cmd.String("ssh-server")
		if sshServerName != "" {
			if singleton.Config == nil || singleton.Config.SSH == nil {
				return fmt.Errorf("SSH configuration not available. Please create a config file at ~/.config/nb.yaml")
			}
			server, exists := singleton.Config.SSH[sshServerName]
			if !exists {
				return fmt.Errorf("ssh server not found: %s", sshServerName)
			}
			args = append(args, "-i", server.Prikey)
			args = append(args, "-p", server.GetPort())
			args = append(args, server.Login+"@"+server.Host)
		}

		return internal.ExecuteInHost(nil, "ssh", append(args, cmd.Args().Slice()...)...)
	},
}

var sshInsecureCmd = &cli.Command{
	Name:    "insecure",
	Aliases: []string{"in"},
	Usage:   "Scan insecure ssh server.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if singleton.Config == nil || singleton.Config.SSH == nil || len(singleton.Config.SSH) == 0 {
			return fmt.Errorf("SSH configuration not available. Please create a config file at ~/.config/nb.yaml")
		}
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

func init() {
	rootCmd.Commands = append(rootCmd.Commands, sshCmd)
}
