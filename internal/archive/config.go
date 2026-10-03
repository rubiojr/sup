// Package archive passively stores WhatsApp conversations and attachments.
package archive

import "fmt"

// Config controls archiving independently of bot command permissions.
type Config struct {
	Enabled bool   `toml:"enabled"`
	Dir     string `toml:"dir,omitempty"`
	Scope   string `toml:"scope"`
}

// DefaultConfig returns opt-in defaults for all direct and group chats.
func DefaultConfig() Config {
	return Config{Scope: "all"}
}

// Validate checks the archive's message scope.
func (c Config) Validate() error {
	if c.Scope != "all" && c.Scope != "direct" && c.Scope != "groups" {
		return fmt.Errorf("archive scope must be all, direct, or groups")
	}
	return nil
}
