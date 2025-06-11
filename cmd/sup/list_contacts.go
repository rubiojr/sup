package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var listContactCmd = &cli.Command{
	Name:    "list-contacts",
	Aliases: []string{"lc"},
	Usage:   "List all WhatsApp contacts",
	Action:  listContactsCommand,
}

func listContactsCommand(ctx context.Context, cmd *cli.Command) error {
	c, err := client.GetClient()
	if err != nil {
		return err
	}

	fmt.Println("\n=== YOUR WHATSAPP CONTACTS ===")
	contacts, err := c.GetAllContacts()
	if err != nil {
		return fmt.Errorf("error getting contacts: %w", err)
	}

	for jid, contact := range contacts {
		fmt.Printf("Name: %s\n", contact.FullName)
		fmt.Printf("JID: %s\n", jid.String())
		fmt.Printf("Phone: %s\n", jid.User)
		if contact.BusinessName != "" {
			fmt.Printf("Business: %s\n", contact.BusinessName)
		}
		fmt.Println("---")
	}
	fmt.Printf("Found %d contacts total\n", len(contacts))

	return nil
}
