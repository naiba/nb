package ccguard

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/naiba/nb/singleton"
)

type Config struct {
	Policy       string        `yaml:"policy"`
	PollInterval time.Duration `yaml:"poll_interval"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"` // 输出停止后多久触发AI判断
	CCRCommand   string        `yaml:"ccr_command"`
	Model        string        `yaml:"model"`
	Notify       struct {
		Sound bool `yaml:"sound"`
		Bell  bool `yaml:"bell"`
	} `yaml:"notify"`
}

func LoadConfig() *Config {
	cfg := &Config{}

	// 从全局配置加载
	if singleton.Config != nil && singleton.Config.CCGuard != nil {
		cfg.Policy = singleton.Config.CCGuard.Policy
		cfg.PollInterval = singleton.Config.CCGuard.PollInterval
		cfg.CCRCommand = singleton.Config.CCGuard.CCRCommand
		cfg.Model = singleton.Config.CCGuard.Model
		cfg.Notify.Sound = singleton.Config.CCGuard.Notify.Sound
		cfg.Notify.Bell = singleton.Config.CCGuard.Notify.Bell
		DebugLog("LoadConfig: 从全局配置加载成功")
	}

	// 尝试加载项目级配置覆盖
	projectCfg := loadProjectConfig()
	if projectCfg != nil {
		DebugLog("LoadConfig: 加载项目配置成功")
		if projectCfg.Policy != "" {
			cfg.Policy = cfg.Policy + "\n" + projectCfg.Policy
		}
		if projectCfg.PollInterval != 0 {
			cfg.PollInterval = projectCfg.PollInterval
		}
		if projectCfg.CCRCommand != "" {
			cfg.CCRCommand = projectCfg.CCRCommand
		}
		if projectCfg.Model != "" {
			cfg.Model = projectCfg.Model
		}
	}

	// 设置默认值
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultIdleTimeout
	}
	if cfg.CCRCommand == "" {
		cfg.CCRCommand = "ccr"
	}
	if cfg.Policy == "" {
		cfg.Policy = `- 允许所有只读操作（Read, Glob, Grep, 查看型 Bash）
- 禁止删除文件、禁止 rm 命令
- 不确定时询问用户`
	}

	DebugLog("LoadConfig: 最终配置 - Model=%s, PollInterval=%v, CCRCommand=%s",
		cfg.Model, cfg.PollInterval, cfg.CCRCommand)
	return cfg
}

func loadProjectConfig() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		DebugLog("loadProjectConfig: 获取工作目录失败: %v", err)
		return nil
	}

	configPath := filepath.Join(cwd, ".ccguard.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			DebugLog("loadProjectConfig: 读取配置文件失败: %v", err)
		}
		return nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		DebugLog("loadProjectConfig: 解析配置文件失败: %v", err)
		return nil
	}
	return &cfg
}
