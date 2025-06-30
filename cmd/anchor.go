package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, anchorCmd)
}

var anchorCmd = &cli.Command{
	Name:  "anchor",
	Usage: "Anchor helper.",
	Commands: []*cli.Command{
		switchingEnvCmd,
		namingEnvCmd,
	},
}

var namingEnvCmd = &cli.Command{
	Name:            "naming-env",
	Aliases:         []string{"ne"},
	Usage:           "Naming anchor env.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		envName := cmd.Args().First()
		if envName == "" {
			return errors.New("env name is required")
		}
		currentPath, err := os.Getwd()
		if err != nil {
			return err
		}
		currentPath = filepath.Join(currentPath, "target", "deploy")
		log.Println("Current path: ", currentPath)
		envKeySuffix := fmt.Sprintf(".%s.json", envName)
		var matchedKeys []string
		filepath.WalkDir(currentPath, func(path string, d os.DirEntry, err error) error {
			fileName := d.Name()
			if strings.HasSuffix(fileName, "-keypair.json") {
				log.Printf("Found env file: %s", fileName)
				matchedKeys = append(matchedKeys, fileName)
			}
			return nil
		})
		if len(matchedKeys) == 0 {
			return fmt.Errorf("no matched env file(%s) found", envKeySuffix)
		}
		for _, key := range matchedKeys {
			baseName := strings.TrimSuffix(key, filepath.Ext(key))
			baseName += envKeySuffix
			if _, err := os.Stat(filepath.Join(currentPath, baseName)); err == nil {
				log.Printf("Env file(%s) already exists", baseName)
				continue
			}
			envKeyFile, err := os.Create(filepath.Join(currentPath, baseName))
			if err != nil {
				log.Fatalf("Failed to create env file: %v", err)
			}
			defer envKeyFile.Close()
			oldKeyFile, err := os.Open(filepath.Join(currentPath, key))
			if err != nil {
				log.Fatalf("Failed to open env file: %v", err)
			}
			defer oldKeyFile.Close()
			_, err = io.Copy(envKeyFile, oldKeyFile)
			if err != nil {
				log.Fatalf("Failed to copy env file: %v", err)
			}
			log.Printf("Env file(%s) created", baseName)
		}
		return nil
	},
}

var switchingEnvCmd = &cli.Command{
	Name:            "switching-env",
	Aliases:         []string{"se"},
	Usage:           "Switching anchor env.",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		envName := cmd.Args().First()
		if envName == "" {
			return errors.New("env name is required")
		}
		currentPath, err := os.Getwd()
		if err != nil {
			return err
		}
		currentPath = filepath.Join(currentPath, "target", "deploy")
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
				log.Fatalf("Failed to remove env file: %v", err)
			}
			if err := os.Symlink(filepath.Join(currentPath, key), filepath.Join(currentPath, baseName)); err != nil {
				log.Fatalf("Failed to Symlink env file: %v", err)
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
