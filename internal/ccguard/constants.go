package ccguard

import "time"

// GuardState 守卫状态
type GuardState int

const (
	StateRunning GuardState = iota
	StateWaitingUser
	StatePaused
	StateStopped
)

// String 返回状态的字符串表示
func (s GuardState) String() string {
	switch s {
	case StateRunning:
		return "运行中"
	case StateWaitingUser:
		return "等待用户"
	case StatePaused:
		return "已暂停"
	case StateStopped:
		return "已停止"
	default:
		return "未知"
	}
}

const (
	// 缓冲区大小
	DefaultRingBufferSize = 64 * 1024 // 64KB
	DefaultReadBufSize    = 4096
	DefaultInputBufSize   = 1024

	// 超时设置
	DefaultJudgeTimeout = 60 * time.Second
	DefaultPollInterval = 2 * time.Second

	// 日志设置
	DefaultLogMaxLen = 2000

	// Channel 缓冲大小
	DefaultChannelBuffer = 1

	// 特殊按键 ASCII 码
	KeyCtrlBackslash = 28 // Ctrl+\ 退出
	KeyCtrlG         = 7  // Ctrl+G 暂停/恢复
)
