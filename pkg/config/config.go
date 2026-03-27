package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PlansDir string `yaml:"plans_dir"`
	LLM      struct {
		Provider string `yaml:"provider"`
		Model    string `yaml:"model"`
		APIKey   string `yaml:"api_key"`
	} `yaml:"llm"`
	Atlassian struct {
		BaseURL string `yaml:"base_url"`
		User    string `yaml:"user"`
		APIKey  string `yaml:"api_key"`
	} `yaml:"atlassian"`
	MaxConcurrency int `yaml:"max_concurrency"`
	MaxRetries     int `yaml:"max_retries"`
}

// DefaultPath returns the default location for the config file: ~/.planner/config.yml
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".planner/config.yml" // fallback
	}
	return filepath.Join(home, ".planner", "config.yml")
}

// expandTilde handles resolving the ~ symbol in file paths
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func LoadConfig(path string) (*Config, error) {
	expandedPath := expandTilde(path)
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if no file exists
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file at %s: %w", expandedPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file at %s: %w", expandedPath, err)
	}

	if cfg.PlansDir == "" {
		cfg.PlansDir = "~/.planner/plans"
	}

	cfg.PlansDir = expandTilde(cfg.PlansDir)

	// Load Atlassian config from environment variables if not set in config file
	if cfg.Atlassian.BaseURL == "" {
		cfg.Atlassian.BaseURL = os.Getenv("PLANNER_ATLASSIAN_BASE_URL")
	}
	if cfg.Atlassian.User == "" {
		cfg.Atlassian.User = os.Getenv("PLANNER_ATLASSIAN_API_USER")
	}
	if cfg.Atlassian.APIKey == "" {
		cfg.Atlassian.APIKey = os.Getenv("PLANNER_ATLASSIAN_API_KEY")
	}

	return &cfg, nil
}

func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.PlansDir = expandTilde("~/.planner/plans")
	cfg.LLM.Provider = "gemini"
	cfg.LLM.Model = "gemini-3.1-flash-lite-preview"
	return cfg
}
