package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rubiojr/sup/bot"
	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/internal/botfs"
	"github.com/rubiojr/sup/internal/config"
	"github.com/rubiojr/sup/store"
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
		{
			Name:      "run",
			Usage:     "Run a plugin's CLI command",
			ArgsUsage: "<plugin-name> [args...]",
			Action:    pluginRunAction,
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

func pluginRunAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required\nUsage: sup plugins run <plugin-name> [args...]")
	}

	pluginName := c.Args().First()

	// Load config for allowed commands
	cfg, _ := config.Load(config.DefaultPath())
	var allowedCommands []string
	if cfg != nil {
		allowedCommands = cfg.Plugins.AllowedCommands
	}

	// Load the plugin
	pluginDir := getDefaultPluginDir()
	wasmPath := filepath.Join(pluginDir, fmt.Sprintf("%s.wasm", pluginName))
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin '%s' not found at %s", pluginName, wasmPath)
	}

	storePath := filepath.Join(botfs.DataDir(), "store", "store.db")
	s, err := store.NewStore(storePath)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}

	cachePath := filepath.Join(botfs.DataDir(), "cache", "cache.db")
	c2, err := cache.NewCache(cachePath)
	if err != nil {
		return fmt.Errorf("failed to open cache: %w", err)
	}

	handler, err := handlers.NewWasmHandler(wasmPath, c2.Namespace(pluginName), s.Namespace(pluginName), allowedCommands)
	if err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	if !handler.SupportsCLI() {
		return fmt.Errorf("plugin '%s' does not support CLI commands", pluginName)
	}

	// Pass remaining args (after plugin name)
	args := c.Args().Tail()
	output, err := handler.HandleCLI(args)
	if err != nil {
		return err
	}

	if output != "" {
		fmt.Print(output)
	}

	return nil
}
