package config

import (
	"os"

	"encoding/json"
)

// Config holds all server configuration.
type Config struct {
	Network string `json:"network"`
	Address string `json:"address"`
	Memory  string `json:"memory"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Network: "tcp",
		Address: ":3307",
		Memory:  "64MB",
	}
}

// Load reads a YAML config file. Returns defaults if the file doesn't exist.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // use defaults
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
