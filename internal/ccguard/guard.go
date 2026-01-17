package ccguard

import (
	"fmt"
	"os"
	"sync"
	"time"
)

type Guard struct {
	mu            sync.RWMutex
	config        *Config
	process       *PTYProcess
	judge         *Judge
	state         GuardState
	autoCount     int
	humanCount    int
	startTime     time.Time
	lastOutput    string
	modelSelector *ModelSelector    // 模型选择器
	notifier      *PlatformNotifier // 跨平台通知器
	userInput     chan string
	stateChange   chan GuardState
	stopChan      chan struct{} // 停止信号
	stopOnce      sync.Once     // 确保 stopChan 只关闭一次
}

func NewGuard(config *Config, task string) *Guard {
	args := []string{"code"}
	if task != "" {
		args = append(args, task)
	}

	return &Guard{
		config:        config,
		process:       NewPTYProcess(config.CCRCommand, args...),
		judge:         NewJudge(config, task),
		state:         StateRunning,
		startTime:     time.Now(),
		modelSelector: NewModelSelector(config.Model),
		notifier:      NewPlatformNotifier(config.Notify.Bell, config.Notify.Sound),
		userInput:     make(chan string, DefaultChannelBuffer),
		stateChange:   make(chan GuardState, DefaultChannelBuffer),
		stopChan:      make(chan struct{}),
	}
}

func (g *Guard) Start() error {
	DebugLog("Guard.Start() 开始")

	// 设置暂停/恢复切换回调 (Ctrl+G)
	g.process.SetToggleCallback(func() {
		g.mu.Lock()
		switch g.state {
		case StateRunning:
			g.state = StatePaused
			DebugLog("状态变更: Running -> Paused (用户按下 Ctrl+G)")
			fmt.Fprintf(os.Stderr, "\r\n[CCGuard] 已暂停自动控制 (Ctrl+G 恢复)\r\n")
		case StatePaused:
			g.state = StateRunning
			DebugLog("状态变更: Paused -> Running (用户按下 Ctrl+G)")
			fmt.Fprintf(os.Stderr, "\r\n[CCGuard] 已恢复自动控制\r\n")
		}
		g.mu.Unlock()
	})

	// 设置用户输入回调 - 人工介入后自动恢复
	g.process.SetUserInputCallback(func() {
		g.mu.Lock()
		if g.state == StateWaitingUser {
			DebugLog("状态变更: WaitingUser -> Running (用户手动输入)")
			g.state = StateRunning
		}
		g.mu.Unlock()
	})

	// 设置退出回调 (Ctrl+\)
	g.process.SetExitCallback(func() {
		DebugLog("收到退出信号 (Ctrl+\\)")
		fmt.Fprintf(os.Stderr, "\r\n[CCGuard] 正在退出...\r\n")
		g.Stop()
	})

	// 设置子进程退出回调
	g.process.SetProcessExitCallback(func() {
		DebugLog("子进程已退出，停止 Guard")
		fmt.Fprintf(os.Stderr, "\r\n[CCGuard] ccr 已退出\r\n")
		g.Stop()
	})

	err := g.process.Start()
	if err != nil {
		DebugLog("Guard.Start() 失败: %v", err)
	} else {
		DebugLog("Guard.Start() 成功，进程已启动")
	}
	return err
}

// Stop 停止Guard
func (g *Guard) Stop() {
	g.stopOnce.Do(func() {
		close(g.stopChan)
	})
}

// SetOutputCallback 设置输出回调（用于TUI模式显示输出）
func (g *Guard) SetOutputCallback(cb func([]byte)) {
	g.process.SetOutputCallback(cb)
}

// GetRecentOutput 获取最近的输出内容
func (g *Guard) GetRecentOutput() string {
	return g.process.GetRecentOutput()
}

func (g *Guard) Run() error {
	DebugLog("Guard.Run() 主循环开始")
	ticker := time.NewTicker(g.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopChan:
			DebugLog("Guard.Run() 收到停止信号，退出主循环")
			return nil

		case <-ticker.C:
			g.mu.RLock()
			currentState := g.state
			g.mu.RUnlock()

			if currentState != StateRunning {
				continue
			}

			if !g.process.IsRunning() {
				DebugLog("子进程已退出，停止 Guard")
				g.mu.Lock()
				g.state = StateStopped
				g.mu.Unlock()
				return nil
			}

			output := g.process.GetRecentOutput()
			if output == g.lastOutput {
				continue // 无新输出
			}
			g.lastOutput = output
			DebugLogOutput("检测到新输出", output)

			// 检测模型选择提示，自动输入配置的模型
			if handled := g.handleModelSelection(output); handled {
				continue
			}

			DebugLog("调用 Judge.Decide() 进行判断")
			decision, err := g.judge.Decide(output)
			if err != nil {
				DebugLog("Judge.Decide() 错误: %v", err)
				fmt.Fprintf(os.Stderr, "\r\n[守卫] 判断失败: %v\r\n", err)
				continue
			}

			DebugLog("判断结果: Action=%s, Content=%s", decision.Action, decision.Content)

			switch decision.Action {
			case "WAIT":
				DebugLog("动作: WAIT - 继续等待")
			case "SELECT":
				// 选择操作：只发送按键，不需要回车
				DebugLog("动作: SELECT - 自动选择: %s", decision.Content)
				g.process.SendInput(decision.Content)
				g.mu.Lock()
				g.autoCount++
				g.mu.Unlock()
				fmt.Fprintf(os.Stderr, "[守卫] 自动选择: %s\n", decision.Content)
			case "INPUT":
				// 检测是否是选择场景（有导航提示），选择场景不需要回车
				if IsSelectScene(output) {
					// 选择场景：只发送按键，不需要回车
					DebugLog("动作: INPUT (检测为选择场景) - 自动选择: %s", decision.Content)
					g.process.SendInput(decision.Content)
					g.mu.Lock()
					g.autoCount++
					g.mu.Unlock()
					fmt.Fprintf(os.Stderr, "[守卫] 自动选择: %s\n", decision.Content)
				} else {
					// 输入场景：发送内容后需要回车
					DebugLog("动作: INPUT - 自动输入: %s", decision.Content)
					g.process.SendInput(decision.Content + "\n")
					g.mu.Lock()
					g.autoCount++
					g.mu.Unlock()
					fmt.Fprintf(os.Stderr, "[守卫] 自动输入: %s\n", decision.Content)
				}
			case "HUMAN":
				DebugLog("动作: HUMAN - 需要人工介入: %s", decision.Content)
				g.mu.Lock()
				g.state = StateWaitingUser
				g.humanCount++
				g.mu.Unlock()
				g.notifier.Notify()
				fmt.Fprintf(os.Stderr, "\n[守卫] 需要人工介入: %s\n", decision.Content)
			}

		case input := <-g.userInput:
			DebugLog("收到用户输入: %s", input)
			g.process.SendInput(input + "\n")
			g.mu.Lock()
			g.state = StateRunning
			g.mu.Unlock()

		case newState := <-g.stateChange:
			DebugLog("状态变更请求: %d", newState)
			g.mu.Lock()
			g.state = newState
			g.mu.Unlock()
		}
	}
}

// SendUserInput 发送用户输入（非阻塞，如果 channel 满则丢弃）
func (g *Guard) SendUserInput(input string) {
	select {
	case g.userInput <- input:
	case <-g.stopChan:
		// Guard 已停止，忽略输入
	default:
		DebugLog("SendUserInput: channel 已满，丢弃输入")
	}
}

// SetState 设置状态（非阻塞，如果 channel 满则丢弃）
func (g *Guard) SetState(state GuardState) {
	select {
	case g.stateChange <- state:
	case <-g.stopChan:
		// Guard 已停止，忽略状态变更
	default:
		DebugLog("SetState: channel 已满，丢弃状态变更")
	}
}

func (g *Guard) GetState() GuardState {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.state
}

func (g *Guard) GetStats() (autoCount, humanCount int, duration time.Duration) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.autoCount, g.humanCount, time.Since(g.startTime)
}

func (g *Guard) Close() {
	g.process.Close()

	// 关闭 channel（在 Stop 之后调用，确保 Run 循环已退出）
	g.mu.Lock()
	if g.userInput != nil {
		close(g.userInput)
		g.userInput = nil
	}
	if g.stateChange != nil {
		close(g.stateChange)
		g.stateChange = nil
	}
	g.mu.Unlock()
}

// handleModelSelection 检测模型选择提示并自动输入配置的模型
// 返回 true 表示已处理模型选择
func (g *Guard) handleModelSelection(output string) bool {
	// 使用 ModelSelector 检查
	if !g.modelSelector.IsConfigured() || g.modelSelector.IsSelected() {
		return false
	}

	// 检测是否包含模型选择提示
	if !g.modelSelector.NeedsSelection(output) {
		return false
	}

	DebugLog("检测到模型选择提示")

	// 查找配置的模型对应的编号
	modelNum := g.modelSelector.FindModelNumber(output)
	if modelNum == "" {
		// 配置的模型名称不在列表中
		DebugLog("警告: 配置的模型 '%s' 不在可用列表中", g.modelSelector.GetModelName())
		fmt.Fprintf(os.Stderr, "[守卫] 警告: 配置的模型 '%s' 不在可用列表中\n", g.modelSelector.GetModelName())
		return false
	}

	DebugLog("自动选择模型: %s (编号 %s)", g.modelSelector.GetModelName(), modelNum)
	// ccr 的模型选择需要回车确认（和 Claude Code 内部的选项选择不同）
	g.process.SendInput(modelNum + "\n")

	g.mu.Lock()
	g.autoCount++
	g.mu.Unlock()

	g.modelSelector.MarkSelected()
	fmt.Fprintf(os.Stderr, "[守卫] 自动选择模型: %s (编号 %s)\n", g.modelSelector.GetModelName(), modelNum)
	return true
}
