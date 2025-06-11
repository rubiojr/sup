package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"

	"github.com/rubiojr/sup/internal/client"
)

var registerCmd = &cli.Command{
	Name:   "register",
	Usage:  "Register and authenticate with WhatsApp by scanning QR code",
	Action: registerCommand,
}

func registerCommand(ctx context.Context, cmd *cli.Command) error {
	// Check if already registered
	registered, err := isRegistered()
	if err != nil {
		return fmt.Errorf("failed to check registration status: %w", err)
	}

	if registered {
		fmt.Println("✓ Already registered with WhatsApp")
		fmt.Println("Use 'sup status' to check your registration status")
		return nil
	}

	c, err := client.NewClientForRegistration()
	if err != nil {
		return fmt.Errorf("failed to initialize client for registration: %w", err)
	}
	defer c.Disconnect()

	err = c.Register()
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	fmt.Println("\nRegistration completed successfully!")
	fmt.Println("When WhatsApp on your phone shows that the registration is finished,")
	fmt.Println("press Ctrl+C to exit this command.")
	fmt.Println("\nWaiting for Ctrl+C...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nExiting registration. You can now use other sup commands.")
	return nil
}
