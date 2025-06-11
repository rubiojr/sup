package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/rubiojr/sup/pkg/plugin"
)

// WildcardLoggerPlugin implements the plugin.Plugin interface
// This plugin logs all messages when registered with the "*" name
type WildcardLoggerPlugin struct{}

// Name returns the name of the plugin
func (w *WildcardLoggerPlugin) Name() string {
	return "wildcard-logger"
}

// Topics returns the topics this plugin should receive messages for
func (w *WildcardLoggerPlugin) Topics() []string {
	return []string{"*"}
}

// HandleMessage processes all incoming messages when registered as wildcard
func (w *WildcardLoggerPlugin) HandleMessage(input plugin.Input) plugin.Output {
	// Log the message details
	timestamp := time.Unix(input.Info.Timestamp, 0).Format("2006-01-02 15:04:05")
	chatType := "private"
	if input.Info.IsGroup {
		chatType = "group"
	}

	fmt.Printf("[%s] [%s] %s (%s): %s\n",
		timestamp,
		chatType,
		input.Info.PushName,
		input.Sender,
		input.Message)

	// Check for specific keywords to respond to
	message := strings.ToLower(strings.TrimSpace(input.Message))

	// Only respond to specific trigger words to avoid spam
	if strings.Contains(message, "hello bot") {
		return plugin.Success("👋 Hello there! I'm watching all messages.")
	}

	if strings.Contains(message, "bot status") {
		return plugin.Success("🤖 Wildcard logger is active and monitoring all messages.")
	}

	// For most messages, we just log them without responding
	// Return success with no reply to indicate we processed it silently
	return plugin.Success("")
}

// GetHelp returns help information for this plugin
func (w *WildcardLoggerPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"wildcard-logger",
		"Logs all messages and responds to specific triggers",
		"Automatically receives all messages",
		[]string{
			"Say 'hello bot' - bot will greet you",
			"Say 'bot status' - bot will report its status",
		},
		"utility",
	)
}

// GetRequiredEnvVars returns the environment variables this plugin needs
func (w *WildcardLoggerPlugin) GetRequiredEnvVars() []string {
	// This plugin doesn't require any environment variables
	return []string{}
}

// Version returns the version of this plugin
func (w *WildcardLoggerPlugin) Version() string {
	return "0.1.0"
}

func init() {
	// Register our plugin with the framework
	plugin.RegisterPlugin(&WildcardLoggerPlugin{})
}

func main() {}
