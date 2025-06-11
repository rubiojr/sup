package handlers

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/internal/client"
)

type WildcardHandler struct{}

func (w *WildcardHandler) Name() string {
	return "wildcard"
}

func (w *WildcardHandler) Topics() []string {
	return []string{"*"}
}

func (w *WildcardHandler) HandleMessage(msg *events.Message) error {
	// Extract message text
	var messageText string
	if msg.Message.GetConversation() != "" {
		messageText = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		messageText = msg.Message.GetExtendedTextMessage().GetText()
	}

	// Log the message
	timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
	chatType := "private"
	if msg.Info.Chat.Server == types.GroupServer {
		chatType = "group"
	}

	fmt.Printf("[WILDCARD] [%s] [%s] %s (%s): %s\n",
		timestamp,
		chatType,
		msg.Info.PushName,
		msg.Info.Chat.String(),
		messageText)

	// Check for specific keywords to respond to
	message := strings.ToLower(strings.TrimSpace(messageText))

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	// Only respond to specific trigger words to avoid spam
	if strings.Contains(message, "hello bot") {
		c.SendText(msg.Info.Chat, "👋 Hello there! I'm watching all messages.")
		return nil
	}

	if strings.Contains(message, "bot status") {
		c.SendText(msg.Info.Chat, "🤖 Wildcard handler is active and monitoring all messages.")
		return nil
	}

	// For most messages, we just log them without responding
	return nil
}

func (h *WildcardHandler) GetHelp() HandlerHelp {
	return HandlerHelp{
		Name:        "wildcard",
		Description: "Logs all messages and responds to specific triggers",
		Usage:       "Automatically receives all messages",
		Examples: []string{
			"Say 'hello bot' - bot will greet you",
			"Say 'bot status' - bot will report its status",
		},
		Category: "utility",
	}
}
