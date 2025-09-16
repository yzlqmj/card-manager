package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 结构对应于 config.json 文件的内容
type Config struct {
	CharactersRootPath   string `json:"charactersRootPath"`
	TavernCharactersPath string `json:"tavernCharactersPath"`
	Port                 int    `json:"port"`
	Proxy                string `json:"proxy"`
}

var config Config

// loadConfig 从 ./config/config.json 加载配置
func loadConfig() error {
	configPath := filepath.Join("config", "config.json")
	file, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(file, &config)
}
