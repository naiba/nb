package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/naiba/nb/singleton"
	"github.com/urfave/cli/v3"
)

var printSnippetCmd = &cli.Command{
	Name:            "print-snippet",
	Usage:           "Prints code snippet.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if singleton.Config != nil && singleton.Config.Snippet != nil {
			fmt.Fprint(os.Stdout, singleton.Config.Snippet[cmd.Args().First()])
		}
		return nil
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, printSnippetCmd)
}
