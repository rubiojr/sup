package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/internal/registry"
	"github.com/urfave/cli/v3"
)

var registryCmd = &cli.Command{
	Name:  "registry",
	Usage: "Manage plugin registry",
	Commands: []*cli.Command{
		{
			Name:      "index",
			Usage:     "Build registry index from plugins directory",
			ArgsUsage: "<plugins-directory>",
			Action:    registryIndexAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "output",
					Usage: "Output directory for index files",
					Value: ".",
				},
				&cli.BoolFlag{
					Name:  "verbose",
					Usage: "Enable verbose output",
				},
			},
			Description: `Build a compressed registry index from a plugins directory structure.

The plugins directory should be organized as:
  plugins/
  ├── plugin-name/
  │   ├── metadata.json (optional)
  │   ├── version/
  │   │   └── plugin-name.wasm
  │   └── another-version/
  │       └── plugin-name.wasm
  └── another-plugin/
      ├── metadata.json (optional)
      └── version/
          └── another-plugin.wasm

This command generates:
- index.json (uncompressed for debugging)
- index.json.gz (compressed registry index)
- index.json.gz.sha256 (checksum file)`,
		},
		{
			Name:   "list",
			Usage:  "List available plugins from the registry",
			Action: registryListAction,
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
			Name:      "install",
			Usage:     "Download and install a plugin from the registry",
			ArgsUsage: "<plugin-name> [version]",
			Action:    registryDownloadAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "registry",
					Usage: "Registry URL",
					Value: registry.DefaultRegistryURL,
				},
				&cli.BoolFlag{
					Name:  "debug",
					Usage: "Enable debugging",
				},
			},
		},
	},
}

func registryIndexAction(ctx context.Context, c *cli.Command) error {
	pluginsDir := c.Args().First()
	outputDir := c.String("output")
	if pluginsDir == "" {
		pluginsDir = "."
	}

	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return fmt.Errorf("plugins directory does not exist: %s", pluginsDir)
	}

	absPluginsDir, err := filepath.Abs(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute output path: %w", err)
	}

	fmt.Printf("Building registry index...\n")
	fmt.Printf("  Plugins directory: %s\n", absPluginsDir)
	fmt.Printf("  Output directory: %s\n", absOutputDir)
	fmt.Println()

	builder := registry.NewBuilder(absPluginsDir)

	index, err := builder.BuildIndex()
	if err != nil {
		return fmt.Errorf("failed to build index: %w", err)
	}

	if len(index.Plugins) == 0 {
		fmt.Println("Warning: No plugins found in directory")
		return nil
	}

	if c.Bool("verbose") {
		fmt.Printf("Found plugins:\n")
		for name, plugin := range index.Plugins {
			fmt.Printf("  %s (latest: %s, versions: %d)\n", name, plugin.Latest, len(plugin.Versions))
			if c.Bool("verbose") {
				for version := range plugin.Versions {
					fmt.Printf("    - %s\n", version)
				}
			}
		}
		fmt.Println()
	}

	if err := builder.WriteIndex(index, absOutputDir); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

func registryListAction(ctx context.Context, c *cli.Command) error {
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

func registryDownloadAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required")
	}

	if c.Bool("debug") {
		log.SetLevel(slog.LevelDebug)
	}

	pluginName := c.Args().First()
	version := ""
	if c.Args().Len() > 1 {
		version = c.Args().Get(1)
	}

	registryURL := c.String("registry")
	client := registry.NewClient(registryURL)

	targetDir := getDefaultPluginDir()

	fmt.Printf("Downloading plugin")
	if version != "" && version != "latest" {
		fmt.Printf(" version %s", version)
	}
	fmt.Println("...")
	log.Debug("Downloading plugin", "name", pluginName, "version", version)
	log.Debug("Registry", "registry", registryURL)
	log.Debug("Target directory", "directory", targetDir)

	if err := client.DownloadPlugin(pluginName, version, targetDir); err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	fmt.Println("Successfully downloaded and installed plugin!")

	return nil
}
