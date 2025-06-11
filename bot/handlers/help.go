package handlers

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/cmd/sup/version"
	"github.com/rubiojr/sup/internal/client"
)

type HelpHandler struct {
	registry      Registry
	pluginManager PluginManager
}

func NewHelpHandler(registry Registry, pm PluginManager) *HelpHandler {
	return &HelpHandler{
		registry:      registry,
		pluginManager: pm,
	}
}

func (h *HelpHandler) Name() string {
	return "help"
}

func (h *HelpHandler) Topics() []string {
	return []string{"help"}
}

func (h *HelpHandler) HandleMessage(msg *events.Message) error {
	// Extract command text from the message
	var messageText string
	if msg.Message.GetConversation() != "" {
		messageText = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		messageText = msg.Message.GetExtendedTextMessage().GetText()
	}

	// Extract command arguments (skip the command prefix and handler name)
	parts := strings.Fields(messageText)
	var text string
	if len(parts) > 2 {
		text = strings.Join(parts[2:], " ")
	}

	fmt.Printf("Help command received from %s: %s\n", msg.Info.Chat.String(), text)

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	// Parse command argument
	commandArg := strings.TrimSpace(text)

	// If a specific command is requested, show detailed help for that command
	if commandArg != "" {
		allHelp := h.GetAllHelp()
		if help, exists := allHelp[commandArg]; exists {
			helpText := fmt.Sprintf("🤖 *Help for %s*\n\n", help.Name)
			helpText += fmt.Sprintf("📝 *Description:* %s\n\n", help.Description)
			helpText += fmt.Sprintf("💡 *Usage:* %s\n\n", help.Usage)

			if len(help.Examples) > 0 {
				helpText += "*Examples:*\n"
				for _, example := range help.Examples {
					helpText += fmt.Sprintf("• %s\n", example)
				}
				helpText += "\n"
			}

			categoryEmojis := map[string]string{
				"basic":   "📍",
				"utility": "🔧",
				"fun":     "🎮",
			}
			if emoji, exists := categoryEmojis[help.Category]; exists {
				helpText += fmt.Sprintf("%s *Category:* %s\n", emoji, capitalizeFirst(help.Category))
			}

			err = c.SendText(msg.Info.Chat, helpText)
			if err != nil {
				return fmt.Errorf("error sending command help: %w", err)
			}
			return nil
		} else {
			// Command not found
			helpText := fmt.Sprintf("❌ Command '%s' not found.\n\nUse `.sup help` to see all available commands.", commandArg)
			err = c.SendText(msg.Info.Chat, helpText)
			if err != nil {
				return fmt.Errorf("error sending command not found message: %w", err)
			}
			return nil
		}
	}

	// Show general help if no specific command requested
	helpText := "🤖 *Sup Bot Commands*\n\n"

	allHelp := h.GetAllHelp()

	// Group commands by category
	categories := make(map[string][]string)
	categoryEmojis := map[string]string{
		"basic":   "📍",
		"utility": "🔧",
		"fun":     "🎮",
	}
	categoryTitles := map[string]string{
		"basic":   "Basic Commands",
		"utility": "Utility Commands",
		"fun":     "Fun Commands",
	}

	// Group handlers by category
	for _, help := range allHelp {
		if help.Category == "" {
			help.Category = "fun" // default category
		}

		cmdLine := fmt.Sprintf("• %s - %s", help.Name, help.Description)

		categories[help.Category] = append(categories[help.Category], cmdLine)
	}

	// Display categories in order
	categoryOrder := []string{"basic", "utility", "fun"}
	for _, category := range categoryOrder {
		if commands, exists := categories[category]; exists && len(commands) > 0 {
			emoji := categoryEmojis[category]
			title := categoryTitles[category]
			helpText += fmt.Sprintf("%s *%s:*\n", emoji, title)
			helpText += strings.Join(commands, "\n") + "\n\n"
		}
	}

	helpText += "💡 *Usage:* Type .sup followed by any command above\n"
	helpText += "For detailed help on a specific command, use: `.sup help <command>`"

	err = c.SendText(msg.Info.Chat, helpText)
	if err != nil {
		return fmt.Errorf("error sending help message: %w", err)
	}

	return nil
}

// isWildcardHandler checks if a handler subscribes to all messages
func (r *HelpHandler) isWildcardHandler(topics []string) bool {
	for _, topic := range topics {
		if topic == "*" {
			return true
		}
	}
	return false
}

func (r *HelpHandler) GetAllHelp() map[string]HandlerHelp {
	result := make(map[string]HandlerHelp)
	for name, handler := range r.registry.GetAllHandlers() {
		help := handler.GetHelp()
		// Only include handlers that don't have wildcard topics in help
		if !r.isWildcardHandler(handler.Topics()) {
			result[name] = help
		}
	}

	// Add WASM plugin help
	for name, plugin := range r.pluginManager.GetAllPlugins() {
		help := plugin.GetHelp()
		// Only include plugins that don't subscribe to wildcard messages in help
		if !r.isWildcardHandler(plugin.Topics()) {
			result[name] = help
		}
	}

	return result
}

func (h *HelpHandler) GetHelp() HandlerHelp {
	return HandlerHelp{
		Name:        "help",
		Description: "Show available bot commands or detailed help for a specific command",
		Usage:       ".sup help [command]",
		Examples:    []string{".sup help", ".sup help ping", ".sup help meteo"},
		Category:    "basic",
	}
}

func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (h *HelpHandler) Version() string {
	return version.String
}
