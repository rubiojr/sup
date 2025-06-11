package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var sendClipboardCmd = &cli.Command{
	Name:  "send-clipboard",
	Usage: "Send clipboard content as a file to a contact or group",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "to",
			Aliases:  []string{"t"},
			Usage:    "Recipient (phone number for user, group JID for group)",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "group",
			Aliases: []string{"g"},
			Usage:   "Send to group (recipient should be group JID)",
			Value:   false,
		},
	},
	Action: sendClipboardCommand,
}

func sendClipboardCommand(ctx context.Context, cmd *cli.Command) error {
	recipient := cmd.String("to")
	isGroup := cmd.Bool("group")

	tmpFile, err := createTempFileFromClipboard()
	if err != nil {
		return fmt.Errorf("failed to create temporary file from clipboard: %w", err)
	}
	defer os.Remove(tmpFile)

	c, err := client.GetClient()
	if err != nil {
		return err
	}

	recipientJID, err := c.ResolveRecipient(recipient, isGroup)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	err = c.SendFile(recipientJID, tmpFile)
	if err != nil {
		return fmt.Errorf("failed to send file: %w", err)
	}

	if isGroup {
		fmt.Printf("Clipboard content sent successfully as file to group %s\n", recipient)
	} else {
		fmt.Printf("Clipboard content sent successfully as file to user %s\n", recipient)
	}

	return nil
}

func createTempFileFromClipboard() (string, error) {
	cmd := exec.Command("wl-paste")
	clipboardData, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get clipboard content: %w", err)
	}

	if len(clipboardData) == 0 {
		return "", fmt.Errorf("clipboard is empty")
	}

	mtype := mimetype.Detect(clipboardData)
	extension := strings.TrimPrefix(mtype.Extension(), ".")
	if extension == "" {
		extension = "txt"
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("clipboard_*.%s", extension))
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, strings.NewReader(string(clipboardData)))
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write clipboard data to file: %w", err)
	}

	return tmpFile.Name(), nil
}
