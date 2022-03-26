package singleton

import (
	"github.com/naiba/nb/model"
)

var Config *model.Config

func init() {

}

func Init(confPath string) error {
	var err error
	Config, err = model.ReadInConfig(confPath)
	return err
}
