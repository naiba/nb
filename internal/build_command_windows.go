//go:build windows
// +build windows

package internal

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func BuildCommand(env []string, name string, args ...string) *exec.Cmd {
	command := exec.Command(name, args...)

	// 使用进程组并注册退出处理
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		ParentProcess: syscall.Handle(0), // 使用当前进程作为父进程
	}

	// 设置环境和标准输入输出
	command.Env = append(os.Environ(), env...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command
}

// 在主程序退出前调用此函数
func CleanupChildProcesses(isSignal bool) {
	// 向所有子进程发送终止信号
	exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", os.Getpid())).Run()
}
