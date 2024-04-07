package cmd

import (
	"runtime"
	"strings"

	"github.com/AppleGamer22/cocainate/session"

	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, runCmd)
}

var runCmd = &cli.Command{
	Name:    "run",
	Aliases: []string{"r"},
	Usage:   "Commands run helper.",
	Subcommands: []*cli.Command{
		awakeCmd,
		beepCmd,
		awakeBeepCmd,
	},
}

var beepCmd = &cli.Command{
	Name:            "beep",
	Aliases:         []string{"b"},
	Usage:           "Beep when an command is finished.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		var beepCmd string
		if runtime.GOOS != "darwin" {
			beepCmd = "echo -ne '\007'"
		} else {
			beepCmd = "say Boom! Mission accomplished!"
		}
		errRunCmd := ExecuteLineInHost(strings.Join(c.Args().Slice(), " "))
		errBeep := ExecuteLineInHost(beepCmd)
		if errRunCmd != nil {
			return errRunCmd
		}
		return errBeep
	},
}

var s session.Session

var awakeCmd = &cli.Command{
	Name:            "awake",
	Aliases:         []string{"a"},
	Usage:           "Awake during the command is running.",
	SkipFlagParsing: true,
	Before: func(c *cli.Context) error {
		s = session.Session{}
		return s.Start()
	},
	Action: func(c *cli.Context) error {
		return ExecuteLineInHost(strings.Join(c.Args().Slice(), " "))
	},
	After: func(c *cli.Context) error {
		return s.Stop()
	},
}

var awakeBeepCmd = &cli.Command{
	Name:            "awake-beep",
	Aliases:         []string{"ab"},
	Usage:           "Awake and beep when an command is finished.",
	SkipFlagParsing: true,
	Before: func(c *cli.Context) error {
		s = session.Session{}
		return s.Start()
	},
	Action: func(c *cli.Context) error {
		return beepCmd.Action(c)
	},
	After: func(c *cli.Context) error {
		return s.Stop()
	},
}
