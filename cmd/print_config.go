package cmd

import (
	"fmt"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, printConfigCmd)
}

var printConfigCmd = &cli.Command{
	Name:  "print-config",
	Usage: "Prints the current configuration.",
	Action: func(c *cli.Context) error {
		fmt.Printf("%+v", singleton.Config)
		return nil
	},
}
