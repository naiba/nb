package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, anchorCmd)
}

var anchorCmd = &cli.Command{
	Name:  "anchor",
	Usage: "Anchor helper.",
	Subcommands: []*cli.Command{
		changeEnvCmd,
	},
}

var changeEnvCmd = &cli.Command{
	Name:            "env",
	Aliases:         []string{"e"},
	Usage:           "change anchor env.",
	SkipFlagParsing: true,
	Action: func(c *cli.Context) error {
		envName := c.Args().First()
		if envName == "" {
			return errors.New("env name is required")
		}
		currentPath, err := os.Getwd()
		if err != nil {
			return err
		}
		currentPath += "/target/deploy/"
		log.Println("Current path: ", currentPath)
		envKeySuffix := fmt.Sprintf(".%s.json", envName)
		log.Printf("Searching env file with suffix: %s", envKeySuffix)
		var matchedKeys []string
		filepath.WalkDir(currentPath, func(path string, d os.DirEntry, err error) error {
			fileName := d.Name()
			if strings.HasSuffix(fileName, envKeySuffix) {
				log.Printf("Found env file: %s", fileName)
				matchedKeys = append(matchedKeys, fileName)
			}
			return nil
		})
		if len(matchedKeys) == 0 {
			return fmt.Errorf("no matched env file(%s) found", envKeySuffix)
		}
		for _, key := range matchedKeys {
			baseName, err := getBaseNameFromKeyName(key)
			if err != nil {
				return err
			}
			baseName += ".json"
			if err := os.Remove(filepath.Join(currentPath, baseName)); err != nil {
				return err
			}
			if err := os.Symlink(filepath.Join(currentPath, key), filepath.Join(currentPath, baseName)); err != nil {
				return err
			}
		}

		keysSyncCmd := exec.Command("anchor", "keys", "sync")
		keysSyncCmd.Dir = currentPath
		keysSyncCmd.Stdin = os.Stdin
		keysSyncCmd.Stdout = os.Stdout
		keysSyncCmd.Stderr = os.Stderr
		if err := keysSyncCmd.Run(); err != nil {
			return err
		}

		buildCmd := exec.Command("anchor", "build")
		buildCmd.Dir = currentPath
		buildCmd.Stdin = os.Stdin
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		return buildCmd.Run()
	},
}

func getBaseNameFromKeyName(key string) (string, error) {
	split := strings.Split(key, ".")
	if len(split) < 2 {
		return "", errors.New("invalid key, need (a.[env].json)")
	}
	return strings.Join(split[:len(split)-2], "."), nil
}
