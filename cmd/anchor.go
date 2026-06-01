package cmd

import (
	"bytes"
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

// ensureCurrentKeypairBackedUp 在切换前校验 currentPath/<program>-keypair.json：
//   - 不存在或是 symlink：可以安全替换。
//   - 是普通文件：必须能在同目录找到一个 <program>-keypair.*.json 与其字节完全一致，
//     否则视为未通过 naming-env 备份，拒绝继续，避免 key 丢失。
func ensureCurrentKeypairBackedUp(deployDir, symlinkPath, programName string) error {
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat current keypair: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	current, err := os.ReadFile(symlinkPath)
	if err != nil {
		return fmt.Errorf("failed to read current keypair: %w", err)
	}

	prefix := fmt.Sprintf("%s-keypair.", programName)
	entries, err := os.ReadDir(deployDir)
	if err != nil {
		return fmt.Errorf("failed to read deploy dir: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		// 跳过 <program>-keypair.json 自身，只比对 <program>-keypair.<env>.json
		if name == fmt.Sprintf("%s-keypair.json", programName) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(deployDir, name))
		if err != nil {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(data), bytes.TrimSpace(current)) {
			return nil
		}
	}
	return fmt.Errorf("current %s-keypair.json has no matching env backup; run 'nb anchor ne %s <env>' first to avoid losing it", programName, programName)
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
		// 切换前必须确保当前 keypair 已被某个 env 备份；否则 os.Remove 会永久丢失这把 key。
		// 之前出过事：从未跑过 naming-env 的 program 直接 switching-env，原始 keypair 直接被删。
		if err := ensureCurrentKeypairBackedUp(currentPath, symlinkPath, programName); err != nil {
			return err
		}
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
