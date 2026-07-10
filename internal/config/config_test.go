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
	if cfg.Panel.Position != "left" {
		t.Errorf("expected default wizard position to be 'left', got %q", cfg.Panel.Position)
	}
	if cfg.Panel.WidthPercent != 25 {
		t.Errorf("expected default wizard width_percent to be 25, got %d", cfg.Panel.WidthPercent)
	}
	if cfg.Panel.HeightPercent != 100 {
		t.Errorf("expected default wizard height_percent to be 100, got %d", cfg.Panel.HeightPercent)
	}
	if !cfg.Output.Stdout {
		t.Error("expected default output.stdout to be true")
	}
	if !cfg.Output.Clipboard {
		t.Error("expected default output.clipboard to be true")
	}
	if cfg.Behavior.AutoCopyOnSelect {
		t.Error("expected default behavior.auto_copy_on_select to be false")
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

[panel]
position = "bottom"
width_percent = 70
height_percent = 50

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

	if cfg.Panel.Position != "bottom" {
		t.Errorf("expected wizard position 'bottom', got %q", cfg.Panel.Position)
	}
	if cfg.Panel.WidthPercent != 70 {
		t.Errorf("expected wizard width_percent 70, got %d", cfg.Panel.WidthPercent)
	}
	if cfg.Panel.HeightPercent != 50 {
		t.Errorf("expected wizard height_percent 50, got %d", cfg.Panel.HeightPercent)
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

func TestLoadConfig_WizardDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pb-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tomlContent := `
[paths]
snippets = ["/tmp/snippets"]
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Panel.Position != "left" {
		t.Errorf("expected default wizard position 'left', got %q", cfg.Panel.Position)
	}
	if cfg.Panel.WidthPercent != 25 {
		t.Errorf("expected default wizard width_percent 25, got %d", cfg.Panel.WidthPercent)
	}
	if cfg.Panel.HeightPercent != 100 {
		t.Errorf("expected default wizard height_percent 100, got %d", cfg.Panel.HeightPercent)
	}
}
