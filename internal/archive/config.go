// Package archive passively stores WhatsApp conversations and attachments.
package archive

import "fmt"

// Config controls archiving independently of bot command permissions.
type Config struct {
	Enabled       bool   `toml:"enabled"`
	Dir           string `toml:"dir,omitempty"`
	Scope         string `toml:"scope"`
	MaxFileBytes  int64  `toml:"max_file_bytes"`
	MaxMediaBytes int64  `toml:"max_media_bytes"`
	MaxDBBytes    int64  `toml:"max_db_bytes"`
	MaxPending    int    `toml:"max_pending"`
}

// DefaultConfig returns opt-in defaults with bounded disk use and backlog.
func DefaultConfig() Config {
	return Config{
		Scope: "all", MaxFileBytes: 64 << 20, MaxMediaBytes: 10 << 30,
		MaxDBBytes: 256 << 20, MaxPending: 1000,
	}
}

// Validate rejects invalid limits rather than silently disabling them.
func (c Config) Validate() error {
	if c.Scope != "all" && c.Scope != "direct" && c.Scope != "groups" {
		return fmt.Errorf("archive scope must be all, direct, or groups")
	}
	if c.MaxFileBytes < 1 || c.MaxFileBytes > 1<<40 {
		return fmt.Errorf("archive max_file_bytes must be between 1 and 1099511627776")
	}
	if c.MaxMediaBytes < c.MaxFileBytes+32 {
		return fmt.Errorf("archive max_media_bytes must exceed max_file_bytes by at least 32 bytes")
	}
	if c.MaxDBBytes < 1<<20 || c.MaxDBBytes > 1<<40 {
		return fmt.Errorf("archive max_db_bytes must be between 1 MiB and 1 TiB")
	}
	if c.MaxPending < 1 || c.MaxPending > 100000 {
		return fmt.Errorf("archive max_pending must be between 1 and 100000")
	}
	return nil
}
