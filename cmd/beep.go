package cmd

import (
	"runtime"
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
		if runtime.GOOS != "darwin" {
			return ExecuteLineInHost(strings.Join(c.Args().Slice(), " ") + " && echo -ne '\007'")
		}
		return ExecuteLineInHost(strings.Join(c.Args().Slice(), " ") + " && say Boom! Mission accomplished!")
	},
}
