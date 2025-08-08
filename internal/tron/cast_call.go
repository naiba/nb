package tron

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/naiba/nb/internal"
)

func CastCall(rpc string, overrideCode []string, otherArgs []string) error {
	var overrideCodeMap sync.Map
	if len(overrideCode) > 0 {
		for _, arg := range overrideCode {
			parts := strings.Split(arg, ":")
			if len(parts) == 2 {
				overrideCodeMap.Store(strings.ToLower(parts[0]), parts[1])
				continue
			}
			return fmt.Errorf("invalid override code: %s", arg)
		}
	}

	targetURL, err := url.Parse(rpc)
	if err != nil {
		return fmt.Errorf("invalid RPC URL: %v", err)
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	handler := &proxyHandler{
		target:          targetURL,
		overrideCodeMap: &overrideCodeMap,
	}

	server := &http.Server{
		Handler: handler,
	}

	var finalError error
	closeCh := make(chan struct{})
	var closeOnce sync.Once

	go func() {
		err := server.Serve(listener)
		if err != nil {
			finalError = err
		}
		closeOnce.Do(func() { close(closeCh) })
	}()

	time.Sleep(time.Second)

	go func() {
		finalError = internal.ExecuteInHost(nil, "cast", append([]string{
			"call",
			"--rpc-url",
			fmt.Sprintf("http://localhost:%d%s", port, targetURL.Path),
		}, otherArgs...)...)
		closeOnce.Do(func() { close(closeCh) })
	}()

	<-closeCh
	return finalError
}
