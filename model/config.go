package model

import (
	"time"

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

type CCGuardConfig struct {
	Policy       string        `mapstructure:"policy"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
	CCRCommand   string        `mapstructure:"ccr_command"`
	Model        string        `mapstructure:"model"`
	Notify       struct {
		Sound bool `mapstructure:"sound"`
		Bell  bool `mapstructure:"bell"`
	} `mapstructure:"notify"`
}

type Config struct {
	Banner  string
	Git     map[string]GitAccount
	SSH     map[string]SSHAccount
	Proxy   map[string]Proxy
	Snippet map[string]string
	CCGuard *CCGuardConfig `mapstructure:"ccguard"`
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

func (c *CCGuardConfig) GetPollInterval() time.Duration {
	if c == nil || c.PollInterval == 0 {
		return 2 * time.Second
	}
	return c.PollInterval
}

func (c *CCGuardConfig) GetCCRCommand() string {
	if c == nil || c.CCRCommand == "" {
		return "ccr"
	}
	return c.CCRCommand
}

func (c *CCGuardConfig) GetPolicy() string {
	if c == nil || c.Policy == "" {
		return `- 允许所有只读操作（Read, Glob, Grep, 查看型 Bash）
- 禁止删除文件、禁止 rm 命令
- 不确定时询问用户`
	}
	return c.Policy
}
