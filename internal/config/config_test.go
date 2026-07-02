package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SuggestionCommands.TimeoutMS != 2000 {
		t.Errorf("expected default timeout to be 2000, got %d", cfg.SuggestionCommands.TimeoutMS)
	}
	if !cfg.SuggestionCommands.AllowCommands {
		t.Error("expected default allow_commands to be true")
	}
}

func TestLoadConfig_ParseFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pb-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tomlContent := `
[paths]
snippets = ["/tmp/snippets", "./local-snippets"]

[suggestion_commands]
timeout_ms = 5000
allow_commands = false

[variables.git_branch]
command = "git rev-parse --abbrev-ref HEAD"
default = "main"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Paths.Snippets) != 2 || cfg.Paths.Snippets[0] != "/tmp/snippets" {
		t.Errorf("unexpected snippets paths: %v", cfg.Paths.Snippets)
	}

	if cfg.SuggestionCommands.TimeoutMS != 5000 {
		t.Errorf("expected timeout 5000, got %d", cfg.SuggestionCommands.TimeoutMS)
	}

	if cfg.SuggestionCommands.AllowCommands {
		t.Error("expected allow_commands to be false")
	}

	gitBranch, exists := cfg.Variables["git_branch"]
	if !exists {
		t.Fatal("expected git_branch variable to be loaded")
	}

	if gitBranch.Command != "git rev-parse --abbrev-ref HEAD" {
		t.Errorf("unexpected command: %s", gitBranch.Command)
	}

	if gitBranch.Default == nil || *gitBranch.Default != "main" {
		t.Errorf("unexpected default: %v", gitBranch.Default)
	}
}
