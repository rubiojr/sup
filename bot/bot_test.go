package bot

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/store"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// newTestBot creates a bot with temp cache and store for testing
func newTestBot(t *testing.T, opts ...Option) (*Bot, error) {
	t.Helper()
	tmpDir := t.TempDir()
	c, err := cache.NewCache(filepath.Join(tmpDir, "cache.db"))
	if err != nil {
		return nil, err
	}
	s, err := store.NewStore(filepath.Join(tmpDir, "store.db"))
	if err != nil {
		return nil, err
	}
	opts = append([]Option{WithCache(c), WithStore(s)}, opts...)
	return New(opts...)
}

func TestNew(t *testing.T) {
	bot, err := newTestBot(t)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if bot == nil {
		t.Fatal("New() returned nil")
	}
	if bot.Registry() == nil {
		t.Fatal("Bot registry is nil")
	}
	if bot.PluginManager() == nil {
		t.Fatal("Bot registry is nil")
	}
	if bot.logger == nil {
		t.Fatal("Bot logger is nil")
	}
}

func TestNewWithCustomLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	bot, err := newTestBot(t, WithLogger(logger))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if bot == nil {
		t.Fatal("New() returned nil")
	}
	if bot.logger != logger {
		t.Fatal("Custom logger was not set")
	}

	// Test that the logger is actually used
	bot.logger.Info("test message", "key", "value")
	if buf.Len() == 0 {
		t.Fatal("Logger was not used")
	}

	// Verify the log contains our test message
	logOutput := buf.String()
	if !contains(logOutput, "test message") {
		t.Errorf("Log output does not contain expected message: %s", logOutput)
	}
}

func TestRegisterHandler(t *testing.T) {
	bot, err := newTestBot(t)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// Create a mock handler
	mockHandler := &mockHandler{}

	err = bot.RegisterHandler(mockHandler)
	if err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	// Verify handler was registered
	_, err = bot.GetHandler("test")
	if err != nil {
		t.Fatalf("Failed to get registered handler: %v", err)
	}
}

func TestStartWithCancellation(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	bot, err := newTestBot(t, WithLogger(logger))
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should return quickly due to context cancellation
	// Note: This will fail to connect to WhatsApp, but that's expected in tests
	err = bot.Start(ctx)

	// We expect an error because WhatsApp client won't be available in tests
	if err == nil {
		t.Fatal("Expected error due to WhatsApp client unavailability")
	}

	// Verify logging occurred
	logOutput := buf.String()
	if !contains(logOutput, "Starting bot mode") {
		t.Errorf("Expected 'Starting bot mode' in logs, got: %s", logOutput)
	}
}

func TestWildcardHandler(t *testing.T) {
	bot, err := newTestBot(t)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// Create a mock wildcard handler
	mockWildcard := &mockWildcardHandler{}
	err = bot.RegisterHandler(mockWildcard)
	if err != nil {
		t.Fatalf("Failed to register wildcard handler: %v", err)
	}

	// Create a mock message
	msg := createMockMessage("Hello, this is a test message", "user@example.com")

	// Test wildcard handler
	bot.handleRegularMessage(msg)

	// Verify the wildcard handler was called
	if !mockWildcard.called {
		t.Fatal("Wildcard handler was not called")
	}

	if mockWildcard.receivedMessage.Info.Chat.String() != msg.Info.Chat.String() {
		t.Errorf("Expected sender %s, got %s",
			msg.Info.Chat.String(),
			mockWildcard.receivedMessage.Info.Chat.String())
	}
}

func TestWildcardHandlerWithCommandMessage(t *testing.T) {
	bot, err := newTestBot(t)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	// Create mock handlers
	mockWildcard := &mockWildcardHandler{}
	mockCommand := &mockHandler{}

	err = bot.RegisterHandler(mockWildcard)
	if err != nil {
		t.Fatalf("Failed to register wildcard handler: %v", err)
	}

	err = bot.RegisterHandler(mockCommand)
	if err != nil {
		t.Fatalf("Failed to register command handler: %v", err)
	}

	// Create a mock command message
	msg := createMockMessage(".sup test argument", "user@example.com")

	// Process the message (should trigger command handler)
	bot.eventHandler(msg, ".sup")

	// Verify command handler was called
	if !mockCommand.called {
		t.Fatal("Command handler was not called")
	}

	// Create a regular message to test wildcard
	regularMsg := createMockMessage("Hello world", "user@example.com")
	bot.eventHandler(regularMsg, ".sup")

	// Verify wildcard handler was called for regular message
	if !mockWildcard.called {
		t.Fatal("Wildcard handler was not called for regular message")
	}
}

func TestBotCache(t *testing.T) {
	// Create temporary directory for cache
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "bot_cache.db")

	// Create cache for bot
	cache, err := cache.NewCache(cachePath)
	if err != nil {
		t.Fatalf("NewCache() returned error: %v", err)
	}

	// Create bot with cache
	bot, err := newTestBot(t, WithCache(cache))
	if err != nil {
		t.Fatalf("New() with cache returned error: %v", err)
	}

	key := "test_bot_key"
	value := []byte("test_bot_value")

	// Test Cache
	botCache, err := bot.Cache()
	if err != nil {
		t.Fatalf("Cache() returned error: %v", err)
	}

	err = botCache.Put([]byte(key), value)
	if err != nil {
		t.Fatalf("Cache.Put() returned error: %v", err)
	}

	// Test GetCached
	retrievedValue, err := botCache.Get([]byte(key))
	if err != nil {
		t.Fatalf("Cache.Get() returned error: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(retrievedValue))
	}
}

func TestBotStore(t *testing.T) {
	bot, err := newTestBot(t)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	key := "test_bot_store_key"
	value := []byte("test_bot_store_value")

	// Test Store
	botStore, err := bot.Store()
	if err != nil {
		t.Fatalf("Store() returned error: %v", err)
	}

	err = botStore.Put([]byte(key), value)
	if err != nil {
		t.Fatalf("Store.Put() returned error: %v", err)
	}

	// Test Get
	retrievedValue, err := botStore.Get([]byte(key))
	if err != nil {
		t.Fatalf("Store.Get() returned error: %v", err)
	}

	if string(retrievedValue) != string(value) {
		t.Errorf("Expected value %s, got %s", string(value), string(retrievedValue))
	}

	// Test persistence - update the value
	newValue := []byte("updated_store_value")
	err = botStore.Put([]byte(key), newValue)
	if err != nil {
		t.Fatalf("Store.Put() update returned error: %v", err)
	}

	// Verify the update
	updatedValue, err := botStore.Get([]byte(key))
	if err != nil {
		t.Fatalf("Store.Get() after update returned error: %v", err)
	}

	if string(updatedValue) != string(newValue) {
		t.Errorf("Expected updated value %s, got %s", string(newValue), string(updatedValue))
	}

	// Test namespace isolation
	namespaced := botStore.Namespace("test")
	err = namespaced.Put([]byte(key), value)
	if err != nil {
		t.Fatalf("Namespaced store Put() returned error: %v", err)
	}

	// Verify namespaced value doesn't affect main store
	mainValue, err := botStore.Get([]byte(key))
	if err != nil {
		t.Fatalf("Store.Get() from main after namespace put returned error: %v", err)
	}

	if string(mainValue) != string(newValue) {
		t.Errorf("Namespace isolation failed: expected %s, got %s", string(newValue), string(mainValue))
	}
}

func TestBotStoreNotInitialized(t *testing.T) {
	// Create bot without store (should fail with store disabled)
	// This test is tricky because New() always initializes a store now
	// We need to create a bot with a nil store directly
	bot := &Bot{
		store: nil,
	}

	// Test Store with nil store
	_, err := bot.Store()
	if err == nil {
		t.Fatal("Expected error for Store() with nil store, got nil")
	}
}

func TestBotCacheNotInitialized(t *testing.T) {
	// Create bot without cache (should fail with cache disabled)
	// This test is tricky because New() always initializes a cache now
	// We need to create a bot with a nil cache directly
	bot := &Bot{
		cache: nil,
	}

	// Test Cache with nil cache
	_, err := bot.Cache()
	if err == nil {
		t.Fatal("Expected error for Cache() with nil cache, got nil")
	}
}

// mockWildcardHandler is a mock implementation for testing wildcard functionality
type mockWildcardHandler struct {
	called          bool
	receivedMessage *events.Message
}

func (m *mockWildcardHandler) HandleMessage(msg *events.Message) error {
	m.called = true
	m.receivedMessage = msg
	return nil
}

func (m *mockWildcardHandler) Name() string {
	return "wildcard"
}

func (m *mockWildcardHandler) Topics() []string {
	return []string{"*"}
}

func (m *mockWildcardHandler) GetHelp() handlers.HandlerHelp {
	return handlers.HandlerHelp{
		Name:        "wildcard",
		Description: "Mock wildcard handler for testing",
		Usage:       "Receives all messages",
		Examples:    []string{"Any message"},
		Category:    "test",
	}
}

func (m *mockWildcardHandler) Version() string {
	return "0.1.0"
}

// Enhanced mock handler to track if it was called
type mockHandler struct {
	called bool
}

func (m *mockHandler) HandleMessage(msg *events.Message) error {
	m.called = true
	return nil
}

func (m *mockHandler) Name() string {
	return "test"
}

func (m *mockHandler) Topics() []string {
	return []string{"test"}
}

func (m *mockHandler) GetHelp() handlers.HandlerHelp {
	return handlers.HandlerHelp{
		Name:        "test",
		Description: "Mock handler for testing",
		Usage:       "test",
		Examples:    []string{"test example"},
		Category:    "test",
	}
}

func (m *mockHandler) Version() string {
	return "0.1.0"
}

// createMockMessage creates a mock WhatsApp message for testing
func createMockMessage(text, sender string) *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			ID: "test-message-id",
			MessageSource: types.MessageSource{
				Chat:   types.JID{User: sender, Server: types.DefaultUserServer},
				Sender: types.JID{User: sender, Server: types.DefaultUserServer},
			},
			PushName:  "Test User",
			Timestamp: time.Now(),
		},
		Message: &waE2E.Message{
			Conversation: &text,
		},
	}
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
