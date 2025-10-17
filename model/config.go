package model

import (
	"github.com/spf13/viper"
)

type GitAccount struct {
	Name       string
	Email      string
	SSHPrikey  string
	SSHSignKey string
}

type SSHAccount struct {
	Login  string
	Host   string
	Port   string
	Prikey string
}

func (sa SSHAccount) GetPort() string {
	if sa.Port == "" {
		return "22"
	}
	return sa.Port
}

type Proxy struct {
	Socks string
	Http  string
}

type Config struct {
	Banner  string
	Git     map[string]GitAccount
	SSH     map[string]SSHAccount
	Proxy   map[string]Proxy
	Snippet map[string]string
}

func ReadInConfig(path string) (*Config, bool, error) {
	viper.SetConfigName("nb")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.config/")
	viper.AddConfigPath(".")
	if path != "" {
		viper.SetConfigFile(path)
	}
	err := viper.ReadInConfig()
	if err != nil {
		// Return empty config but allow execution
		return &Config{
			Git:     make(map[string]GitAccount),
			SSH:     make(map[string]SSHAccount),
			Proxy:   make(map[string]Proxy),
			Snippet: make(map[string]string),
		}, false, nil
	}
	var config Config
	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, false, err
	}
	return &config, true, nil
}
