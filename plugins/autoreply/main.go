package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rubiojr/sup/pkg/plugin"
)

const maxReplyBytes = 4096

type autoreplyPlugin struct{}

// Keep this wire format in sync with the host guard in bot/handlers/autoreply.go.
type settings struct {
	Enabled         bool   `json:"enabled"`
	Scope           string `json:"scope,omitempty"`
	DryRun          bool   `json:"dry_run"`
	CooldownSeconds int64  `json:"cooldown_seconds"`
	TestUnlimited   bool   `json:"test_unlimited"`
}

func loadSettings() (settings, error) {
	cfg := settings{CooldownSeconds: 3600, Scope: "allow-list"}
	s := plugin.Storage()
	// The plugin store doesn't expose a typed not-found error. Listing lets us
	// distinguish an unconfigured plugin from an unavailable store.
	keys, err := s.List("settings")
	if err != nil {
		return cfg, err
	}
	for _, key := range keys {
		if key != "settings" {
			continue
		}
		data, err := s.Get(key)
		if err != nil {
			return cfg, err
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("invalid settings: %w", err)
		}
		break
	}
	if cfg.CooldownSeconds < 1 || cfg.CooldownSeconds > 86400 {
		return cfg, fmt.Errorf("cooldown must be between 1s and 24h")
	}
	if cfg.Scope != "allow-list" && cfg.Scope != "all" {
		return cfg, fmt.Errorf("scope must be allow-list or all")
	}
	return cfg, nil
}

func render(name string) (string, error) {
	data, err := plugin.ReadFile("reply.txt")
	if err != nil {
		return "", fmt.Errorf("read reply.txt in the autoreply plugin data directory: %w", err)
	}
	text := strings.TrimSpace(string(data))
	if text == "" || len(text) > maxReplyBytes || !utf8.ValidString(text) {
		return "", fmt.Errorf("template must be nonempty UTF-8 text of at most %d bytes", maxReplyBytes)
	}
	// One literal substitution, with no template functions or executable logic.
	remaining := strings.ReplaceAll(text, "{{name}}", "")
	if strings.Contains(remaining, "{{") || strings.Contains(remaining, "}}") {
		return "", fmt.Errorf("invalid template placeholder; only {{name}} is supported")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "there"
	}
	if len(name) > 256 || !utf8.ValidString(name) {
		return "", fmt.Errorf("display name must be valid UTF-8 of at most 256 bytes")
	}
	if len(text)+strings.Count(text, "{{name}}")*(len(name)-len("{{name}}")) > maxReplyBytes {
		return "", fmt.Errorf("rendered reply exceeds %d bytes", maxReplyBytes)
	}
	text = strings.ReplaceAll(text, "{{name}}", name)
	if len(text) > maxReplyBytes || !utf8.ValidString(text) {
		return "", fmt.Errorf("rendered reply exceeds %d bytes or contains invalid UTF-8", maxReplyBytes)
	}
	return text, nil
}

func (p *autoreplyPlugin) HandleMessage(input plugin.Input) plugin.Output {
	// Older hosts and renamed copies cannot accidentally bypass the host guard.
	if !input.Info.AutoReplyAllowed {
		return plugin.Error("autoreply requires an updated Sup host and the filename autoreply.wasm")
	}
	if input.Info.IsGroup || (input.Info.IsFromMe && !input.Info.IsSelf) {
		return plugin.Success("")
	}
	if !strings.HasSuffix(input.Sender, "@s.whatsapp.net") && !strings.HasSuffix(input.Sender, "@lid") {
		return plugin.Success("")
	}
	cfg, err := loadSettings()
	if err != nil {
		return plugin.Error(err.Error())
	}
	if !cfg.Enabled {
		return plugin.Success("")
	}
	text, err := render(input.Info.PushName)
	if err != nil {
		return plugin.Error(err.Error())
	}
	return plugin.Success(text)
}

func (p *autoreplyPlugin) HandleCLI(input plugin.CLIInput) plugin.CLIOutput {
	output, err := runCLI(input.Args)
	if err != nil {
		return plugin.CLIOutput{Error: err.Error()}
	}
	return plugin.CLIOutput{Success: true, Output: output}
}

func runCLI(args []string) (string, error) {
	if len(args) == 0 {
		args = []string{"status"}
	}
	if args[0] == "preview" {
		if len(args) > 2 {
			return "", fmt.Errorf("usage: preview [name]")
		}
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		text, err := render(name)
		return text + "\n", err
	}
	cfg, err := loadSettings()
	if err != nil {
		return "", err
	}
	if err := changeSettings(&cfg, args); err != nil {
		return "", err
	}
	if args[0] != "status" {
		data, err := json.Marshal(cfg)
		if err != nil {
			return "", err
		}
		if err := plugin.Storage().Set("settings", data); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("enabled: %t\nscope: %s\ndry-run: %t\ncooldown: %s\ntest-unlimited (self-chat only): %t\n",
		cfg.Enabled, cfg.Scope, cfg.DryRun, time.Duration(cfg.CooldownSeconds)*time.Second, cfg.TestUnlimited), nil
}

func changeSettings(cfg *settings, args []string) error {
	switch args[0] {
	case "status", "enable", "disable":
		if len(args) != 1 {
			return fmt.Errorf("%s takes no arguments", args[0])
		}
		if args[0] == "enable" {
			if _, err := render(""); err != nil {
				return err
			}
			cfg.Enabled = true
		} else if args[0] == "disable" {
			cfg.Enabled = false
		}
	case "dry-run", "test-unlimited":
		if len(args) != 2 || (args[1] != "on" && args[1] != "off") {
			return fmt.Errorf("usage: %s on|off", args[0])
		}
		if args[0] == "dry-run" {
			cfg.DryRun = args[1] == "on"
		} else {
			cfg.TestUnlimited = args[1] == "on"
		}
	case "cooldown":
		if len(args) != 2 {
			return fmt.Errorf("usage: cooldown <duration, 1s to 24h>")
		}
		duration, err := time.ParseDuration(args[1])
		if err != nil || duration < time.Second || duration > 24*time.Hour || duration%time.Second != 0 {
			return fmt.Errorf("cooldown must be a whole number of seconds between 1s and 24h")
		}
		cfg.CooldownSeconds = int64(duration / time.Second)
	case "scope":
		if len(args) != 2 || (args[1] != "allow-list" && args[1] != "all") {
			return fmt.Errorf("usage: scope allow-list|all (direct messages only)")
		}
		cfg.Scope = args[1]
	default:
		return fmt.Errorf("commands: status, enable, disable, preview [name], scope allow-list|all, cooldown <duration>, dry-run on|off, test-unlimited on|off")
	}
	return nil
}

func (p *autoreplyPlugin) Name() string { return "autoreply" }

func (p *autoreplyPlugin) Topics() []string { return []string{"*"} }

func (p *autoreplyPlugin) Version() string { return "0.2.0" }

func (p *autoreplyPlugin) GetRequiredEnvVars() []string { return nil }

func (p *autoreplyPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput("autoreply", "Template replies to all or allow-listed direct messages, including Message yourself",
		"sup plugins run autoreply <status|enable|disable|preview|scope|cooldown|dry-run|test-unlimited>",
		[]string{"sup plugins run autoreply scope all", "sup plugins run autoreply preview", "sup plugins run autoreply enable", "sup plugins run autoreply test-unlimited on"},
		"utility")
}

func init() { plugin.RegisterPlugin(&autoreplyPlugin{}) }

func main() {}
