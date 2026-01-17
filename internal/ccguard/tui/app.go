package tui

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/naiba/nb/internal/ccguard"
)

// TitleBar 在终端标题栏显示状态，完全不干扰claude code cli
type TitleBar struct {
	guard    *ccguard.Guard
	quit     chan struct{}
	quitOnce sync.Once
	oldTitle string
	wg       sync.WaitGroup
}

func NewTitleBar(guard *ccguard.Guard) *TitleBar {
	return &TitleBar{
		guard: guard,
		quit:  make(chan struct{}),
	}
}

// setTitle 设置终端标题
func setTitle(title string) {
	fmt.Printf("\033]0;%s\007", title)
}

// Start 启动标题栏状态显示（非阻塞）
func (t *TitleBar) Start() {
	// 保存原标题（尝试）
	t.oldTitle = os.Getenv("TERM_PROGRAM")

	// 立即显示初始状态
	t.updateTitle()

	// 启动更新循环
	t.wg.Add(1)
	go t.updateLoop()
}

func (t *TitleBar) updateLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.quit:
			return
		case <-ticker.C:
			t.updateTitle()
		}
	}
}

func (t *TitleBar) updateTitle() {
	var stateStr string
	switch t.guard.GetState() {
	case ccguard.StateRunning:
		stateStr = "🟢 自动"
	case ccguard.StateWaitingUser:
		stateStr = "🟡 人工介入"
	case ccguard.StatePaused:
		stateStr = "🟠 暂停"
	default:
		stateStr = "⚫ 停止"
	}

	autoCount, humanCount, duration := t.guard.GetStats()

	title := fmt.Sprintf("CCGuard %s | 自动:%d 人工:%d | %s | Ctrl+G暂停 Ctrl+\\退出",
		stateStr, autoCount, humanCount, formatDuration(duration))

	setTitle(title)
}

func (t *TitleBar) Close() {
	t.quitOnce.Do(func() {
		close(t.quit)
	})

	// 等待 updateLoop 退出
	t.wg.Wait()

	// 恢复标题
	setTitle("Terminal")
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
}

// Run 启动标题栏状态显示并等待guard结束
func Run(guard *ccguard.Guard) error {
	tb := NewTitleBar(guard)
	tb.Start()
	defer tb.Close()

	// 捕获 Ctrl+C 以便清理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Guard 运行结果 channel
	guardDone := make(chan error, 1)

	// 在 goroutine 中运行 guard
	go func() {
		guardDone <- guard.Run()
	}()

	// 等待信号或 guard 完成
	select {
	case <-sigChan:
		// 收到中断信号，优雅关闭
		ccguard.DebugLog("TUI: 收到中断信号，开始清理")
		guard.Close()
		// 等待 guard 完全退出
		<-guardDone
		return nil
	case err := <-guardDone:
		// Guard 正常退出
		return err
	}
}
