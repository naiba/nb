package cmd

import (
	"bytes"
	"os"

	"github.com/dimiro1/banner"
	"github.com/naiba/nb/assets"
	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, printBannerCmd)
}

var printBannerCmd = &cli.Command{
	Name:  "print-banner",
	Usage: "Can be used to print banners at terminal startup.",
	Action: func(c *cli.Context) error {
		if singleton.Config.Banner != "" {
			banner.Init(os.Stdout, true, true, bytes.NewBufferString(singleton.Config.Banner))
		} else {
			banner.Init(os.Stdout, true, true, bytes.NewBufferString(assets.Nyancat))
		}
		return nil
	},
}
