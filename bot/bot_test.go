package bot

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/rubiojr/sup/bot/handlers"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestNew(t *testing.T) {
	bot := New()
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

	bot := New(WithLogger(logger))
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
	bot := New()

	// Create a mock handler
	mockHandler := &mockHandler{}

	err := bot.RegisterHandler(mockHandler)
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

	bot := New(WithLogger(logger))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should return quickly due to context cancellation
	// Note: This will fail to connect to WhatsApp, but that's expected in tests
	err := bot.Start(ctx)

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
	bot := New()

	// Create a mock wildcard handler
	mockWildcard := &mockWildcardHandler{}
	err := bot.RegisterHandler(mockWildcard)
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
	bot := New()

	// Create mock handlers
	mockWildcard := &mockWildcardHandler{}
	mockCommand := &mockHandler{}

	err := bot.RegisterHandler(mockWildcard)
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
