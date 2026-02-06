package config

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/rubiojr/sup/internal/botfs"
)

// Config represents the bot configuration.
type Config struct {
	Trigger  string         `toml:"trigger"`
	LogLevel string         `toml:"log_level"`
	Allow    Allow          `toml:"allow"`
	Agendalo AgendaloConfig `toml:"agendalo"`
	Plugins  PluginsConfig  `toml:"plugins"`
}

// AgendaloConfig holds settings for the agendalo handler.
type AgendaloConfig struct {
	// Command is the external command (with optional args) that reads text
	// from stdin and returns JSON events.
	Command string `toml:"command"`
	// RateLimit is the max number of add interactions per sender per hour.
	RateLimit int `toml:"rate_limit"`
}

// PluginsConfig holds settings for the WASM plugin system.
type PluginsConfig struct {
	// AllowedCommands is a whitelist of commands that WASM plugins can execute.
	AllowedCommands []string `toml:"allowed_commands"`
}

// AllowEntry represents an allowed JID with an optional display name.
type AllowEntry struct {
	JID  string `toml:"jid"`
	Name string `toml:"name,omitempty"`
}

// Allow defines the allow lists for groups and users.
// Empty lists mean deny all.
type Allow struct {
	Groups []AllowEntry `toml:"groups"`
	Users  []AllowEntry `toml:"users"`
}

// GroupJIDs returns the JID strings from the groups allow list.
func (a *Allow) GroupJIDs() []string {
	jids := make([]string, len(a.Groups))
	for i, g := range a.Groups {
		jids[i] = g.JID
	}
	return jids
}

// UserJIDs returns the JID strings from the users allow list.
func (a *Allow) UserJIDs() []string {
	jids := make([]string, len(a.Users))
	for i, u := range a.Users {
		jids[i] = u.JID
	}
	return jids
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	return filepath.Join(botfs.ConfigDir(), "bot.toml")
}

// Load reads and parses a TOML config file.
// Returns a default Config if the file does not exist.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Trigger:  ".sup",
		LogLevel: "info",
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Save writes the config to the given path in TOML format.
// Parent directories are created if they don't exist.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
