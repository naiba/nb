package ccguard

import "time"

// Process 进程管理接口，用于抽象 PTY 进程操作
type Process interface {
	Start() error
	Close()
	Wait() error
	IsRunning() bool
	SendInput(text string) error
	GetRecentOutput() string
	SetOutputCallback(cb func([]byte))
	SetToggleCallback(cb func())
	SetUserInputCallback(cb func())
	SetExitCallback(cb func())
	SetProcessExitCallback(cb func())
}

// Decider 决策器接口，用于抽象 AI 决策
type Decider interface {
	Decide(output string) (*Decision, error)
}

// ModelSelectorInterface 模型选择器接口
type ModelSelectorInterface interface {
	IsConfigured() bool
	IsSelected() bool
	MarkSelected()
	Reset()
	FindModelNumber(output string) string
	NeedsSelection(output string) bool
	GetModelName() string
}

// LoggerInterface 日志接口，用于依赖注入
type LoggerInterface interface {
	Log(format string, args ...any)
	LogOutput(label string, content string)
	IsEnabled() bool
}

// Notifier 通知接口，用于跨平台通知
type Notifier interface {
	Bell()
	Sound()
}

// GuardConfig 配置接口
type GuardConfig interface {
	GetPolicy() string
	GetPollInterval() time.Duration
	GetCCRCommand() string
	GetModel() string
	IsBellEnabled() bool
	IsSoundEnabled() bool
}
