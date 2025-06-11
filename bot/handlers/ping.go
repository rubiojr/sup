package handlers

import (
	"fmt"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/internal/client"
)

type PingHandler struct{}

func (h *PingHandler) HandleMessage(msg *events.Message) error {
	fmt.Printf("Ping command received from %s\n", msg.Info.Chat.String())
	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("Error getting client: %w", err)
	}

	c.SendText(msg.Info.Chat, "pong")

	return nil
}

func (h *PingHandler) Name() string {
	return "ping"
}

func (h *PingHandler) Topics() []string {
	return []string{"ping"}
}

func (h *PingHandler) GetHelp() HandlerHelp {
	return HandlerHelp{
		Name:        "ping",
		Description: "Test bot connectivity",
		Usage:       ".sup ping",
		Examples:    []string{".sup ping"},
		Category:    "basic",
	}
}
