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

var sendAudioCmd = &cli.Command{
	Name:  "send-audio",
	Usage: "Send an audio file to a contact or group",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "to",
			Aliases:  []string{"t"},
			Usage:    "Recipient (phone number for user, group JID for group)",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "audio",
			Aliases:  []string{"a"},
			Usage:    "Path to audio file to send",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "group",
			Aliases: []string{"g"},
			Usage:   "Send to group (recipient should be group JID)",
			Value:   false,
		},
	},
	Action: sendAudioCommand,
}

func sendAudioCommand(ctx context.Context, cmd *cli.Command) error {
	recipient := cmd.String("to")
	audioPath := cmd.String("audio")
	isGroup := cmd.Bool("group")

	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return fmt.Errorf("audio file does not exist: %s", audioPath)
	}

	ext := strings.ToLower(filepath.Ext(audioPath))
	validExts := []string{".mp3", ".wav", ".m4a", ".ogg", ".aac", ".flac"}
	isValidAudio := false
	for _, validExt := range validExts {
		if ext == validExt {
			isValidAudio = true
			break
		}
	}
	if !isValidAudio {
		return fmt.Errorf("invalid audio format: %s (supported: mp3, wav, m4a, ogg, aac, flac)", ext)
	}

	c, err := client.GetClient()
	if err != nil {
		return err
	}

	recipientJID, err := c.ResolveRecipient(recipient, isGroup)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	err = c.SendAudio(recipientJID, audioPath)
	if err != nil {
		return fmt.Errorf("failed to send audio: %w", err)
	}

	if isGroup {
		fmt.Printf("Audio %s sent successfully to group %s\n", filepath.Base(audioPath), recipient)
	} else {
		fmt.Printf("Audio %s sent successfully to user %s\n", filepath.Base(audioPath), recipient)
	}

	return nil
}
