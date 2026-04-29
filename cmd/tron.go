package cmd

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal/tron"
	"github.com/naiba/nb/model"
)

var tronCmd = &cli.Command{
	Name:  "tron",
	Usage: "Tron helper.",
	Commands: []*cli.Command{
		tronVanityCmd,
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

func init() {
	rootCmd.Commands = append(rootCmd.Commands, tronCmd)
}
