package cmd

import (
	"fmt"
	"os"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

var rootCmd = &cli.App{
	Name:  "nb",
	Usage: "Nb is not only no bullshit.",
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
			Name:    "config-path",
			Aliases: []string{"c"},
			Usage:   "Choose a config file path.",
			EnvVars: []string{"NB_CONFIG_PATH"},
		},
	},
	Before: func(c *cli.Context) error {
		return singleton.Init(c.String("config-path"))
	},
	Action: func(c *cli.Context) error {
		args := c.Args().Slice()
		if len(args) == 0 {
			return nil
		}
		if len(args) > 1 {
			return ExecuteInHost(nil, args[0], args[1:]...)
		}
		return ExecuteInHost(nil, args[0])
	},
}

func Execute() {
	if err := rootCmd.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
