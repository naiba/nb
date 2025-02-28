package cmd

import (
	"errors"
	"runtime"
	"strings"

	"github.com/AppleGamer22/cocainate/session"
	"github.com/urfave/cli/v2"

	nbInternal "github.com/naiba/nb/internal"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, runCmd)
}

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "Commands run helper.",
	Subcommands: []*cli.Command{
		awakeCmd,
		beepCmd,
		awakeBeepCmd,
	},
}

func getBeepCommand() string {
	if runtime.GOOS != "darwin" {
		return "echo -ne '\007'"
	}
	return "afplay /System/Library/Sounds/Frog.aiff"
}

var beepCmd = &cli.Command{
	Name:            "beep",
	Aliases:         []string{"b"},
	Usage:           "Beep when an command is finished.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		errExec := BashScriptExecuteInHost(strings.Join(c.Args().Slice(), " "))
		errBeep := BashScriptExecuteInHost(getBeepCommand())
		return errors.Join(errExec, errBeep)
	},
}

var awakeCmd = &cli.Command{
	Name:            "awake",
	Aliases:         []string{"a"},
	Usage:           "Awake during the command is running.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		cmd := nbInternal.BuildCommand(nil, "bash", "-c", strings.Join(c.Args().Slice(), " "))
		if err := cmd.Start(); err != nil {
			return err
		}
		s := session.New(0, cmd.Process.Pid)
		if err := s.Start(); err != nil {
			return err
		}
		errExec := cmd.Wait()
		errSessionStop := s.Stop()
		return errors.Join(errExec, errSessionStop)
	},
}

var awakeBeepCmd = &cli.Command{
	Name:            "awake-beep",
	Aliases:         []string{"ab"},
	Usage:           "Awake and beep when an command is finished.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		cmd := nbInternal.BuildCommand(nil, "bash", "-c", strings.Join(c.Args().Slice(), " "))
		if err := cmd.Start(); err != nil {
			return err
		}
		s := session.New(0, cmd.Process.Pid)
		if err := s.Start(); err != nil {
			return err
		}
		errExec := cmd.Wait()
		errSessionStop := s.Stop()
		errBeep := BashScriptExecuteInHost(getBeepCommand())
		return errors.Join(errExec, errSessionStop, errBeep)
	},
}
