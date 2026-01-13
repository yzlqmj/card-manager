package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	CharactersRootPath   string `json:"charactersRootPath"`
	TavernCharactersPath string `json:"tavernCharactersPath"`
	TavernPublicPath     string `json:"tavernPublicPath"`
	Port                 int    `json:"port"`
	Proxy                string `json:"proxy"`
}

// Load 从 ./config/config.json 加载配置
func Load() (*Config, error) {
	configPath := filepath.Join("config", "config.json")
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}