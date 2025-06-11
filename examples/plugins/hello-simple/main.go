package main

import (
	"fmt"

	"github.com/rubiojr/sup/pkg/plugin"
)

// HelloPlugin implements the plugin.Plugin interface
type HelloPlugin struct{}

// Name returns the name of the plugin
func (h *HelloPlugin) Name() string {
	return "hello"
}

// Topics returns the topics this plugin should receive messages for
func (h *HelloPlugin) Topics() []string {
	return []string{"hello"}
}

// HandleMessage processes incoming messages
func (h *HelloPlugin) HandleMessage(input plugin.Input) plugin.Output {
	if input.Message == "" {
		return plugin.Success(fmt.Sprintf("Hello %s! How can I help you?", input.Info.PushName))
	}

	return plugin.Success(fmt.Sprintf("Hello %s! You said: %s", input.Info.PushName, input.Message))
}

// GetHelp returns help information for this plugin
func (h *HelloPlugin) GetHelp() plugin.HelpOutput {
	return plugin.HelpOutput{
		Name:        "hello",
		Description: "A simple hello world plugin",
		Usage:       ".sup hello [message]",
		Examples:    []string{".sup hello", ".sup hello world"},
		Category:    "examples",
	}
}

func (h *HelloPlugin) GetRequiredEnvVars() []string {
	return []string{}
}

// Version returns the version of this plugin
func (h *HelloPlugin) Version() string {
	return "0.1.0"
}

func init() {
	// Register our plugin with the framework
	plugin.RegisterPlugin(&HelloPlugin{})
}

func main() {}
