package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// Mirror 镜像配置
type Mirror struct {
	Host         string   `json:"host"`
	Capabilities []string `json:"capabilities"`
	SkipVerify   bool     `json:"skip_verify"`
	Header       map[string]string `json:"header,omitempty"`
}

// RegistryConfig Registry 配置
type RegistryConfig struct {
	Prefix  string   `json:"prefix"`
	Mirrors []Mirror `json:"mirrors"`
	Port    int      `json:"port,omitempty"` // 可选端口，为空则使用全局默认
}

// Config 全局配置
type Config struct {
	DefaultPort int              `json:"default_port"`
	TLSCert     string           `json:"tls_cert,omitempty"`
	TLSKey      string           `json:"tls_key,omitempty"`
	LogFile     string           `json:"log_file,omitempty"`
	Registries  []RegistryConfig `json:"registries"`
}

// 默认配置（参考用户需求）
var defaultConfig = Config{
	DefaultPort: 5000,
	Registries: []RegistryConfig{
	{
		Prefix: "docker.io",
		Port:   5000,
		Mirrors: []Mirror{
			{
				Host:         "https://registry-1.docker.io",
				Capabilities: []string{"pull", "resolve"},
				SkipVerify:   false,
			},
		},
	},
	{
		Prefix: "gcr.io",
		Port:   5002,
		Mirrors: []Mirror{
			{
				Host:         "https://gcr.io",
				Capabilities: []string{"pull", "resolve"},
				SkipVerify:   false,
			},
		},
	},
	{
		Prefix: "k8s.gcr.io",
		Port:   5003,
		Mirrors: []Mirror{
			{
				Host:         "https://k8s.gcr.io",
				Capabilities: []string{"pull", "resolve"},
				SkipVerify:   false,
			},
		},
	},
	{
		Prefix: "registry.k8s.io",
		Port:   5004,
		Mirrors: []Mirror{
			{
				Host:         "https://registry.k8s.io",
				Capabilities: []string{"pull", "resolve"},
				SkipVerify:   false,
			},
		},
	},
		{
			Prefix: "quay.io",
			Port:   5001,
			Mirrors: []Mirror{
				{
					Host:         "https://quay.io",
					Capabilities: []string{"pull", "resolve"},
					SkipVerify:   false,
				},
			},
		},
		{
			Prefix: "ghcr.io",
			Port:   5005,
			Mirrors: []Mirror{
				{
					Host:         "https://ghcr.io",
					Capabilities: []string{"pull", "resolve"},
					SkipVerify:   false,
				},
			},
		},
		{
			Prefix: "nvcr.io",
			Port:   5008,
			Mirrors: []Mirror{
				{
					Host:         "https://nvcr.io",
					Capabilities: []string{"pull", "resolve"},
					SkipVerify:   false,
				},
			},
		},
	},
}

// LoadConfig 加载配置（支持 JSON 文件或内置默认配置）
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		log.Println("使用内置默认配置")
		return &defaultConfig, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if cfg.DefaultPort == 0 {
		cfg.DefaultPort = 5000
	}

	return &cfg, nil
}

// GetUpstreamHost 根据前缀获取上游 host
func (c *Config) GetUpstreamHost(prefix string) string {
	prefix = strings.TrimPrefix(prefix, "https://")
	prefix = strings.TrimPrefix(prefix, "http://")
	for _, r := range c.Registries {
		if r.Prefix == prefix {
			if len(r.Mirrors) > 0 {
				return strings.TrimPrefix(strings.TrimPrefix(r.Mirrors[0].Host, "https://"), "http://")
			}
		}
	}
	return "registry-1.docker.io" // 默认 Docker Hub
}

// GetPortForRegistry 获取 registry 对应端口
func (c *Config) GetPortForRegistry(prefix string) int {
	for _, r := range c.Registries {
		if r.Prefix == prefix {
			if r.Port > 0 {
				return r.Port
			}
			return c.DefaultPort
		}
	}
	return c.DefaultPort
}

// GetRegistryByPort 根据端口反查 registry 配置
func (c *Config) GetRegistryByPort(port int) *RegistryConfig {
	for i := range c.Registries {
		rPort := c.Registries[i].Port
		if rPort == 0 {
			rPort = c.DefaultPort
		}
		if rPort == port {
			return &c.Registries[i]
		}
	}
	return nil
}
