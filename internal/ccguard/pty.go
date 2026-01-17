//go:build !windows

package ccguard

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

type PTYProcess struct {
	cmd           *exec.Cmd
	pty           *os.File
	output        *RingBuffer
	mu            sync.RWMutex
	onOutput      func([]byte) // 输出回调，用于TUI显示
	rawMode       bool         // 是否直接输出到stdout（非TUI模式）
	oldState      *term.State  // 终端原始状态
	onToggle      func()       // 暂停/恢复切换回调
	onUserInput   func()       // 用户输入回调（用于人工介入后恢复）
	onExit        func()       // 退出回调 (Ctrl+\)
	onProcessExit func()       // 子进程退出回调

	// goroutine 生命周期管理
	wg         sync.WaitGroup
	closeChan  chan struct{} // 通知所有 goroutine 退出
	closeOnce  sync.Once     // 确保 closeChan 只关闭一次
	doneOnce   sync.Once     // 确保 done channel 只关闭一次
	done       chan struct{} // 子进程退出信号
	resizeChan chan os.Signal // resize 信号 channel，用于清理
}

func NewPTYProcess(command string, args ...string) *PTYProcess {
	return &PTYProcess{
		cmd:        exec.Command(command, args...),
		output:     NewRingBuffer(DefaultRingBufferSize),
		rawMode:    true, // 默认直接输出到stdout
		done:       make(chan struct{}),
		closeChan:  make(chan struct{}),
		resizeChan: make(chan os.Signal, 1),
	}
}

// SetOutputCallback 设置输出回调（用于TUI模式）
func (p *PTYProcess) SetOutputCallback(cb func([]byte)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onOutput = cb
	p.rawMode = false // 使用回调时不直接输出到stdout
}

// SetRawMode 设置是否直接输出到stdout
func (p *PTYProcess) SetRawMode(raw bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rawMode = raw
}

// SetToggleCallback 设置暂停/恢复切换回调 (Ctrl+G触发)
func (p *PTYProcess) SetToggleCallback(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onToggle = cb
}

// SetUserInputCallback 设置用户输入回调
func (p *PTYProcess) SetUserInputCallback(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onUserInput = cb
}

// SetExitCallback 设置退出回调 (Ctrl+\触发)
func (p *PTYProcess) SetExitCallback(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onExit = cb
}

// SetProcessExitCallback 设置子进程退出回调
func (p *PTYProcess) SetProcessExitCallback(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onProcessExit = cb
}

func (p *PTYProcess) Start() error {
	DebugLog("PTYProcess.Start(): 启动子进程")
	var err error
	p.pty, err = pty.Start(p.cmd)
	if err != nil {
		DebugLog("PTYProcess.Start(): 启动失败: %v", err)
		return err
	}

	// 设置终端为raw模式以便正确传递按键
	p.oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		p.pty.Close()
		DebugLog("PTYProcess.Start(): 设置 raw 模式失败: %v", err)
		return fmt.Errorf("failed to set raw mode: %w", err)
	}

	// 同步PTY大小
	if ws, err := pty.GetsizeFull(os.Stdin); err == nil {
		pty.Setsize(p.pty, ws)
		DebugLog("PTYProcess.Start(): 终端大小 %dx%d", ws.Cols, ws.Rows)
	}

	// 处理终端大小变化
	p.wg.Add(1)
	go p.handleResize()

	// 后台读取输出
	p.wg.Add(1)
	go p.readLoop()

	// 后台转发stdin到PTY
	p.wg.Add(1)
	go p.inputLoop()

	DebugLog("PTYProcess.Start(): 子进程启动成功")
	return nil
}

func (p *PTYProcess) handleResize() {
	defer p.wg.Done()

	signal.Notify(p.resizeChan, syscall.SIGWINCH)
	defer signal.Stop(p.resizeChan) // 清理信号注册

	for {
		select {
		case <-p.closeChan:
			DebugLog("PTYProcess.handleResize(): 收到关闭信号，退出")
			return
		case <-p.resizeChan:
			if ws, err := pty.GetsizeFull(os.Stdin); err == nil {
				p.mu.RLock()
				ptyFile := p.pty
				p.mu.RUnlock()
				if ptyFile != nil {
					pty.Setsize(ptyFile, ws)
				}
			}
		}
	}
}

func (p *PTYProcess) inputLoop() {
	defer p.wg.Done()

	buf := make([]byte, DefaultInputBufSize)
	for {
		// 检查是否应该退出
		select {
		case <-p.closeChan:
			DebugLog("PTYProcess.inputLoop(): 收到关闭信号，退出")
			return
		default:
		}

		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}

		// 检查 Ctrl+\ (ASCII 28) - 退出
		if n == 1 && buf[0] == KeyCtrlBackslash {
			p.mu.RLock()
			exitCb := p.onExit
			p.mu.RUnlock()
			if exitCb != nil {
				exitCb()
			}
			return
		}

		// 检查 Ctrl+G (ASCII 7) - 暂停/恢复切换
		if n == 1 && buf[0] == KeyCtrlG {
			p.mu.RLock()
			toggle := p.onToggle
			p.mu.RUnlock()
			if toggle != nil {
				toggle()
			}
			continue // 不转发这个按键
		}

		// 通知有用户输入
		p.mu.RLock()
		userInput := p.onUserInput
		p.mu.RUnlock()
		if userInput != nil {
			userInput()
		}

		// 转发到PTY，检查错误
		p.mu.RLock()
		ptyFile := p.pty
		p.mu.RUnlock()
		if ptyFile != nil {
			if _, err := ptyFile.Write(buf[:n]); err != nil {
				DebugLog("PTYProcess.inputLoop(): PTY 写入错误: %v", err)
				return
			}
		}
	}
}

func (p *PTYProcess) readLoop() {
	defer p.wg.Done()

	buf := make([]byte, DefaultReadBufSize)
	for {
		// 获取 pty 文件句柄（加锁保护）
		p.mu.RLock()
		ptyFile := p.pty
		p.mu.RUnlock()

		if ptyFile == nil {
			DebugLog("PTYProcess.readLoop(): PTY 已关闭，退出")
			return
		}

		n, err := ptyFile.Read(buf)
		if err != nil {
			// 子进程退出，关闭 done channel 通知其他 goroutine（使用 sync.Once 保证只关闭一次）
			DebugLog("PTYProcess.readLoop(): PTY 读取结束: %v", err)
			p.doneOnce.Do(func() {
				close(p.done)
			})
			// 调用进程退出回调
			p.mu.RLock()
			exitCb := p.onProcessExit
			p.mu.RUnlock()
			if exitCb != nil {
				exitCb()
			}
			return
		}
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			p.mu.Lock()
			p.output.Write(data)
			rawMode := p.rawMode
			onOutput := p.onOutput
			p.mu.Unlock()

			// 根据模式处理输出
			if rawMode {
				os.Stdout.Write(data)
			}
			if onOutput != nil {
				onOutput(data)
			}
		}
	}
}

func (p *PTYProcess) SendInput(text string) error {
	p.mu.RLock()
	ptyFile := p.pty
	p.mu.RUnlock()

	if ptyFile == nil {
		return fmt.Errorf("pty not started")
	}
	DebugLog("PTYProcess.SendInput(): 发送输入: %q", text)
	_, err := ptyFile.WriteString(text)
	return err
}

func (p *PTYProcess) GetRecentOutput() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.output.String()
}

func (p *PTYProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *PTYProcess) Close() {
	DebugLog("PTYProcess.Close(): 开始关闭")

	// 关闭 closeChan 通知所有 goroutine 退出（只执行一次）
	p.closeOnce.Do(func() {
		close(p.closeChan)
	})

	// 恢复终端状态
	if p.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), p.oldState)
	}

	// 终止子进程
	if p.cmd != nil && p.cmd.Process != nil {
		// 先发送 SIGTERM 让进程优雅退出
		p.cmd.Process.Signal(syscall.SIGTERM)
	}

	p.mu.Lock()
	if p.pty != nil {
		p.pty.Close()
		p.pty = nil
	}
	p.mu.Unlock()

	// 等待进程退出，避免僵尸进程
	if p.cmd != nil {
		p.cmd.Wait()
	}

	// 等待所有 goroutine 退出
	p.wg.Wait()
	DebugLog("PTYProcess.Close(): 关闭完成")
}

func (p *PTYProcess) IsRunning() bool {
	if p.cmd == nil {
		return false
	}
	return p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited()
}
