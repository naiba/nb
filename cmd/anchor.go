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
		programName := cmd.Args().Get(0)
		envName := cmd.Args().Get(1)
		if programName == "" || envName == "" {
			return errors.New("usage: nb anchor naming-env <program> <env>")
		}
		currentPath, err := os.Getwd()
		if err != nil {
			return err
		}
		currentPath = filepath.Join(currentPath, "target", "deploy")
		// 只匹配指定 program 的 keypair，避免影响其他 program 的环境配置
		keypairName := fmt.Sprintf("%s-keypair.json", programName)
		keypairPath := filepath.Join(currentPath, keypairName)
		if _, err := os.Stat(keypairPath); err != nil {
			return fmt.Errorf("keypair file not found: %s", keypairName)
		}
		log.Printf("Found keypair: %s", keypairName)

		envKeySuffix := fmt.Sprintf(".%s.json", envName)
		envBaseName := strings.TrimSuffix(keypairName, filepath.Ext(keypairName)) + envKeySuffix
		envPath := filepath.Join(currentPath, envBaseName)
		if _, err := os.Stat(envPath); err == nil {
			log.Printf("Env file(%s) already exists", envBaseName)
			return nil
		}
		envKeyFile, err := os.Create(envPath)
		if err != nil {
			return fmt.Errorf("failed to create env file: %w", err)
		}
		defer envKeyFile.Close()
		oldKeyFile, err := os.Open(keypairPath)
		if err != nil {
			return fmt.Errorf("failed to open keypair file: %w", err)
		}
		defer oldKeyFile.Close()
		if _, err = io.Copy(envKeyFile, oldKeyFile); err != nil {
			return fmt.Errorf("failed to copy keypair file: %w", err)
		}
		log.Printf("Env file(%s) created", envBaseName)
		return nil
	},
}

var switchingEnvCmd = &cli.Command{
	Name:            "switching-env",
	Aliases:         []string{"se"},
	Usage:           "Switching anchor env. Usage: nb anchor se <program> <env>",
	SkipFlagParsing: true,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		programName := cmd.Args().Get(0)
		envName := cmd.Args().Get(1)
		if programName == "" || envName == "" {
			return errors.New("usage: nb anchor switching-env <program> <env>")
		}
		currentPath, err := os.Getwd()
		if err != nil {
			return err
		}
		currentPath = filepath.Join(currentPath, "target", "deploy")

		// 只操作指定 program 的 keypair，避免影响其他 program 的环境配置
		envFileName := fmt.Sprintf("%s-keypair.%s.json", programName, envName)
		envFilePath := filepath.Join(currentPath, envFileName)
		if _, err := os.Stat(envFilePath); err != nil {
			return fmt.Errorf("env keypair not found: %s (run 'nb anchor ne %s %s' first)", envFileName, programName, envName)
		}
		log.Printf("Found env keypair: %s", envFileName)

		symlinkName := fmt.Sprintf("%s-keypair.json", programName)
		symlinkPath := filepath.Join(currentPath, symlinkName)
		// 删除旧的 keypair 文件/符号链接，替换为指向目标环境的符号链接
		if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old keypair: %w", err)
		}
		if err := os.Symlink(envFilePath, symlinkPath); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
		log.Printf("Symlink: %s -> %s", symlinkName, envFileName)

		// anchor keys sync 和 build 都限定到指定 program，避免重新编译全部程序
		projectPath := filepath.Join(currentPath, "..", "..")
		keysSyncCmd := exec.Command("anchor", "keys", "sync", "--program-name", programName)
		keysSyncCmd.Dir = projectPath
		keysSyncCmd.Stdin = os.Stdin
		keysSyncCmd.Stdout = os.Stdout
		keysSyncCmd.Stderr = os.Stderr
		if err := keysSyncCmd.Run(); err != nil {
			return err
		}

		buildCmd := exec.Command("anchor", "build", "-p", programName)
		buildCmd.Dir = projectPath
		buildCmd.Stdin = os.Stdin
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		return buildCmd.Run()
	},
}
