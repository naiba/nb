package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/singleton"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, printConfigCmd)
}

var printConfigCmd = &cli.Command{
	Name:  "print-config",
	Usage: "Prints the current configuration.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		fmt.Printf("%+v", singleton.Config)
		return nil
	},
}
