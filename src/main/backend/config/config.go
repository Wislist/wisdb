package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all server configuration.
type Config struct {
	Network string `yaml:"network"`
	Address string `yaml:"address"`
	Memory  string `yaml:"memory"`
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

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
