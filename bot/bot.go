package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/internal/botfs"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/store"
)

const DefaultTrigger = ".sup"

// Bot represents the WhatsApp bot instance
type Bot struct {
	registry      handlers.Registry
	pluginManager handlers.PluginManager
	logger        *slog.Logger
	trigger       string
	cache         cache.Cache
	store         store.Store
	allowedGroups   map[string]struct{}
	allowedUsers    map[string]struct{}
	allowedCommands []string
}

// Option is a function that configures the Bot
type Option func(*Bot)

// WithLogger sets a custom logger for the bot.
// If not provided, the bot will use slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(b *Bot) {
		b.logger = logger
	}
}

// WithTrigger sets a custom trigger prefix for the bot commands.
// The default trigger is ".sup". Commands will be recognized when they
// start with the specified trigger followed by a space and command name.
//
// Example:
//
//	bot := New(WithTrigger("mybot"))
//	// Commands will now be triggered with "mybot ping" instead of "sup ping"
func WithTrigger(t string) Option {
	return func(b *Bot) {
		b.trigger = t
	}
}

// WithRegistry sets a custom handler registry for the bot.
// If not provided, the bot will create a new registry with default settings.
func WithRegistry(registry handlers.Registry) Option {
	return func(b *Bot) {
		b.registry = registry
	}
}

// WithPluginManager sets a custom plugin manager for the bot's registry.
// This option creates a new registry with the provided plugin manager.
// If used together with WithRegistry, this option should be applied first.
func WithPluginManager(pm handlers.PluginManager) Option {
	return func(b *Bot) {
		b.pluginManager = pm
		b.registry.SetPluginManager(pm)
	}
}

// WithCache sets a custom cache for the bot.
// If not provided, the bot will create a default cache.
func WithCache(cache cache.Cache) Option {
	return func(b *Bot) {
		b.cache = cache
	}
}

// WithStore sets a custom store for the bot.
// If not provided, the bot will create a default store.
func WithStore(store store.Store) Option {
	return func(b *Bot) {
		b.store = store
	}
}

// WithAllowedGroups sets the allowed group JIDs.
// Only messages from these groups will be processed.
// An empty list means no groups are allowed.
func WithAllowedGroups(groups []string) Option {
	return func(b *Bot) {
		b.allowedGroups = make(map[string]struct{}, len(groups))
		for _, g := range groups {
			b.allowedGroups[g] = struct{}{}
		}
	}
}

// WithAllowedUsers sets the allowed user JIDs.
// Only messages from these users will be processed.
// An empty list means no users are allowed.
func WithAllowedUsers(users []string) Option {
	return func(b *Bot) {
		b.allowedUsers = make(map[string]struct{}, len(users))
		for _, u := range users {
			b.allowedUsers[u] = struct{}{}
		}
	}
}

// WithAllowedCommands sets the commands that WASM plugins are allowed to execute.
func WithAllowedCommands(commands []string) Option {
	return func(b *Bot) {
		b.allowedCommands = commands
	}
}

// New creates a new Bot instance with the given options.
// The bot is initialized with a default logger (slog.Default()) and
// all handlers are automatically registered.
//
// The default plugin manager is also initialized, which loads plugins from
// the default plugin path ($HOME/.local/share/sup/plugins").
//
// Example:
//
//	// Create a bot with default logger
//	bot := New()
//
//	// Create a bot with custom logger
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	bot := New(WithLogger(logger))
//
//	// Create a bot with custom plugin manager
//	pm := handlers.NewPluginManager("/custom/plugin/path", cache, store)
//	bot := New(WithPluginManager(pm))
//
//	// Create a bot with custom registry
//	registry := handlers.NewRegistry()
//	bot := New(WithRegistry(registry))
func New(opts ...Option) (*Bot, error) {
	b := &Bot{
		registry: handlers.NewRegistry(),
		logger:   slog.Default(),
		trigger:  DefaultTrigger,
	}

	for _, opt := range opts {
		opt(b)
	}

	// Initialize default cache if none provided
	cacheDir := filepath.Join(botfs.DataDir(), "cache")
	cachePath := filepath.Join(cacheDir, "cache.db")
	if b.cache == nil {
		err := os.MkdirAll(cacheDir, os.ModeDir|0755)
		if err != nil {
			return nil, err
		}
		cache, err := cache.NewCache(cachePath)
		if err != nil {
			b.logger.Warn("Failed to initialize default cache", "error", err)
			return nil, err
		} else {
			b.logger.Debug("Cache initialized")
			b.cache = cache
		}
	}

	// Initialize default store if none provided
	storeDir := filepath.Join(botfs.DataDir(), "store")
	storePath := filepath.Join(storeDir, "store.db")
	if b.store == nil {
		err := os.MkdirAll(storeDir, os.ModeDir|0755)
		if err != nil {
			return nil, err
		}
		store, err := store.NewStore(storePath)
		if err != nil {
			b.logger.Warn("Failed to initialize default store", "error", err)
			return nil, err
		} else {
			b.logger.Debug("Store initialized")
			b.store = store
		}
	}
	b.pluginManager = handlers.DefaultPluginManager(b.cache, b.store, b.allowedCommands)

	if b.pluginManager != nil {
		// Load WASM plugins
		if err := b.pluginManager.LoadPlugins(); err != nil {
			b.logger.Warn("Failed to load WASM plugins", "error", err)
		}
		b.registry.SetPluginManager(b.pluginManager)
	}

	return b, nil
}

// RegisterHandler registers a new handler with the bot
func (b *Bot) RegisterHandler(handler handlers.Handler) error {
	return b.Registry().Register(handler.Name(), handler)
}

// Start starts the event handler loop
func (b *Bot) Start(ctx context.Context) error {
	b.logger.Debug("Starting bot mode", "prefix", b.trigger)

	c, err := client.GetClient()
	if err != nil {
		return err
	}
	c.AddEventHandler(func(evt any) { b.eventHandler(evt, b.trigger) })
	defer c.Disconnect()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	b.logger.Debug("Bot is now running and listening for commands")

	select {
	case <-ctx.Done():
		b.logger.Debug("Bot shutting down via context cancellation")
		return ctx.Err()
	case <-sigChan:
		b.logger.Debug("Bot shutting down via signal")
		return nil
	}
}

func (b *Bot) Registry() handlers.Registry {
	return b.registry
}

// GetAllHandlers returns all registered handlers
func (b *Bot) GetAllHandlers() map[string]handlers.Handler {
	return b.registry.GetAllHandlers()
}

// GetHandler returns a specific handler by name
func (b *Bot) GetHandler(name string) (handlers.Handler, error) {
	return b.registry.Get(name)
}

// LoadPlugins loads all WASM plugins from the plugin directory
func (b *Bot) LoadPlugins() error {
	return b.pluginManager.LoadPlugins()
}

// ReloadPlugins reloads all WASM plugins
func (b *Bot) ReloadPlugins() error {
	return b.pluginManager.ReloadPlugins()
}

// UnloadPlugins unloads all WASM plugins
func (b *Bot) UnloadPlugins() error {
	return b.pluginManager.UnloadAll()
}

// eventHandler handles incoming WhatsApp events
func (b *Bot) eventHandler(evt any, handlerPrefix string) {
	switch v := evt.(type) {
	case *events.Message:
		isGroup := v.Info.Chat.Server == types.GroupServer

		if !b.isAllowed(v.Info.Chat.String(), isGroup) {
			b.logger.Warn("Message from non-allowed source ignored",
				"jid", v.Info.Chat.String(),
				"is_group", isGroup)
			return
		}

		if loc := v.Message.GetLocationMessage(); loc != nil {
			fmt.Printf("Accuracy: %d\n", loc.AccuracyInMeters)
			fmt.Printf("Latitude: %f\n", loc.GetDegreesLatitude())
			fmt.Printf("Longitude: %f\n", loc.GetDegreesLongitude())
		}
		if v.Message.GetConversation() != "" || v.Message.GetExtendedTextMessage() != nil {
			var messageText string
			if v.Message.GetConversation() != "" {
				messageText = v.Message.GetConversation()
			} else if v.Message.GetExtendedTextMessage() != nil {
				messageText = v.Message.GetExtendedTextMessage().GetText()
			}

			if strings.HasPrefix(messageText, handlerPrefix) {
				// Handle as command
				b.handleCommand(v, handlerPrefix)
			}
		}
		b.handleRegularMessage(v)

		if isGroup {
			b.logger.Debug("Received group message", "jid", v.Info.Chat.String())
		} else {
			b.logger.Debug("Received user message",
				"jid", v.Info.Chat.String(),
				"phone", v.Info.Chat.User)
		}
	}
}

// handleCommand processes a command message and routes it to subscribed handlers
func (b *Bot) handleCommand(msg *events.Message, handlerPrefix string) {
	var messageText string
	if msg.Message.GetConversation() != "" {
		messageText = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		messageText = msg.Message.GetExtendedTextMessage().GetText()
	}

	command := strings.TrimSpace(strings.TrimPrefix(messageText, handlerPrefix))
	parts := strings.Split(command, " ")

	commandName := parts[0]
	if commandName == "" {
		commandName = "help"
	}
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}

	b.logger.Debug("Processing command",
		"command", commandName,
		"args", args,
		"sender", msg.Info.Chat.String())

	// Get all handlers that should receive this command
	handlers := b.registry.GetHandlersForMessage(commandName, true)
	if len(handlers) == 0 {
		b.logger.Warn("Unknown command received",
			"command", commandName,
			"sender", msg.Info.Chat.String())
		return
	}

	// Send message to all subscribed handlers
	for _, handler := range handlers {
		if err := handler.HandleMessage(msg); err != nil {
			b.logger.Error("Error handling command",
				"command", commandName,
				"sender", msg.Info.Chat.String(),
				"handler", handler.Name(),
				"error", err)
		} else {
			b.logger.Debug("Command handled successfully",
				"command", commandName,
				"sender", msg.Info.Chat.String(),
				"handler", handler.Name(),
			)
		}
	}
}

// handleRegularMessage sends non-command messages to wildcard subscribers
func (b *Bot) handleRegularMessage(msg *events.Message) {
	// Get handlers that subscribe to all messages (wildcard)
	handlers := b.registry.GetHandlersForMessage("", false)
	if len(handlers) == 0 {
		return
	}

	b.logger.Debug("Processing regular message",
		"sender", msg.Info.Chat.String(),
		"handlers", len(handlers))

	for _, handler := range handlers {
		if err := handler.HandleMessage(msg); err != nil {
			b.logger.Error("Error handling regular message",
				"sender", msg.Info.Chat.String(),
				"error", err)
		} else {
			b.logger.Debug("Regular message handled successfully",
				"sender", msg.Info.Chat.String())
		}
	}
}

func (b *Bot) PluginManager() handlers.PluginManager {
	return b.pluginManager
}

// Cache returns the cache service
func (b *Bot) Cache() (cache.Cache, error) {
	if b.cache == nil {
		return nil, errors.New("cache service not initialized")
	}
	return b.cache, nil
}

// Store returns the store service
func (b *Bot) Store() (store.Store, error) {
	if b.store == nil {
		return nil, errors.New("store service not initialized")
	}
	return b.store, nil
}

// RegisterDefaultHandlers registers all available bot handlers with the given bot
func (b *Bot) RegisterDefaultHandlers() error {
	// Create help handler with registry reference
	helpHandler := handlers.NewHelpHandler(b.registry, b.pluginManager)

	// Register basic handlers
	if err := b.RegisterHandler(&handlers.PingHandler{}); err != nil {
		return err
	}
	if err := b.RegisterHandler(helpHandler); err != nil {
		return err
	}

	return nil
}

// isAllowed checks if a message source is in the allow list.
func (b *Bot) isAllowed(jid string, isGroup bool) bool {
	if isGroup {
		if b.allowedGroups == nil {
			return false
		}
		_, ok := b.allowedGroups[jid]
		return ok
	}
	if b.allowedUsers == nil {
		return false
	}
	_, ok := b.allowedUsers[jid]
	return ok
}
