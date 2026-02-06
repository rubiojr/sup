package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	cfg, err := Load("/tmp/does-not-exist-sup-test.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Trigger != ".sup" {
		t.Errorf("expected default trigger .sup, got %s", cfg.Trigger)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log_level info, got %s", cfg.LogLevel)
	}
	if len(cfg.Allow.Groups) != 0 {
		t.Errorf("expected empty groups, got %v", cfg.Allow.Groups)
	}
	if len(cfg.Allow.Users) != 0 {
		t.Errorf("expected empty users, got %v", cfg.Allow.Users)
	}
}

func TestLoadValid(t *testing.T) {
	content := `
trigger = "!bot"
log_level = "debug"

[[allow.groups]]
jid = "120363001234@g.us"
name = "My Group"

[[allow.users]]
jid = "1234567890@s.whatsapp.net"
name = "Alice"

[[allow.users]]
jid = "9876543210@s.whatsapp.net"
name = "Bob"
`
	path := filepath.Join(t.TempDir(), "bot.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Trigger != "!bot" {
		t.Errorf("expected trigger !bot, got %s", cfg.Trigger)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level debug, got %s", cfg.LogLevel)
	}
	if len(cfg.Allow.Groups) != 1 || cfg.Allow.Groups[0].JID != "120363001234@g.us" {
		t.Errorf("unexpected groups: %v", cfg.Allow.Groups)
	}
	if cfg.Allow.Groups[0].Name != "My Group" {
		t.Errorf("expected group name 'My Group', got %s", cfg.Allow.Groups[0].Name)
	}
	if len(cfg.Allow.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(cfg.Allow.Users))
	}
	if cfg.Allow.Users[0].Name != "Alice" {
		t.Errorf("expected user name 'Alice', got %s", cfg.Allow.Users[0].Name)
	}

	// Test JID helpers
	groupJIDs := cfg.Allow.GroupJIDs()
	if len(groupJIDs) != 1 || groupJIDs[0] != "120363001234@g.us" {
		t.Errorf("unexpected GroupJIDs: %v", groupJIDs)
	}
	userJIDs := cfg.Allow.UserJIDs()
	if len(userJIDs) != 2 {
		t.Errorf("expected 2 UserJIDs, got %d", len(userJIDs))
	}
}

func TestLoadInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bot.toml")
	if err := os.WriteFile(path, []byte("not valid [[ toml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadPartial(t *testing.T) {
	content := `
[[allow.users]]
jid = "1234567890@s.whatsapp.net"
`
	path := filepath.Join(t.TempDir(), "bot.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Defaults preserved for unset fields
	if cfg.Trigger != ".sup" {
		t.Errorf("expected default trigger, got %s", cfg.Trigger)
	}
	if len(cfg.Allow.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(cfg.Allow.Users))
	}
	if len(cfg.Allow.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(cfg.Allow.Groups))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "bot.toml")
	cfg := &Config{
		Trigger:  ".bot",
		LogLevel: "debug",
		Allow: Allow{
			Groups: []AllowEntry{{JID: "group@g.us", Name: "Test Group"}},
			Users:  []AllowEntry{{JID: "user@s.whatsapp.net", Name: "Test User"}},
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Allow.Groups) != 1 || loaded.Allow.Groups[0].Name != "Test Group" {
		t.Errorf("unexpected groups after round-trip: %v", loaded.Allow.Groups)
	}
	if len(loaded.Allow.Users) != 1 || loaded.Allow.Users[0].Name != "Test User" {
		t.Errorf("unexpected users after round-trip: %v", loaded.Allow.Users)
	}
}
