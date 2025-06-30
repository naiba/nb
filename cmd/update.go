package cmd

import (
	"context"
	"log"

	"github.com/blang/semver"
	"github.com/nezhahq/go-github-selfupdate/selfupdate"
	"github.com/urfave/cli/v3"
)

var updateCmd = &cli.Command{
	Name:  "update",
	Usage: "Update nb to the latest version.",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		v := semver.MustParse(version)
		latest, err := selfupdate.UpdateSelf(v, "naiba/nb")
		if err != nil {
			return err
		}
		if latest.Version.Equals(v) {
			// latest version is the same as current version. It means current binary is up to date.
			log.Println("Current binary is the latest version", version)
		} else {
			log.Println("Successfully updated to version", latest.Version)
			log.Println("Release note:\n", latest.ReleaseNotes)
		}
		return nil
	},
}

func init() {
	rootCmd.Commands = append(rootCmd.Commands, updateCmd)
}
