package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/naiba/nb/singleton"
)

var version = "1.0.0"

var rootCmd = &cli.App{
	Name:        "nb",
	Usage:       "Nb is not only no bullshit.",
	Description: "Author: naiba https://github.com/naiba",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "proxy",
			Aliases: []string{"p"},
			Usage:   "Choose a proxy server to execute commands.",
		},
		&cli.StringFlag{
			Name:    "ssh-server",
			Aliases: []string{"ss"},
			Usage:   "Choose a ssh server to execute commands.",
		},
		&cli.StringFlag{
			Name:    "git-user",
			Aliases: []string{"gu"},
			Usage:   "Choose a git account to set GIT_SSH_COMMAND env.",
		},
		&cli.StringFlag{
			Name:    "config-path",
			Aliases: []string{"c"},
			Usage:   "Choose a config file path.",
			EnvVars: []string{"NB_CONFIG_PATH"},
		},
		&cli.BoolFlag{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "Print version.",
		},
	},
	Before: func(c *cli.Context) error {
		return singleton.Init(c.String("config-path"))
	},
	Action: func(c *cli.Context) error {
		if c.Bool("version") {
			fmt.Println(version)
			return nil
		}

		args := c.Args().Slice()
		if len(args) == 0 {
			return cli.ShowAppHelp(c)
		}

		var env []string

		proxyName := c.String("proxy")
		if proxyName != "" {
			server, exists := singleton.Config.Proxy[proxyName]
			if !exists {
				return cli.Exit("proxy server not found: "+proxyName, 1)
			}
			socksHost, socksPort, _ := net.SplitHostPort(server.Socks)
			env = append(env, fmt.Sprintf("all_proxy=socks5h://%s:%s", socksHost, socksPort))
			if server.Http != "" {
				httpHost, httpPort, _ := net.SplitHostPort(server.Http)
				env = append(env, fmt.Sprintf("http_proxy=http://%s:%s", httpHost, httpPort))
				env = append(env, fmt.Sprintf("https_proxy=http://%s:%s", httpHost, httpPort))
			}
		}

		if len(args) > 1 {
			return ExecuteInHost(env, args[0], args[1:]...)
		}
		return ExecuteInHost(env, args[0])
	},
}

func Execute() error {
	return rootCmd.Run(os.Args)
}
