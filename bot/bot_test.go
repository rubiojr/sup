package bot

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/exec"
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
	t.Setenv("HOME", tmpDir)
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
	bot, err := newTestBot(t,
		WithAllowedUsers([]string{"user@example.com@s.whatsapp.net"}),
	)
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
	bot.eventHandler(t.Context(), msg, ".sup")

	// Verify command handler was called
	if !mockCommand.called {
		t.Fatal("Command handler was not called")
	}
	if mockWildcard.calls != 1 {
		t.Fatalf("Wildcard received command %d times, want 1", mockWildcard.calls)
	}

	// Create a regular message to test wildcard
	regularMsg := createMockMessage("Hello world", "user@example.com")
	bot.eventHandler(t.Context(), regularMsg, ".sup")

	// Verify wildcard handler was called for regular message
	if !mockWildcard.called {
		t.Fatal("Wildcard handler was not called for regular message")
	}
	if mockWildcard.calls != 2 {
		t.Fatalf("Wildcard received %d messages, want 2", mockWildcard.calls)
	}
}

func TestAutoReplyDoesNotExecuteCommands(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo required for WASM integration test")
	}
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "plugins")
	if err := os.Mkdir(pluginDir, 0o750); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "tinygo", "build", "-target", "wasi", "-o", filepath.Join(pluginDir, "autoreply.wasm"), ".")
	cmd.Dir = "../plugins/autoreply"
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, output)
	}
	b, err := newTestBot(t, WithAllowedUsers([]string{"123@s.whatsapp.net"}))
	if err != nil {
		t.Fatal(err)
	}
	pm := handlers.NewPluginManager(pluginDir, b.cache, b.store, nil)
	if err := pm.LoadPlugins(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pm.UnloadAll() })
	autoReply, ok := pm.GetPlugin("autoreply")
	if !ok {
		t.Fatal("autoreply failed to load")
	}
	b.pluginManager = pm
	if err := b.registry.SetPluginManager(pm); err != nil {
		t.Fatal(err)
	}
	ledger, err := sql.Open("sqlite3", filepath.Join(dir, "autoreply.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	_, err = ledger.Exec(`INSERT INTO replies VALUES (?, ?, ?, ?, 0, 1)`, "original", "generated", "123@s.whatsapp.net", time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	command, wildcard := &mockHandler{}, &mockWildcardHandler{}
	if err := b.RegisterHandler(command); err != nil {
		t.Fatal(err)
	}
	if err := b.RegisterHandler(wildcard); err != nil {
		t.Fatal(err)
	}
	msg := createMockMessage(".sup test", "123")
	msg.Info.IsFromMe = true
	msg.Info.ID = "generated"
	capture := &recordingArchive{}
	WithArchive(capture)(b)
	b.eventHandler(t.Context(), msg, ".sup")
	if len(capture.ids) != 1 || capture.ids[0] != "generated" {
		t.Fatal("auto-reply must be archived before it is filtered from bot routing")
	}
	if command.called || wildcard.called {
		t.Fatal("auto-generated reply reached a handler")
	}
	msg.Info.ID = "manual-self-message"
	b.eventHandler(t.Context(), msg, ".sup")
	if !command.called || wildcard.calls != 1 {
		t.Fatal("manual self-message should still reach both handlers once")
	}

	t.Run("auto-reply scope does not grant command access", func(t *testing.T) {
		templatePath := filepath.Join(dir, "plugin-data", "autoreply", "reply.txt")
		if err := os.WriteFile(templatePath, []byte("I'm away."), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"dry-run", "on"}, {"test-unlimited", "on"}, {"enable"}} {
			if _, err := autoReply.HandleCLI(args); err != nil {
				t.Fatal(err)
			}
		}
		b.allowedGroups = map[string]struct{}{"123@g.us": {}}
		for _, tc := range []struct {
			name         string
			scope        string
			user         string
			server       string
			text         string
			self         bool
			wantReply    int
			wantHandlers bool
		}{
			{"default blocks unlisted", "allow-list", "456", types.DefaultUserServer, ".sup test", false, 0, false},
			{"all answers unlisted DM", "all", "456", types.DefaultUserServer, "hello", false, 1, false},
			{"all answers unlisted LID command", "all", "789", types.HiddenUserServer, ".sup test", false, 1, false},
			{"all excludes unlisted group", "all", "456", types.GroupServer, ".sup test", false, 0, false},
			{"all excludes listed group", "all", "123", types.GroupServer, ".sup test", false, 0, true},
			{"listed DM gets normal routing", "all", "123", types.DefaultUserServer, ".sup test", false, 1, true},
			{"all answers unlisted self", "all", "999", types.DefaultUserServer, "hello", true, 1, false},
			{"allow-list blocks unlisted self again", "allow-list", "999", types.DefaultUserServer, "hello", true, 0, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := autoReply.HandleCLI([]string{"scope", tc.scope}); err != nil {
					t.Fatal(err)
				}
				command.called, wildcard.called, wildcard.calls = false, false, 0
				msg := createMockMessage(tc.text, tc.user)
				msg.Info.Chat.Server = tc.server
				msg.Info.Sender = msg.Info.Chat
				msg.Info.IsFromMe = tc.self
				msg.Info.ID = tc.name
				msg.Info.Timestamp = time.Now().Add(time.Second)
				b.eventHandler(t.Context(), msg, ".sup")
				var attempts int
				if err := ledger.QueryRow("SELECT count(*) FROM replies WHERE incoming_id = ?", tc.name).Scan(&attempts); err != nil {
					t.Fatal(err)
				}
				if attempts != tc.wantReply {
					t.Errorf("auto-reply attempts = %d, want %d", attempts, tc.wantReply)
				}
				if command.called != tc.wantHandlers || wildcard.called != tc.wantHandlers {
					t.Errorf("other handlers called: command=%t, wildcard=%t, want %t", command.called, wildcard.called, tc.wantHandlers)
				}
			})
		}
	})
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
	calls           int
	receivedMessage *events.Message
}

func (m *mockWildcardHandler) HandleMessage(msg *events.Message) error {
	m.called = true
	m.calls++
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

func TestAllowListBlocksUnknownUser(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	bot, err := newTestBot(t,
		WithLogger(logger),
		WithAllowedUsers([]string{"allowed@s.whatsapp.net"}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	mock := &mockHandler{}
	if err := bot.RegisterHandler(mock); err != nil {
		t.Fatal(err)
	}

	msg := createMockMessage(".sup test", "blocked@example.com")
	bot.eventHandler(t.Context(), msg, ".sup")

	if mock.called {
		t.Fatal("Handler should not have been called for non-allowed user")
	}
	if !contains(buf.String(), "non-allowed") {
		t.Errorf("Expected warn log about non-allowed source, got: %s", buf.String())
	}
}

func TestAllowListPermitsUser(t *testing.T) {
	bot, err := newTestBot(t,
		WithAllowedUsers([]string{"allowed@s.whatsapp.net"}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	mock := &mockHandler{}
	if err := bot.RegisterHandler(mock); err != nil {
		t.Fatal(err)
	}

	msg := createMockMessage(".sup test", "allowed")
	msg.Info.Chat.Server = types.DefaultUserServer
	bot.eventHandler(t.Context(), msg, ".sup")

	if !mock.called {
		t.Fatal("Handler should have been called for allowed user")
	}
}

func TestAllowListBlocksUnknownGroup(t *testing.T) {
	bot, err := newTestBot(t,
		WithAllowedGroups([]string{"allowed-group@g.us"}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	mock := &mockHandler{}
	if err := bot.RegisterHandler(mock); err != nil {
		t.Fatal(err)
	}

	msg := createMockMessage(".sup test", "other-group")
	msg.Info.Chat.Server = types.GroupServer
	bot.eventHandler(t.Context(), msg, ".sup")

	if mock.called {
		t.Fatal("Handler should not have been called for non-allowed group")
	}
}

func TestAllowListPermitsGroup(t *testing.T) {
	bot, err := newTestBot(t,
		WithAllowedGroups([]string{"allowed-group@g.us"}),
	)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	mock := &mockHandler{}
	if err := bot.RegisterHandler(mock); err != nil {
		t.Fatal(err)
	}

	msg := createMockMessage(".sup test", "allowed-group")
	msg.Info.Chat.Server = types.GroupServer
	bot.eventHandler(t.Context(), msg, ".sup")

	if !mock.called {
		t.Fatal("Handler should have been called for allowed group")
	}
}

func TestEmptyAllowListDeniesAll(t *testing.T) {
	bot, err := newTestBot(t)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	mock := &mockHandler{}
	if err := bot.RegisterHandler(mock); err != nil {
		t.Fatal(err)
	}

	msg := createMockMessage(".sup test", "anyone")
	bot.eventHandler(t.Context(), msg, ".sup")

	if mock.called {
		t.Fatal("Handler should not have been called when allow lists are nil (deny all)")
	}
}
