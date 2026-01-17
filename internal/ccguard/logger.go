package ccguard

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Logger 调试日志记录器
type Logger struct {
	mu      sync.Mutex
	file    *os.File
	enabled bool
}

// 全局日志实例（可通过 SetLogger 替换，便于测试）
var debugLogger *Logger

// InitLogger 初始化调试日志记录器
func InitLogger(logPath string) error {
	if logPath == "" {
		logPath = "ccguard.log"
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %w", err)
	}

	debugLogger = &Logger{
		file:    file,
		enabled: true,
	}

	debugLogger.Log("========== CCGuard 启动 ==========")
	return nil
}

// SetLogger 设置自定义日志器（用于测试）
func SetLogger(logger *Logger) {
	debugLogger = logger
}

// GetLogger 获取当前日志器
func GetLogger() *Logger {
	return debugLogger
}

// CloseLogger 关闭日志记录器
func CloseLogger() {
	if debugLogger != nil && debugLogger.file != nil {
		debugLogger.Log("========== CCGuard 结束 ==========")
		debugLogger.file.Close()
		debugLogger = nil
	}
}

// Log 记录日志
func (l *Logger) Log(format string, args ...interface{}) {
	if l == nil || !l.enabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.file, "[%s] %s\n", timestamp, msg)
	l.file.Sync()
}

// LogOutput 记录输出内容（清理ANSI颜色代码）
func (l *Logger) LogOutput(label string, content string) {
	if l == nil || !l.enabled {
		return
	}
	// 清理 ANSI 颜色代码
	content = ansiRegex.ReplaceAllString(content, "")
	l.Log("%s:\n%s", label, content)
}

// IsEnabled 检查日志是否启用
func (l *Logger) IsEnabled() bool {
	return l != nil && l.enabled
}

// DebugLog 全局调试日志函数
func DebugLog(format string, args ...interface{}) {
	if debugLogger != nil {
		debugLogger.Log(format, args...)
	}
}

// DebugLogOutput 记录输出内容（截断过长内容）
func DebugLogOutput(label string, content string) {
	if debugLogger != nil {
		debugLogger.LogOutput(label, content)
	}
}

// IsDebugEnabled 检查调试模式是否启用
func IsDebugEnabled() bool {
	return debugLogger != nil && debugLogger.enabled
}

// NullLogger 空日志器（用于测试）
type NullLogger struct{}

func (l *NullLogger) Log(format string, args ...interface{})       {}
func (l *NullLogger) LogOutput(label string, content string)       {}
func (l *NullLogger) IsEnabled() bool                              { return false }
