//go:build windows

package ccguard

import (
	"errors"
	"sync"
)

var errNotSupported = errors.New("PTY is not supported on Windows")

type PTYProcess struct {
	output   *RingBuffer
	mu       sync.RWMutex
	done     chan struct{}
	doneOnce sync.Once
}

func NewPTYProcess(command string, args ...string) *PTYProcess {
	return &PTYProcess{
		output: NewRingBuffer(DefaultRingBufferSize),
		done:   make(chan struct{}),
	}
}

func (p *PTYProcess) SetOutputCallback(cb func([]byte)) {}

func (p *PTYProcess) SetRawMode(raw bool) {}

func (p *PTYProcess) SetToggleCallback(cb func()) {}

func (p *PTYProcess) SetUserInputCallback(cb func()) {}

func (p *PTYProcess) SetExitCallback(cb func()) {}

func (p *PTYProcess) SetProcessExitCallback(cb func()) {}

func (p *PTYProcess) Start() error {
	return errNotSupported
}

func (p *PTYProcess) SendInput(text string) error {
	return errNotSupported
}

func (p *PTYProcess) GetRecentOutput() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.output.String()
}

func (p *PTYProcess) Wait() error {
	return errNotSupported
}

func (p *PTYProcess) Close() {
	p.doneOnce.Do(func() {
		close(p.done)
	})
}

func (p *PTYProcess) IsRunning() bool {
	return false
}
