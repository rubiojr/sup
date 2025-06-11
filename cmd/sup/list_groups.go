package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var listGroupCmd = &cli.Command{
	Name:    "list-groups",
	Aliases: []string{"lg"},
	Usage:   "List all WhatsApp groups",
	Action:  listGroupsCommand,
}

func listGroupsCommand(ctx context.Context, cmd *cli.Command) error {
	c, err := client.GetClient()
	if err != nil {
		return err
	}

	fmt.Println("\n=== YOUR WHATSAPP GROUPS ===")
	groups, err := c.GetJoinedGroups()
	if err != nil {
		return fmt.Errorf("error getting groups: %w", err)
	}

	for _, group := range groups {
		fmt.Printf("Group: %s\n", group.Name)
		fmt.Printf("JID: %s\n", group.JID.String())
		fmt.Printf("Participants: %d\n", len(group.Participants))
		fmt.Println("---")
	}
	fmt.Printf("Found %d groups total\n", len(groups))

	return nil
}
