package cmd

import (
	"bytes"
	"os"

	"github.com/dimiro1/banner"
	"github.com/naiba/nb/assets"
	"github.com/naiba/nb/singleton"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(printBannerCmd)
}

var printBannerCmd = &cobra.Command{
	Use:   "print-banner",
	Short: "可用在终端启动时打印 banner",
	Run: func(cmd *cobra.Command, args []string) {
		if singleton.Config.Banner != "" {
			banner.Init(os.Stdout, true, true, bytes.NewBufferString(singleton.Config.Banner))
		} else {
			banner.Init(os.Stdout, true, true, bytes.NewBufferString(assets.Nyancat))
		}
	},
}
