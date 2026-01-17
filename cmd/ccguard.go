package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/naiba/nb/internal/ccguard"
	"github.com/naiba/nb/internal/ccguard/tui"
)

func init() {
	rootCmd.Commands = append(rootCmd.Commands, ccguardCmd)
}

var ccguardCmd = &cli.Command{
	Name:  "ccguard",
	Usage: "Claude Code 守卫 - 自动化控制 ccr code",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "指定配置文件",
		},
		&cli.BoolFlag{
			Name:  "show-policy",
			Usage: "显示当前策略",
		},
		&cli.BoolFlag{
			Name:  "no-tui",
			Usage: "不使用 TUI 界面",
		},
		&cli.BoolFlag{
			Name:    "debug",
			Aliases: []string{"d"},
			Usage:   "启用调试模式，将所有行为和交互记录到日志文件",
		},
		&cli.StringFlag{
			Name:  "debug-log",
			Usage: "调试日志文件路径 (默认: ccguard.log)",
			Value: "ccguard.log",
		},
	},
	Action: runCCGuard,
}

func runCCGuard(ctx context.Context, cmd *cli.Command) error {
	config := ccguard.LoadConfig()

	if cmd.Bool("show-policy") {
		fmt.Println("当前策略:")
		fmt.Println(config.Policy)
		return nil
	}

	// 初始化调试日志
	if cmd.Bool("debug") {
		logPath := cmd.String("debug-log")
		if err := ccguard.InitLogger(logPath); err != nil {
			return fmt.Errorf("初始化调试日志失败: %w", err)
		}
		defer ccguard.CloseLogger()
		fmt.Printf("[CCGuard] 调试模式已启用，日志文件: %s\n", logPath)
	}

	task := strings.Join(cmd.Args().Slice(), " ")
	ccguard.DebugLog("任务: %s", task)
	ccguard.DebugLog("配置: CCRCommand=%s, Model=%s, PollInterval=%v", config.CCRCommand, config.Model, config.PollInterval)

	guard := ccguard.NewGuard(config, task)

	if err := guard.Start(); err != nil {
		return fmt.Errorf("启动 ccr code 失败: %w", err)
	}
	defer guard.Close()

	if cmd.Bool("no-tui") {
		return guard.Run()
	}

	// 带状态栏模式运行
	return tui.Run(guard)
}
