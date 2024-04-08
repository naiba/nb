package cmd

import (
	"os"
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

var s *session.Session

var awakeCmd = &cli.Command{
	Name:            "awake",
	Aliases:         []string{"a"},
	Usage:           "Awake during the command is running.",
	SkipFlagParsing: true,
	Before: func(c *cli.Context) error {
		s = session.New(0, os.Getppid())
		return s.Start()
	},
	Action: func(c *cli.Context) error {
		retCmd := ExecuteLineInHost(strings.Join(c.Args().Slice(), " "))
		retSession := s.Stop()
		if retCmd != nil {
			return retCmd
		}
		return retSession
	},
}

var awakeBeepCmd = &cli.Command{
	Name:            "awake-beep",
	Aliases:         []string{"ab"},
	Usage:           "Awake and beep when an command is finished.",
	SkipFlagParsing: true,
	Before: func(c *cli.Context) error {
		s = session.New(0, os.Getppid())
		return s.Start()
	},
	Action: func(c *cli.Context) error {
		retCmd := beepCmd.Action(c)
		retSession := s.Stop()
		if retCmd != nil {
			return retCmd
		}
		return retSession
	},
}
