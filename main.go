package main

import (
	"fmt"
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
	defer func() {
		if killed.CompareAndSwap(false, true) {
			internal.CleanupChildProcesses(false)
		}
	}()
	err := cmd.Execute()
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
}
