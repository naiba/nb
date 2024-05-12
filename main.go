package main

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/naiba/nb/cmd"
)

func main() {
	pid := os.Getpid()
	gid, err := syscall.Getpgid(pid)
	if err != nil {
		panic(err)
	}
	var killed atomic.Bool
	signalChain := make(chan os.Signal, 1)
	signal.Notify(signalChain, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalChain
		if killed.CompareAndSwap(false, true) {
			if pid == gid {
				syscall.Kill(-gid, syscall.SIGTERM)
			}
			os.Exit(1)
		}
	}()
	err = cmd.Execute()
	if killed.CompareAndSwap(false, true) {
		syscall.Kill(-gid, syscall.SIGTERM)
	}
	if err != nil {
		os.Exit(1)
	}
}
