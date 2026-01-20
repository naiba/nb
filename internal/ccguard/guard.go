package ccguard

import (
	"sync"
	"time"
)

type Guard struct {
	mu             sync.RWMutex
	config         *Config
	process        *PTYProcess
	judge          *Judge
	state          GuardState
	autoCount      int
	humanCount     int
	startTime      time.Time
	lastOutput     string
	lastOutputTime time.Time         // 最后一次输出变化的时间
	judgedOutput   string            // 上次AI判断时的输出（避免重复判断）
	modelSelector  *ModelSelector    // 模型选择器
	notifier       *PlatformNotifier // 跨平台通知器
	userInput      chan string
	stateChange    chan GuardState
	stopChan       chan struct{} // 停止信号
	stopOnce       sync.Once     // 确保 stopChan 只关闭一次
}

func NewGuard(config *Config, task string) *Guard {
	args := []string{"code"}
	if task != "" {
		args = append(args, task)
	}

	return &Guard{
		config:         config,
		process:        NewPTYProcess(config.CCRCommand, args...),
		judge:          NewJudge(config, task),
		state:          StateRunning,
		startTime:      time.Now(),
		lastOutputTime: time.Now(),
		modelSelector:  NewModelSelector(config.Model),
		notifier:       NewPlatformNotifier(config.Notify.Bell, config.Notify.Sound),
		userInput:      make(chan string, DefaultChannelBuffer),
		stateChange:    make(chan GuardState, DefaultChannelBuffer),
		stopChan:       make(chan struct{}),
	}
}

func (g *Guard) Start() error {
	// 设置暂停/恢复切换回调 (Ctrl+G)
	g.process.SetToggleCallback(func() {
		g.mu.Lock()
		switch g.state {
		case StateRunning:
			g.state = StatePaused
		case StatePaused:
			g.state = StateRunning
		}
		g.mu.Unlock()
	})

	// 设置用户输入回调 - 人工介入后自动恢复
	g.process.SetUserInputCallback(func() {
		g.mu.Lock()
		if g.state == StateWaitingUser {
			g.state = StateRunning
		}
		g.mu.Unlock()
	})

	// 设置退出回调 (Ctrl+\)
	g.process.SetExitCallback(func() {
		g.Stop()
	})

	// 设置子进程退出回调
	g.process.SetProcessExitCallback(func() {
		g.Stop()
	})

	return g.process.Start()
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
	ticker := time.NewTicker(g.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopChan:
			return nil

		case <-ticker.C:
			g.mu.RLock()
			currentState := g.state
			g.mu.RUnlock()

			if currentState != StateRunning {
				continue
			}

			if !g.process.IsRunning() {
				g.mu.Lock()
				g.state = StateStopped
				g.mu.Unlock()
				return nil
			}

			output := g.process.GetRecentOutput()

			// 检测输出是否变化
			if output != g.lastOutput {
				g.lastOutput = output
				g.lastOutputTime = time.Now()

				// 检测模型选择提示，自动输入配置的模型
				if handled := g.handleModelSelection(output); handled {
					continue
				}
				continue // 输出还在变化，继续等待
			}

			// 输出已稳定，检查是否达到空闲超时
			idleDuration := time.Since(g.lastOutputTime)
			if idleDuration < g.config.IdleTimeout {
				continue // 还未达到空闲超时
			}

			// 检查是否已经对此输出进行过判断
			if output == g.judgedOutput {
				continue // 已经判断过，不重复判断
			}

			// 输出已停止且未判断过，调用AI进行判断
			g.judgedOutput = output // 标记为已判断

			decision, err := g.judge.Decide(output)
			if err != nil {
				continue
			}

			switch decision.Action {
			case "NONE", "WAIT":
				// 无需操作，不记录日志
			case "CLEAR":
				DebugLog("执行: CLEAR - 清理输入框")
				g.cleanInputResidue(output)
				g.judgedOutput = ""
			case "SELECT", "INPUT":
				DebugLog("执行: %s - %s", decision.Action, decision.Content)
				// 先清空输入框残留
				g.cleanInputResidue(output)
				g.process.SendInput(decision.Content)
				// 如果有输入框，发送回车
				if HasInputPrompt(output) {
					time.Sleep(50 * time.Millisecond)
					g.process.SendInput("\r")
					DebugLog("%s: 检测到输入框，发送回车", decision.Action)
				}
				g.mu.Lock()
				g.autoCount++
				g.mu.Unlock()
				g.judgedOutput = ""
			case "HUMAN":
				DebugLog("执行: HUMAN - %s", decision.Content)
				g.mu.Lock()
				g.state = StateWaitingUser
				g.humanCount++
				g.mu.Unlock()
				g.notifier.Notify()
			}

		case input := <-g.userInput:
			g.process.SendInput(input + "\r")
			g.mu.Lock()
			g.state = StateRunning
			g.mu.Unlock()

		case newState := <-g.stateChange:
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
	default:
	}
}

// SetState 设置状态（非阻塞，如果 channel 满则丢弃）
func (g *Guard) SetState(state GuardState) {
	select {
	case g.stateChange <- state:
	case <-g.stopChan:
	default:
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

// cleanInputResidue 清理输入框残留
func (g *Guard) cleanInputResidue(output string) {
	residueInfo := DetectInputResidue(output)
	if !residueInfo.HasResidue {
		return
	}
	for range len(residueInfo.Residue) {
		g.process.SendInput("\x7f") // 退格键 (DEL)
	}
}

// handleModelSelection 检测模型选择提示并自动输入配置的模型
func (g *Guard) handleModelSelection(output string) bool {
	if !g.modelSelector.IsConfigured() || g.modelSelector.IsSelected() {
		return false
	}
	if !g.modelSelector.NeedsSelection(output) {
		return false
	}

	modelNum := g.modelSelector.FindModelNumber(output)
	if modelNum == "" {
		return false
	}

	g.process.SendInput(modelNum + "\r")
	g.mu.Lock()
	g.autoCount++
	g.mu.Unlock()
	g.modelSelector.MarkSelected()
	return true
}
