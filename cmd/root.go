package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
	"github.com/naiba/nb/singleton"
)

var version = "1.0.0"

var rootCmd = &cli.Command{
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
			Sources: cli.EnvVars("NB_CONFIG_PATH"),
		},
		&cli.BoolFlag{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "Print version.",
		},
	},
	Before: func(ctx context.Context, cmd *cli.Command) error {
		return singleton.Init(cmd.String("config-path"))
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Bool("version") {
			fmt.Println(version)
			return nil
		}

		args := cmd.Args().Slice()
		if len(args) == 0 {
			fmt.Println(cmd.Usage)
			return nil
		}

		var env []string

		proxyConfig, err := GetProxyConfig(cmd.String("proxy"))
		if err != nil {
			return err
		}
		if proxyConfig != nil {
			env = append(env, fmt.Sprintf("all_proxy=socks5h://%s:%s", proxyConfig.SocksHost, proxyConfig.SocksPort))
			if proxyConfig.HttpHost != "" {
				env = append(env, fmt.Sprintf("http_proxy=http://%s:%s", proxyConfig.HttpHost, proxyConfig.HttpPort))
				env = append(env, fmt.Sprintf("https_proxy=http://%s:%s", proxyConfig.HttpHost, proxyConfig.HttpPort))
			}
		}

		if len(args) > 1 {
			return internal.ExecuteInHost(env, args[0], args[1:]...)
		}
		return internal.ExecuteInHost(env, args[0])
	},
}

func Execute() error {
	return rootCmd.Run(context.Background(), os.Args)
}
