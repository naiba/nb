package cmd

import (
	"bytes"
	"context"
	"os"

	"github.com/dimiro1/banner"
	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/assets"
	"github.com/naiba/nb/singleton"
)

var printBannerCmd = &cli.Command{
	Name:  "print-banner",
	Usage: "Can be used to print banners at terminal startup.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if singleton.Config.Banner != "" {
			banner.Init(os.Stdout, true, true, bytes.NewBufferString(singleton.Config.Banner))
		} else {
			banner.Init(os.Stdout, true, true, bytes.NewBufferString(assets.Nyancat))
		}
		return nil
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, printBannerCmd)
}
