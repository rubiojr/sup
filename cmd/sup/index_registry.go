package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rubiojr/sup/internal/registry"
	"github.com/urfave/cli/v3"
)

var indexRegistryCmd = &cli.Command{
	Name:      "index-registry",
	Usage:     "Build registry index from plugins directory",
	ArgsUsage: "<plugins-directory> <base-url>",
	Action:    indexRegistryAction,
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
  │   ├── version/
  │   │   ├── plugin-name.wasm
  │   │   └── metadata.json (optional)
  │   └── another-version/
  │       ├── plugin-name.wasm
  │       └── metadata.json (optional)
  └── another-plugin/
      └── version/
          ├── another-plugin.wasm
          └── metadata.json (optional)

Metadata files should contain plugin information:
{
  "name": "plugin-name",
  "description": "Plugin description",
  "author": "Author Name",
  "home_url": "https://github.com/author/plugin",
  "category": "utility",
  "tags": ["tag1", "tag2"]
}

This command generates:
- index.json (uncompressed for debugging)
- index.json.gz (compressed registry index)
- index.json.gz.sha256 (checksum file)`,
}

func indexRegistryAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("plugins directory and base URL are required")
	}

	pluginsDir := c.Args().First()
	baseURL := c.Args().Get(1)
	outputDir := c.String("output")

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
	fmt.Printf("  Base URL: %s\n", baseURL)
	fmt.Printf("  Output directory: %s\n", absOutputDir)
	fmt.Println()

	builder := registry.NewBuilder(absPluginsDir, baseURL)

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