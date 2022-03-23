package main

import (
	"bytes"
	"flag"
	"os"

	_ "embed"

	"github.com/dimiro1/banner"
)

//go:embed assets/nyancat.txt
var nyancat string

var printBanner bool

func main() {
	flag.BoolVar(&printBanner, "banner", false, "可用在终端启动时打印 banner")
	flag.Parse()

	if printBanner {
		banner.Init(os.Stdout, true, true, bytes.NewBufferString(nyancat))
		return
	}
}
