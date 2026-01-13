package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	
	"gopkg.in/yaml.v3"
)

// 本地化工具配置结构体
type LocalizerConfig struct {
	// 基础路径 - 本地化资源的基础存储路径
	BasePath       string   `yaml:"基础路径" json:"base_path"`
	// 强制代理列表 - 必须通过代理访问的域名列表
	ForceProxyList []string `yaml:"强制代理列表" json:"force_proxy_list"`
}

// 应用配置结构体 - 统一配置，包含主应用和本地化工具的所有配置
type Config struct {
	// 角色卡根目录 - 存放所有角色卡文件的主目录
	CharactersRootPath   string `yaml:"角色卡根目录" json:"charactersRootPath"`
	// 酒馆角色卡目录 - SillyTavern应用中角色卡的存储位置
	TavernCharactersPath string `yaml:"酒馆角色卡目录" json:"tavernCharactersPath"`
	// 酒馆公共目录 - SillyTavern的公共资源目录
	TavernPublicPath     string `yaml:"酒馆公共目录" json:"tavernPublicPath"`
	// 端口号 - 应用程序监听的端口号
	Port                 int    `yaml:"端口" json:"port"`
	// 代理地址 - 网络请求使用的代理服务器地址
	Proxy                string `yaml:"代理地址" json:"proxy"`
	// 本地化工具配置
	Localizer            LocalizerConfig `yaml:"本地化工具" json:"localizer"`
}

// 从 ./config/config.json 加载配置（兼容性支持）
func Load() (*Config, error) {
	// 优先尝试加载YAML配置
	yamlPath := filepath.Join("config", "config.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return LoadFromYAML(yamlPath)
	}
	
	// 回退到JSON配置以保持兼容性
	jsonPath := filepath.Join("config", "config.json")
	file, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

// 从YAML文件加载配置
func LoadFromYAML(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}
// 路径构建器 - 用于动态构建各种子目录路径
type PathBuilder struct {
	// 酒馆公共目录路径
	tavernPublicPath string
}

// 创建新的路径构建器
func NewPathBuilder(tavernPublicPath string) *PathBuilder {
	return &PathBuilder{
		tavernPublicPath: tavernPublicPath,
	}
}

// 构建niko目录路径
func (pb *PathBuilder) BuildNikoPath() string {
	return filepath.Join(pb.tavernPublicPath, "niko")
}

// 从配置创建路径构建器
func (c *Config) NewPathBuilder() *PathBuilder {
	return NewPathBuilder(c.TavernPublicPath)
}