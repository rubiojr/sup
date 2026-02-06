package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rubiojr/sup/internal/botfs"
	"github.com/rubiojr/sup/store"
	"github.com/urfave/cli/v3"
)

var pluginStoreCmd = &cli.Command{
	Name:  "store",
	Usage: "Manage plugin persistent storage",
	Commands: []*cli.Command{
		{
			Name:      "list",
			Aliases:   []string{"ls"},
			Usage:     "List keys in a plugin's store",
			ArgsUsage: "<plugin-name> [prefix]",
			Action:    pluginStoreListAction,
		},
		{
			Name:      "get",
			Usage:     "Get the value of a key",
			ArgsUsage: "<plugin-name> <key>",
			Action:    pluginStoreGetAction,
		},
		{
			Name:      "delete",
			Aliases:   []string{"rm"},
			Usage:     "Delete a key from a plugin's store",
			ArgsUsage: "<plugin-name> <key>",
			Action:    pluginStoreDeleteAction,
		},
	},
}

func openPluginStore(pluginName string) (store.Store, error) {
	storePath := filepath.Join(botfs.DataDir(), "store", "store.db")
	s, err := store.NewStore(storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}
	return s.Namespace(pluginName), nil
}

func pluginStoreListAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() == 0 {
		return fmt.Errorf("plugin name required")
	}

	pluginName := c.Args().Get(0)
	prefix := ""
	if c.Args().Len() > 1 {
		prefix = c.Args().Get(1)
	}

	s, err := openPluginStore(pluginName)
	if err != nil {
		return err
	}

	keys, err := s.List(prefix)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if len(keys) == 0 {
		fmt.Printf("No keys found for plugin %q\n", pluginName)
		return nil
	}

	for _, key := range keys {
		fmt.Println(key)
	}

	return nil
}

func pluginStoreGetAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("usage: plugins store get <plugin-name> <key>")
	}

	pluginName := c.Args().Get(0)
	key := c.Args().Get(1)

	s, err := openPluginStore(pluginName)
	if err != nil {
		return err
	}

	value, err := s.Get([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}

	if value == nil {
		return fmt.Errorf("key %q not found", key)
	}

	fmt.Println(string(value))

	return nil
}

func pluginStoreDeleteAction(ctx context.Context, c *cli.Command) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("usage: plugins store delete <plugin-name> <key>")
	}

	pluginName := c.Args().Get(0)
	key := c.Args().Get(1)

	s, err := openPluginStore(pluginName)
	if err != nil {
		return err
	}

	if err := s.Delete([]byte(key)); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	fmt.Printf("Deleted key %q from plugin %q\n", key, pluginName)

	return nil
}
