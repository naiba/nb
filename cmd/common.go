package cmd

import (
	"fmt"
	"net"
	"strings"

	"github.com/naiba/nb/model"
	"github.com/naiba/nb/singleton"
)

// ProxyConfig holds parsed proxy configuration
type ProxyConfig struct {
	SocksHost string
	SocksPort string
	HttpHost  string
	HttpPort  string
}

// GetProxyConfig retrieves and parses proxy configuration by name
func GetProxyConfig(proxyName string) (*ProxyConfig, error) {
	if proxyName == "" {
		return nil, nil
	}
	if singleton.Config == nil || singleton.Config.Proxy == nil {
		return nil, fmt.Errorf("proxy configuration not available. Please create a config file at ~/.config/nb.yaml")
	}
	server, exists := singleton.Config.Proxy[proxyName]
	if !exists {
		return nil, fmt.Errorf("proxy server not found: %s", proxyName)
	}

	socksHost, socksPort, err := net.SplitHostPort(server.Socks)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy socks address %q: %w", server.Socks, err)
	}

	config := &ProxyConfig{
		SocksHost: socksHost,
		SocksPort: socksPort,
	}

	if server.Http != "" {
		httpHost, httpPort, err := net.SplitHostPort(server.Http)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy http address %q: %w", server.Http, err)
		}
		config.HttpHost = httpHost
		config.HttpPort = httpPort
	}

	return config, nil
}

// GetSSHServerConfig retrieves SSH server configuration by name
func GetSSHServerConfig(sshServerName string) (*model.SSHAccount, error) {
	if sshServerName == "" {
		return nil, nil
	}
	if singleton.Config == nil || singleton.Config.SSH == nil {
		return nil, fmt.Errorf("SSH configuration not available. Please create a config file at ~/.config/nb.yaml")
	}
	server, exists := singleton.Config.SSH[sshServerName]
	if !exists {
		return nil, fmt.Errorf("ssh server not found: %s", sshServerName)
	}
	return &server, nil
}

func GetGitSSHCommandEnv(user string, proxyName string) (*model.GitAccount, []string, error) {
	if user == "" {
		return nil, nil, nil
	}
	if singleton.Config == nil || singleton.Config.Git == nil {
		return nil, nil, fmt.Errorf("git configuration not available. Please create a config file at ~/.config/nb.yaml")
	}
	account, exists := singleton.Config.Git[user]
	if !exists {
		return nil, nil, fmt.Errorf("git user not exists: %s", user)
	}
	if proxyName == "" {
		return &account, []string{
			"GIT_SSH_COMMAND=ssh -i \"" + account.SSHPrikey + "\" -o IdentitiesOnly=yes",
		}, nil
	}

	proxyConfig, err := GetProxyConfig(proxyName)
	if err != nil {
		return nil, nil, err
	}

	return &account, []string{
		fmt.Sprintf("GIT_SSH_COMMAND=ssh -i \"%s\" -o ProxyCommand=\"nc -X 5 -x %s:%s %%h %%p\" -o IdentitiesOnly=yes",
			account.SSHPrikey, proxyConfig.SocksHost, proxyConfig.SocksPort),
	}, nil
}

func ReplaceRemotePath(slice []string, server model.SSHAccount) error {
	var replaced bool
	for i := 0; i < len(slice); i++ {
		if strings.HasPrefix(slice[i], "remote:") {
			slice[i] = strings.Replace(slice[i], "remote:", fmt.Sprintf("%s@%s:", server.Login, server.Host), 1)
			replaced = true
		}
	}
	if !replaced {
		return fmt.Errorf("remote path (remote:) not found in args: %v", slice)
	}
	return nil
}
