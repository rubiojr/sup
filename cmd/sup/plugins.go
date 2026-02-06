package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rubiojr/sup/bot"
	"github.com/rubiojr/sup/bot/handlers"
	"github.com/urfave/cli/v3"
)

var pluginsCmd = &cli.Command{
	Name:  "plugins",
	Usage: "Manage WASM plugins",
	Commands: []*cli.Command{
		{
			Name:   "list",
			Usage:  "List all loaded plugins",
			Action: listPluginsAction,
		},
		{
			Name:   "info",
			Usage:  "Show plugin information",
			Action: pluginInfoAction,
		},
		{
			Name:      "remove",
			Usage:     "Remove an installed plugin",
			ArgsUsage: "<plugin-name>",
			Action:    pluginRemoveAction,
		},
		pluginStoreCmd,
	},
}

func listPluginsAction(ctx context.Context, c *cli.Command) error {
	botInstance, err := bot.New()
	if err != nil {
		return err
	}

	if err := botInstance.RegisterDefaultHandlers(); err != nil {
		return fmt.Errorf("failed to register handlers: %w", err)
	}

	allHandlers := botInstance.GetAllHandlers()

	builtinHandlers := make(map[string]handlers.Handler)
	pluginHandlers := make(map[string]handlers.Handler)

	// Get plugin manager to identify which handlers are plugins
	pluginManager := botInstance.PluginManager()
	allPlugins := pluginManager.GetAllPlugins()

	for name, handler := range allHandlers {
		if _, isPlugin := allPlugins[name]; isPlugin {
			pluginHandlers[name] = handler
		} else {
			builtinHandlers[name] = handler
		}
	}

	fmt.Printf("Built-in Handlers (%d):\n", len(builtinHandlers))
	for name, handler := range builtinHandlers {
		version := handler.Version()
		fmt.Printf("  %-15s %-10s\n", name, version)
	}

	fmt.Printf("\nWASM Plugin Handlers (%d):\n", len(pluginHandlers))
	if len(pluginHandlers) == 0 {
		fmt.Println("  No WASM plugins loaded")
		fmt.Printf("  Plugin directory: %s\n", getDefaultPluginDir())
	} else {
		for name, handler := range pluginHandlers {
			version := handler.Version()
			fmt.Printf("  %-15s %-10s\n", name, version)
		}
	}

	return nil
}

func pluginInfoAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required")
	}

	pluginName := c.Args().First()

	botInstance, err := bot.New()
	if err != nil {
		return fmt.Errorf("failed to create bot instance: %w", err)
	}
	if err := botInstance.RegisterDefaultHandlers(); err != nil {
		return fmt.Errorf("failed to register handlers: %w", err)
	}

	handler, err := botInstance.GetHandler(pluginName)
	if err != nil {
		return fmt.Errorf("plugin '%s' not found", pluginName)
	}

	help := handler.GetHelp()

	// Check if it's a WASM plugin
	pluginManager := botInstance.PluginManager()
	_, isPlugin := pluginManager.GetPlugin(pluginName)

	pluginType := "Built-in Handler"
	if isPlugin {
		pluginType = "WASM Plugin"
	}

	fmt.Printf("Plugin Information: %s\n", pluginName)
	fmt.Printf("Type:        %s\n", pluginType)
	fmt.Printf("Version:     %s\n", handler.Version())
	fmt.Printf("Description: %s\n", help.Description)
	fmt.Printf("Usage:       %s\n", help.Usage)
	fmt.Printf("Category:    %s\n", help.Category)

	if len(help.Examples) > 0 {
		fmt.Printf("Examples:\n")
		for _, example := range help.Examples {
			fmt.Printf("  %s\n", example)
		}
	}

	return nil
}

func getDefaultPluginDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.local/share/sup/plugins"
	}
	return filepath.Join(homeDir, ".local", "share", "sup", "plugins")
}

func pluginRemoveAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required")
	}

	pluginName := c.Args().First()
	pluginDir := getDefaultPluginDir()
	pluginPath := filepath.Join(pluginDir, fmt.Sprintf("%s.wasm", pluginName))

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin '%s' is not installed", pluginName)
	}

	fmt.Printf("Removing plugin '%s'...\n", pluginName)

	if err := os.Remove(pluginPath); err != nil {
		return fmt.Errorf("failed to remove plugin file: %w", err)
	}

	fmt.Printf("Successfully removed plugin '%s'\n", pluginName)

	return nil
}
