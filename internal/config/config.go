package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"peanutclone/internal/domain"
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

// Config represents the application configuration format.
type Config struct {
	Paths              PathsConfig                     `toml:"paths"`
	SuggestionCommands SuggestionCommandsConfig        `toml:"suggestion_commands"`
	Variables          map[string]domain.VariableSpec `toml:"variables"`
}

// DefaultConfig returns a Config initialized with default fallback settings.
func DefaultConfig() *Config {
	return &Config{
		SuggestionCommands: SuggestionCommandsConfig{
			TimeoutMS:     2000,
			AllowCommands: true,
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
			path = filepath.Join(home, ".config", "peanutbutter", "config.toml")
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

	return cfg, nil
}
