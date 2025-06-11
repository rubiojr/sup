package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rubiojr/sup/bot"
	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/internal/registry"
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
			Name:   "load",
			Usage:  "Load plugins from directory",
			Action: loadPluginsAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "dir",
					Usage: "Plugin directory path",
					Value: getDefaultPluginDir(),
				},
			},
		},
		{
			Name:   "reload",
			Usage:  "Reload all plugins",
			Action: reloadPluginsAction,
		},
		{
			Name:   "info",
			Usage:  "Show plugin information",
			Action: pluginInfoAction,
		},
		{
			Name:      "plugin-download",
			Usage:     "Download and install a plugin from the registry",
			ArgsUsage: "<plugin-name> [version]",
			Action:    pluginDownloadAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "registry",
					Usage: "Registry URL",
					Value: registry.DefaultRegistryURL,
				},
			},
		},
		{
			Name:   "plugin-list",
			Usage:  "List available plugins from the registry",
			Action: pluginListAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "registry",
					Usage: "Registry URL",
					Value: registry.DefaultRegistryURL,
				},
				&cli.BoolFlag{
					Name:  "installed-only",
					Usage: "Show only installed plugins",
				},
				&cli.BoolFlag{
					Name:  "available-only",
					Usage: "Show only available (not installed) plugins",
				},
			},
		},
		{
			Name:      "plugin-remove",
			Usage:     "Remove an installed plugin",
			ArgsUsage: "<plugin-name>",
			Action:    pluginRemoveAction,
		},
	},
}

func listPluginsAction(ctx context.Context, c *cli.Command) error {
	botInstance := bot.New()
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
	for name := range builtinHandlers {
		fmt.Printf("  %-15s\n", name)
	}

	fmt.Printf("\nWASM Plugin Handlers (%d):\n", len(pluginHandlers))
	if len(pluginHandlers) == 0 {
		fmt.Println("  No WASM plugins loaded")
		fmt.Printf("  Plugin directory: %s\n", getDefaultPluginDir())
	} else {
		for name := range pluginHandlers {
			fmt.Printf("  %-15s\n", name)
		}
	}

	return nil
}

func loadPluginsAction(ctx context.Context, c *cli.Command) error {
	pluginDir := c.String("dir")

	pluginManager := handlers.NewPluginManager(pluginDir)

	fmt.Printf("Loading plugins from: %s\n", pluginDir)

	if err := pluginManager.LoadPlugins(); err != nil {
		return fmt.Errorf("failed to load plugins: %w", err)
	}

	plugins := pluginManager.GetAllPlugins()
	if len(plugins) == 0 {
		fmt.Println("No WASM plugins found in directory")
		return nil
	}

	fmt.Printf("Successfully loaded %d plugin(s):\n", len(plugins))
	for name, plugin := range plugins {
		help := plugin.GetHelp()
		fmt.Printf("  %-15s - %s\n", name, help.Description)
	}

	return nil
}

func reloadPluginsAction(ctx context.Context, c *cli.Command) error {
	botInstance := bot.New()

	fmt.Println("Reloading WASM plugins...")

	if err := botInstance.ReloadPlugins(); err != nil {
		return fmt.Errorf("failed to reload plugins: %w", err)
	}

	pluginManager := botInstance.PluginManager()
	plugins := pluginManager.GetAllPlugins()

	fmt.Printf("Successfully reloaded %d plugin(s)\n", len(plugins))
	for name, plugin := range plugins {
		help := plugin.GetHelp()
		fmt.Printf("  %-15s - %s\n", name, help.Description)
	}

	return nil
}

func pluginInfoAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required")
	}

	pluginName := c.Args().First()

	botInstance := bot.New()
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

func pluginDownloadAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required")
	}

	pluginName := c.Args().First()
	version := ""
	if c.Args().Len() > 1 {
		version = c.Args().Get(1)
	}

	registryURL := c.String("registry")
	client := registry.NewClient(registryURL)

	targetDir := getDefaultPluginDir()

	fmt.Printf("Downloading plugin '%s'", pluginName)
	if version != "" && version != "latest" {
		fmt.Printf(" version %s", version)
	}
	fmt.Printf(" from %s...\n", registryURL)

	if err := client.DownloadPlugin(pluginName, version, targetDir); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	fmt.Printf("Successfully downloaded and installed plugin '%s' to %s\n", pluginName, targetDir)
	fmt.Println("Run 'sup plugins reload' to load the new plugin.")

	return nil
}

func pluginListAction(ctx context.Context, c *cli.Command) error {
	registryURL := c.String("registry")
	client := registry.NewClient(registryURL)

	fmt.Printf("Fetching plugin list from %s...\n", registryURL)

	plugins, err := client.ListPlugins()
	if err != nil {
		return fmt.Errorf("failed to list plugins: %w", err)
	}

	installedOnly := c.Bool("installed-only")
	availableOnly := c.Bool("available-only")

	var filteredPlugins []registry.PluginInfo
	for _, plugin := range plugins {
		if installedOnly && !plugin.Installed {
			continue
		}
		if availableOnly && plugin.Installed {
			continue
		}
		filteredPlugins = append(filteredPlugins, plugin)
	}

	sort.Slice(filteredPlugins, func(i, j int) bool {
		return filteredPlugins[i].Name < filteredPlugins[j].Name
	})

	if len(filteredPlugins) == 0 {
		if installedOnly {
			fmt.Println("No installed plugins found.")
		} else if availableOnly {
			fmt.Println("No available plugins found.")
		} else {
			fmt.Println("No plugins found in registry.")
		}
		return nil
	}

	fmt.Printf("\nAvailable Plugins (%d):\n", len(filteredPlugins))
	fmt.Printf("%-20s %-10s %-15s %-10s %s\n", "NAME", "VERSION", "AUTHOR", "STATUS", "DESCRIPTION")
	fmt.Printf("%s\n", strings.Repeat("-", 80))

	for _, plugin := range filteredPlugins {
		status := "available"
		if plugin.Installed {
			status = "installed"
		}

		description := plugin.Description
		if len(description) > 35 {
			description = description[:32] + "..."
		}

		fmt.Printf("%-20s %-10s %-15s %-10s %s\n",
			plugin.Name,
			plugin.Version,
			plugin.Author,
			status,
			description,
		)
	}

	return nil
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
	fmt.Println("Run 'sup plugins reload' to unload the plugin from memory.")

	return nil
}
