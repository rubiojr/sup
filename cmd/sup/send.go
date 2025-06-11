package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var sendCmd = &cli.Command{
	Name:  "send",
	Usage: "Send a text message to a contact or group",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "to",
			Aliases:  []string{"t"},
			Usage:    "Recipient (phone number for user, group JID for group)",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "message",
			Aliases:  []string{"m"},
			Usage:    "Text message to send",
			Required: true,
		},
		&cli.BoolFlag{
			Name:    "group",
			Aliases: []string{"g"},
			Usage:   "Send to group (recipient should be group JID)",
			Value:   false,
		},
	},
	Action: sendCommand,
}

func sendCommand(ctx context.Context, cmd *cli.Command) error {
	recipient := cmd.String("to")
	message := cmd.String("message")
	isGroup := cmd.Bool("group")

	if message == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		message = strings.Join(lines, "\n")
	}

	c, err := client.GetClient()
	if err != nil {
		return err
	}

	recipientJID, err := c.ResolveRecipient(recipient, isGroup)
	if err != nil {
		return fmt.Errorf("invalid recipient: %w", err)
	}

	err = c.SendText(recipientJID, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if isGroup {
		fmt.Printf("Message sent successfully to group %s\n", recipient)
	} else {
		fmt.Printf("Message sent successfully to user %s\n", recipient)
	}

	return nil
}
