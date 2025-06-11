package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var sendImageCmd = &cli.Command{
	Name:  "send-image",
	Usage: "Send an image to a contact or group",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "to",
			Aliases:  []string{"t"},
			Usage:    "Recipient (phone number for user, group JID for group)",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "image",
			Aliases:  []string{"i"},
			Usage:    "Path to image file to send",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "group",
			Aliases: []string{"g"},
			Usage:   "Send to group (recipient should be group JID)",
			Value:   false,
		},
	},
	Action: sendImageCommand,
}

func sendImageCommand(ctx context.Context, cmd *cli.Command) error {
	recipient := cmd.String("to")
	imagePath := cmd.String("image")
	isGroup := cmd.Bool("group")

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return fmt.Errorf("image file does not exist: %s", imagePath)
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
	isValidImage := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValidImage = true
			break
		}
	}
	if !isValidImage {
		return fmt.Errorf("invalid image format: %s (supported: jpg, jpeg, png, gif, webp)", ext)
	}

	c, err := client.GetClient()
	if err != nil {
		return err
	}

	recipientJID, err := c.ResolveRecipient(recipient, isGroup)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	err = c.SendImage(recipientJID, imagePath)
	if err != nil {
		return fmt.Errorf("failed to send image: %w", err)
	}

	if isGroup {
		fmt.Printf("Image %s sent successfully to group %s\n", filepath.Base(imagePath), recipient)
	} else {
		fmt.Printf("Image %s sent successfully to user %s\n", filepath.Base(imagePath), recipient)
	}

	return nil
}
