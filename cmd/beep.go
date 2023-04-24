package cmd

import (
	"strings"

	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, beepCmd)
}

var beepCmd = &cli.Command{
	Name:            "beep",
	Aliases:         []string{"b"},
	Usage:           "Beep when an command is finished.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		return ExecuteLineInHost(strings.Join(c.Args().Slice(), " ") + " && echo -ne '\007'")
	},
}
