package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var sendFileCmd = &cli.Command{
	Name:  "send-file",
	Usage: "Send a file to a contact or group",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "to",
			Aliases:  []string{"t"},
			Usage:    "Recipient (phone number for user, group JID for group)",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "file",
			Aliases:  []string{"f"},
			Usage:    "Path to file to send",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "group",
			Aliases: []string{"g"},
			Usage:   "Send to group (recipient should be group JID)",
			Value:   false,
		},
	},
	Action: sendFileCommand,
}

func sendFileCommand(ctx context.Context, cmd *cli.Command) error {
	recipient := cmd.String("to")
	filePath := cmd.String("file")
	isGroup := cmd.Bool("group")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	c, err := client.GetClient()
	if err != nil {
		return err
	}

	recipientJID, err := c.ResolveRecipient(recipient, isGroup)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	err = c.SendFile(recipientJID, filePath)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	if isGroup {
		fmt.Printf("File %s sent successfully to group %s\n", filepath.Base(filePath), recipient)
	} else {
		fmt.Printf("File %s sent successfully to user %s\n", filepath.Base(filePath), recipient)
	}

	return nil
}
