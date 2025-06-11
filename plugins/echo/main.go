package main

import (
	"fmt"
	"strings"

	"github.com/rubiojr/sup/pkg/plugin"
)

// EchoPlugin implements the plugin.Plugin interface
type EchoPlugin struct{}

// Name returns the name of the plugin
func (e *EchoPlugin) Name() string {
	return "echo"
}

// Topics returns the topics this plugin should receive messages for
func (e *EchoPlugin) Topics() []string {
	return []string{"echo"}
}

// HandleMessage processes incoming messages
func (e *EchoPlugin) HandleMessage(input plugin.Input) plugin.Output {
	message := strings.TrimSpace(input.Message)

	// Handle empty message
	if message == "" {
		return plugin.Error("Please provide a message to echo. Usage: .sup echo <message>")
	}

	// Handle special commands
	switch strings.ToLower(message) {
	case "reverse":
		return plugin.Success(fmt.Sprintf("🔄 %s", reverse(input.Info.PushName)))
	case "upper":
		return plugin.Success(fmt.Sprintf("📢 %s", strings.ToUpper(input.Info.PushName)))
	case "lower":
		return plugin.Success(fmt.Sprintf("🔇 %s", strings.ToLower(input.Info.PushName)))
	case "info":
		return e.handleInfoCommand(input)
	}

	// Default echo behavior
	prefix := "🔊"
	if input.Info.IsGroup {
		prefix = "📢"
	}

	return plugin.Success(fmt.Sprintf("%s Echo from %s: %s", prefix, input.Info.PushName, message))
}

// handleInfoCommand returns information about the message
func (e *EchoPlugin) handleInfoCommand(input plugin.Input) plugin.Output {
	chatType := "private chat"
	if input.Info.IsGroup {
		chatType = "group chat"
	}

	info := fmt.Sprintf(`ℹ️ Message Info:
👤 Sender: %s
💬 Chat Type: %s
🆔 Message ID: %s
⏰ Timestamp: %d
📧 JID: %s`,
		input.Info.PushName,
		chatType,
		input.Info.ID,
		input.Info.Timestamp,
		input.Sender,
	)

	return plugin.Success(info)
}

// reverse reverses a string
func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// GetHelp returns help information for this plugin
func (e *EchoPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"echo",
		"Echo messages with various formatting options",
		".sup echo <message|command>",
		[]string{
			".sup echo hello world",
			".sup echo reverse",
			".sup echo upper",
			".sup echo lower",
			".sup echo info",
		},
		"utility",
	)
}

// GetRequiredEnvVars returns the environment variables this plugin needs
func (e *EchoPlugin) GetRequiredEnvVars() []string {
	return []string{}
}

// Version returns the version of this plugin
func (e *EchoPlugin) Version() string {
	return "0.1.0"
}

func init() {
	// Register our plugin with the framework
	plugin.RegisterPlugin(&EchoPlugin{})
}

func main() {}
