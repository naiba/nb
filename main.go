package main

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/naiba/nb/cmd"
	"github.com/naiba/nb/internal"
)

func main() {
	var killed atomic.Bool
	signalChain := make(chan os.Signal, 1)
	signal.Notify(signalChain, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChain
		if killed.CompareAndSwap(false, true) {
			internal.CleanupChildProcesses(true)
			os.Exit(1)
		}
	}()
	err := cmd.Execute()
	if killed.CompareAndSwap(false, true) {
		internal.CleanupChildProcesses(false)
	}
	if err != nil {
		os.Exit(1)
	}
}
