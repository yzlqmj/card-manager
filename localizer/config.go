package localizer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CliConfig defines the structure for the cli/config.json file.
type CliConfig struct {
	BasePath string `json:"base_path"`
	Proxy    string `json:"proxy"`
}

// LoadCliConfig loads configuration from ./config.json.
func LoadCliConfig() (*CliConfig, error) {
	configPath := filepath.Join("config.json")
	file, err := os.ReadFile(configPath)
	if err != nil {
		// If the file doesn't exist, return a default empty config.
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
