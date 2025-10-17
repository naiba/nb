package singleton

import (
	"fmt"
	"os"

	"github.com/naiba/nb/model"
)

var Config *model.Config

func Init(confPath string) error {
	var err error
	var configLoaded bool
	Config, configLoaded, err = model.ReadInConfig(confPath)
	if err != nil {
		return err
	}
	if !configLoaded {
		fmt.Fprintln(os.Stderr, "Warning: Config file not found. Using default empty configuration.")
		fmt.Fprintln(os.Stderr, "         Create ~/.config/nb.yaml to configure git accounts, SSH servers, and proxies.")
	}
	return nil
}
