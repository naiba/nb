package cmd

import (
	"fmt"
	"os"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, printSnippedCmd)
}

var printSnippedCmd = &cli.Command{
	Name:            "print-snippet",
	Usage:           "Prints code snippet.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		fmt.Fprint(os.Stdout, singleton.Config.Snippet[c.Args().First()])
		return nil
	},
}
