package main

import (
	"fmt"
	"strings"

	"github.com/rubiojr/sup/pkg/plugin"
)

type TestPlugin struct{}

func (p *TestPlugin) Name() string {
	return "test-plugin"
}

func (p *TestPlugin) Topics() []string {
	return []string{"test", "echo", "greet"}
}

func (p *TestPlugin) HandleMessage(input plugin.Input) plugin.Output {
	message := strings.ToLower(strings.TrimSpace(input.Message))

	switch {
	case strings.HasPrefix(message, "echo "):
		// Echo back the message after "echo "
		echoText := strings.TrimPrefix(message, "echo ")
		return plugin.Success(fmt.Sprintf("Echo: %s", echoText))

	case message == "hello" || message == "hi":
		return plugin.Success(fmt.Sprintf("Hello %s! Nice to meet you.", input.Info.PushName))

	case message == "error":
		return plugin.Error("This is a test error")

	case message == "info":
		groupStatus := "direct message"
		if input.Info.IsGroup {
			groupStatus = "group message"
		}
		return plugin.Success(fmt.Sprintf("Message ID: %s, From: %s (%s), Type: %s",
			input.Info.ID, input.Info.PushName, input.Sender, groupStatus))

	case message == "cache test":
		return plugin.Success("Cache test not available in CLI mode")

	case message == "storage test":
		return plugin.Success("Storage test not available in CLI mode")

	case message == "help":
		help := p.GetHelp()
		return plugin.Success(fmt.Sprintf("%s - %s\nUsage: %s", help.Name, help.Description, help.Usage))

	default:
		return plugin.Success("I received your message: " + input.Message)
	}
}

func (p *TestPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"test-plugin",
		"A test plugin for integration testing",
		"Send 'hello', 'echo <text>', 'error', 'info', or 'help'",
		[]string{
			"hello - Get a greeting",
			"echo hello world - Echo back 'hello world'",
			"error - Trigger a test error",
			"info - Get message information",
			"help - Show this help",
		},
		"testing",
	)
}

func (p *TestPlugin) GetRequiredEnvVars() []string {
	return []string{"TEST_ENV_VAR"}
}

func (p *TestPlugin) Version() string {
	return "1.0.0"
}

func init() {
	plugin.RegisterPlugin(&TestPlugin{})
}

func main() {
	// Keep empty main for WASM compatibility
}
