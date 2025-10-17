package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal/tron"
	"github.com/naiba/nb/model"
)

var tronCmd = &cli.Command{
	Name:  "tron",
	Usage: "Tron helper.",
	Commands: []*cli.Command{
		tronVanityCmd,
		rpcProxyCmd,
	},
}

var tronVanityCmd = &cli.Command{
	Name:    "vanity",
	Aliases: []string{"v"},
	Usage:   "Generate a vanity Tron address",
	Flags:   model.VanityFlags(),
	Action: func(ctx context.Context, cmd *cli.Command) error {
		config, err := model.ParseVanityConfig(cmd)
		if err != nil {
			return err
		}
		return tron.VanityAddress(config)
	},
}

var rpcProxyCmd = &cli.Command{
	Name:  "rpc-proxy",
	Usage: "Tron RPC proxy.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "rpc-url",
			Aliases: []string{"r"},
			Value:   "https://api.trongrid.io/jsonrpc",
		},
		&cli.StringSliceFlag{
			Name: "override-code",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		rpcUrl := cmd.String("rpc-url")
		if rpcUrl == "" {
			return fmt.Errorf("RPC endpoint is required")
		}
		return tron.RpcProxy(rpcUrl, cmd.StringSlice("override-code"))
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, tronCmd)
}
