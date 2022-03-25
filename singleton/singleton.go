package singleton

import (
	"os"

	"github.com/naiba/nb/model"
)

var Config *model.Config

func init() {
	var err error
	Config, err = model.ReadInConfig(os.Getenv("CONFIG_PATH"))
	if err != nil {
		panic(err)
	}
}
