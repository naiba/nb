package cmd

import (
	"context"
	"fmt"
	"log"
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

		proxyConfig, err := GetProxyConfig(cmd.String("proxy"))
		if err != nil {
			return err
		}
		if proxyConfig != nil {
			args = append(args, "-o", fmt.Sprintf("ProxyCommand=nc -X 5 -x %s:%s %%h %%p", proxyConfig.SocksHost, proxyConfig.SocksPort))
		}

		server, err := GetSSHServerConfig(cmd.String("ssh-server"))
		if err != nil {
			return err
		}
		if server != nil {
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
	Usage:   "Scan SSH servers for password authentication vulnerability (tests if server accepts password auth).",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if singleton.Config == nil || singleton.Config.SSH == nil || len(singleton.Config.SSH) == 0 {
			return fmt.Errorf("SSH configuration not available. Please create a config file at ~/.config/nb.yaml")
		}
		var wg sync.WaitGroup
		wg.Add(len(singleton.Config.SSH))
		for _, item := range singleton.Config.SSH {
			go func(s model.SSHAccount) {
				defer wg.Done()
				// This config tests if the server accepts password authentication.
				// If it does (even with wrong password), the server is considered insecure
				// because it should only allow key-based authentication.
				config := &ssh.ClientConfig{
					User: s.Login,
					Auth: []ssh.AuthMethod{
						// Empty password to test if password auth is enabled
						ssh.Password(""),
					},
					// WARNING: Host key verification is intentionally disabled for this security scan.
					// This allows testing servers without prior known_hosts entries.
					HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				}
				addr := fmt.Sprintf("%s:%s", s.Host, s.GetPort())
				conn, err := ssh.Dial("tcp", addr, config)
				if err == nil {
					conn.Close()
					log.Println("SSH Server:", addr, "is insecure (accepts empty password)")
				} else if !strings.Contains(err.Error(), "attempted methods [none]") {
					// Server allows password auth attempts (insecure - should only allow key auth)
					log.Println("SSH Server:", addr, "allows password auth:", err.Error())
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
