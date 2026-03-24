package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM struct {
		Provider string `yaml:"provider"` // e.g., "gemini"
		Model    string `yaml:"model"`    // e.g., "gemini-1.5-pro-latest"
		APIKey   string `yaml:"api_key"`  // Can be empty if using env var
	} `yaml:"llm"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if no file exists
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.LLM.Provider = "gemini"
	cfg.LLM.Model = "gemini-1.5-flash"
	return cfg
}
