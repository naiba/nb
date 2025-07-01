package cmd

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/AppleGamer22/cocainate/session"
	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, runCmd)
}

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "Commands run helper.",
	Commands: []*cli.Command{
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
	Action: func(ctx context.Context, cmd *cli.Command) error {
		errExec := internal.BashScriptExecuteInHost(strings.Join(cmd.Args().Slice(), " "))
		errBeep := internal.BashScriptExecuteInHost(getBeepCommand())
		return errors.Join(errExec, errBeep)
	},
}

var awakeCmd = &cli.Command{
	Name:            "awake",
	Aliases:         []string{"a"},
	Usage:           "Awake during the command is running.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		command := internal.BuildCommand(nil, "bash", "-c", strings.Join(cmd.Args().Slice(), " "))
		if err := command.Start(); err != nil {
			return err
		}
		s := session.New(0, command.Process.Pid)
		if err := s.Start(); err != nil {
			return err
		}
		errExec := command.Wait()
		errSessionStop := s.Stop()
		return errors.Join(errExec, errSessionStop)
	},
}

var awakeBeepCmd = &cli.Command{
	Name:            "awake-beep",
	Aliases:         []string{"ab"},
	Usage:           "Awake and beep when an command is finished.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		command := internal.BuildCommand(nil, "bash", "-c", strings.Join(cmd.Args().Slice(), " "))
		if err := command.Start(); err != nil {
			return err
		}
		s := session.New(0, command.Process.Pid)
		if err := s.Start(); err != nil {
			return err
		}
		errExec := command.Wait()
		errSessionStop := s.Stop()
		errBeep := internal.BashScriptExecuteInHost(getBeepCommand())
		return errors.Join(errExec, errSessionStop, errBeep)
	},
}
