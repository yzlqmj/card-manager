package localizer

import (
	"encoding/json"
	"os"
	"path/filepath"
	
	"gopkg.in/yaml.v3"
)

// 本地化工具配置结构体
type CliConfig struct {
	// 强制代理列表 - 必须通过代理访问的域名列表
	ForceProxyList []string `yaml:"强制代理列表" json:"force_proxy_list"`
}

// 主配置结构体（简化版，只包含localizer需要的部分）
type MainConfig struct {
	// 酒馆公共目录
	TavernPublicPath string    `yaml:"酒馆公共目录" json:"tavernPublicPath"`
	// 代理地址
	Proxy            string    `yaml:"代理地址" json:"proxy"`
	// 本地化工具配置
	Localizer        CliConfig `yaml:"本地化工具" json:"localizer"`
}

// 从主配置文件加载本地化工具配置
func LoadCliConfig() (*CliConfig, error) {
	// 优先尝试从主配置文件加载
	mainConfigPath := filepath.Join("config", "config.yaml")
	if _, err := os.Stat(mainConfigPath); err == nil {
		config, _, _, err := loadFromMainConfig(mainConfigPath)
		return config, err
	}
	
	// 回退到独立的JSON配置文件（兼容性）
	configPath := filepath.Join("localizer", "config.json")
	file, err := os.ReadFile(configPath)
	if err != nil {
		// 如果文件不存在，返回默认空配置
		if os.IsNotExist(err) {
			return &CliConfig{}, nil
		}
		return nil, err
	}

	var config CliConfig
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// 从主配置文件加载完整配置信息（包括基础路径和代理）
func LoadFullConfig() (*CliConfig, string, string, error) {
	// 优先尝试从主配置文件加载
	mainConfigPath := filepath.Join("config", "config.yaml")
	if _, err := os.Stat(mainConfigPath); err == nil {
		return loadFromMainConfig(mainConfigPath)
	}
	
	// 回退到独立配置文件
	config, err := LoadCliConfig()
	if err != nil {
		return nil, "", "", err
	}
	
	// 对于旧配置，返回空的基础路径和代理
	return config, "", "", nil
}

// 从主配置文件加载本地化工具配置，同时返回基础路径和代理地址
func loadFromMainConfig(configPath string) (*CliConfig, string, string, error) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", "", err
	}
	
	var mainConfig MainConfig
	if err := yaml.Unmarshal(file, &mainConfig); err != nil {
		return nil, "", "", err
	}
	
	// 返回本地化工具配置、基础路径（使用酒馆公共目录）和代理地址
	return &mainConfig.Localizer, mainConfig.TavernPublicPath, mainConfig.Proxy, nil
}
