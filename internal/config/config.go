package config

import (
	"fmt"
	"os"
	"path/filepath"

	"basin/internal/domain"
	"github.com/pelletier/go-toml/v2"
)

// PathsConfig holds the list of paths where snippets are stored.
type PathsConfig struct {
	Snippets []string `toml:"snippets"`
}

// SuggestionCommandsConfig holds settings for executing shell commands within variables.
type SuggestionCommandsConfig struct {
	TimeoutMS     int  `toml:"timeout_ms"`
	AllowCommands bool `toml:"allow_commands"`
}

// PanelConfig holds settings for panel position and sizing across the whole app.
type PanelConfig struct {
	Position      string `toml:"position"`
	WidthPercent  int    `toml:"width_percent"`
	HeightPercent int    `toml:"height_percent"`
}

// OutputConfig controls what happens when a strand is completed.
type OutputConfig struct {
	Stdout    bool `toml:"stdout"`
	Clipboard bool `toml:"clipboard"`
}

// BehaviorConfig controls interactive behavior settings.
type BehaviorConfig struct {
	AutoCopyOnSelect bool `toml:"auto_copy_on_select"`
}

// Config represents the application configuration format.
type Config struct {
	Paths              PathsConfig                    `toml:"paths"`
	SuggestionCommands SuggestionCommandsConfig       `toml:"suggestion_commands"`
	Panel              PanelConfig                    `toml:"panel"`
	Output             OutputConfig                   `toml:"output"`
	Behavior           BehaviorConfig                 `toml:"behavior"`
	Variables          map[string]domain.VariableSpec `toml:"variables"`
}

// DefaultConfig returns a Config initialized with default fallback settings.
func DefaultConfig() *Config {
	return &Config{
		SuggestionCommands: SuggestionCommandsConfig{
			TimeoutMS:     2000,
			AllowCommands: true,
		},
		Panel: PanelConfig{
			Position:      "left",
			WidthPercent:  25,
			HeightPercent: 100,
		},
		Output: OutputConfig{
			Stdout:    true,
			Clipboard: true,
		},
		Behavior: BehaviorConfig{
			AutoCopyOnSelect: false,
		},
		Variables: make(map[string]domain.VariableSpec),
	}
}

// LoadConfig reads the TOML configuration from the specified path or returns defaults if it doesn't exist.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".config", "basin", "config.toml")
		}
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config TOML: %w", err)
	}

	// Apply default values if omitted or 0
	if cfg.SuggestionCommands.TimeoutMS <= 0 {
		cfg.SuggestionCommands.TimeoutMS = 2000
	}

	if cfg.Panel.Position == "" {
		cfg.Panel.Position = "left"
	}
	if cfg.Panel.WidthPercent <= 0 || cfg.Panel.WidthPercent > 100 {
		cfg.Panel.WidthPercent = 25
	}
	if cfg.Panel.HeightPercent <= 0 || cfg.Panel.HeightPercent > 100 {
		cfg.Panel.HeightPercent = 100
	}

	return cfg, nil
}
