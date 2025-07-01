package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal/tron"
)

var tronCmd = &cli.Command{
	Name:  "tron",
	Usage: "Tron helper.",
	Commands: []*cli.Command{
		castCallCmd,
	},
}

var castCallCmd = &cli.Command{
	Name:    "cast-call",
	Aliases: []string{"cc"},
	Usage:   "Tron cast call helper.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "rpc-url",
			Aliases: []string{"r"},
			Value:   "https://api.trongrid.io/jsonrpc",
		},
		&cli.IntFlag{
			Name:    "block",
			Aliases: []string{"b"},
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		rpcUrl := cmd.String("rpc-url")
		if rpcUrl == "" {
			return fmt.Errorf("RPC endpoint is required")
		}
		block := cmd.Int("block")
		if block > 0 {
			return fmt.Errorf("block number is not supported yet")
		}
		for _, arg := range cmd.Args().Slice() {
			if arg == "-b" || arg == "--block" {
				return fmt.Errorf("block number is not supported yet")
			}
		}
		return tron.CastCall(rpcUrl, cmd.Args().Slice())
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, tronCmd)
}
