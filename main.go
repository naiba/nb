package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/naiba/nb/cmd"
)

func main() {
	pid := os.Getpid()
	gid, err := syscall.Getpgid(pid)
	if err != nil {
		panic(err)
	}

	var suicide bool

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		err = cmd.Execute()
		suicide = true
		syscall.Kill(-gid, syscall.SIGTERM)
	}()

	<-stop
	if !suicide {
		syscall.Kill(-gid, syscall.SIGTERM)
	}
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
