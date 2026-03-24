package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadDefault(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LLM.Provider != "gemini" {
		t.Errorf("Expected default provider gemini, got %s", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "gemini-3.1-flash-lite-preview" {
		t.Errorf("Expected default model gemini-3.1-flash-lite-preview, got %s", cfg.LLM.Model)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		expectedPlansDir := filepath.Join(home, ".planner", "plans")
		if cfg.PlansDir != expectedPlansDir {
			t.Errorf("Expected default plans dir %s, got %s", expectedPlansDir, cfg.PlansDir)
		}
	}
}

func TestConfigExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot resolve home directory")
	}

	expanded := expandTilde("~/foo/bar")
	expected := filepath.Join(home, "foo/bar")

	if expanded != expected {
		t.Errorf("Expected %s, got %s", expected, expanded)
	}

	notTilde := expandTilde("/foo/bar")
	if notTilde != "/foo/bar" {
		t.Errorf("Expected /foo/bar, got %s", notTilde)
	}
}

func TestConfigLoadMissingFile(t *testing.T) {
	cfg, err := LoadConfig("/path/to/nonexistent/file.yml")
	if err != nil {
		t.Errorf("Expected no error when loading missing config file, got %v", err)
	}

	if cfg.LLM.Provider != "gemini" {
		t.Errorf("Expected fallback default config")
	}
}

func TestConfigLoadValidFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `
plans_dir: "~/custom/plans"
llm:
  provider: mock
  model: some-model
  api_key: secret
`
	configPath := filepath.Join(tempDir, "config.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.LLM.Provider != "mock" {
		t.Errorf("Expected provider mock, got %s", cfg.LLM.Provider)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		expectedPlansDir := filepath.Join(home, "custom", "plans")
		if cfg.PlansDir != expectedPlansDir {
			t.Errorf("Expected expanded plans dir %s, got %s", expectedPlansDir, cfg.PlansDir)
		}
	}
}
