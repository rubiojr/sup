package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"go.mau.fi/whatsmeow/store/sqlstore"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

var statusCmd = &cli.Command{
	Name:    "status",
	Aliases: []string{"s"},
	Usage:   "Check WhatsApp registration status",
	Action:  statusCommand,
}

func statusCommand(ctx context.Context, cmd *cli.Command) error {
	registered, err := isRegistered()
	if err != nil {
		return fmt.Errorf("failed to check registration status: %w", err)
	}

	if registered {
		fmt.Println("✓ WhatsApp session is registered and active")
		fmt.Println("You can use other commands to send messages and files")
	} else {
		fmt.Println("✗ No WhatsApp session found")
		fmt.Println("Run 'sup register' to authenticate with WhatsApp")
	}

	return nil
}

func isRegistered() (bool, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}

	dataDir := filepath.Join(h, ".local/share/sup")
	dbFile := filepath.Join(dataDir, "sup.db")

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false, nil
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbFile), nil)
	if err != nil {
		return false, err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return false, err
	}

	return deviceStore.ID != nil, nil
}
