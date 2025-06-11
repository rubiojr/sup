# Wildcard Handlers Implementation

This document describes the wildcard handler functionality that has been implemented in the Sup WhatsApp bot.

## Overview

Wildcard handlers are special handlers that receive ALL messages sent to the bot, not just commands that start with the trigger prefix (`.sup`). They enable passive monitoring, logging, and natural language responses.

## How It Works

### Registration
Handlers are registered as wildcard handlers by using the special name `*`:

```go
bot.RegisterHandler("*", &MyWildcardHandler{})
```

### Message Flow
1. **All Messages**: When any message is received by the bot
2. **Wildcard First**: If a wildcard handler (`*`) is registered, it receives the message
3. **Command Check**: If the message starts with the trigger prefix, command handlers are also called
4. **Both Execute**: Both wildcard and command handlers can process the same message

### Key Features
- **Receive All Messages**: Gets every message regardless of prefix
- **Full Message Text**: Receives complete message content, not parsed arguments
- **Selective Response**: Can choose when to respond to avoid spam
- **Background Processing**: Perfect for logging and monitoring

## Implementation Details

### Bot Changes (`internal/bot/bot.go`)

Added `handleWildcardMessage()` method that:
- Checks if a wildcard handler is registered
- Calls the handler with the full message
- Handles errors gracefully
- Logs wildcard processing

Modified `eventHandler()` to:
- Call wildcard handlers first for all messages
- Then process command handlers for prefixed messages

### WASM Handler Updates (`internal/handlers/wasm_handler.go`)

Enhanced `WasmHandler` to:
- Detect wildcard handlers (name == "*")
- Pass full message text to wildcard plugins
- Pass parsed arguments to regular command plugins

### Handler Interface
All handlers use the same interface:
```go
type Handler interface {
    HandleMessage(msg *events.Message) error
    GetHelp() HandlerHelp
}
```

Wildcard handlers extract the full message text from `msg.Message`.

## Examples

### Built-in Go Handler

```go
type MyWildcardHandler struct{}

func (h *MyWildcardHandler) HandleMessage(msg *events.Message) error {
    var messageText string
    if msg.Message.GetConversation() != "" {
        messageText = msg.Message.GetConversation()
    } else if msg.Message.GetExtendedTextMessage() != nil {
        messageText = msg.Message.GetExtendedTextMessage().GetText()
    }

    // Log all messages
    fmt.Printf("Message from %s: %s\n", msg.Info.PushName, messageText)
    
    // Respond only to specific triggers
    if strings.Contains(strings.ToLower(messageText), "hello bot") {
        // Send response...
    }
    
    return nil
}

func (h *MyWildcardHandler) GetHelp() handlers.HandlerHelp {
    return handlers.HandlerHelp{
        Name: "*",
        Description: "Monitors all messages",
        // ... other fields
    }
}
```

### WASM Plugin Handler

```go
type WildcardPlugin struct{}

func (w *WildcardPlugin) HandleMessage(input plugin.Input) plugin.Output {
    // input.Message contains full text for wildcard handlers
    fmt.Printf("Received: %s\n", input.Message)
    
    if strings.Contains(strings.ToLower(input.Message), "bot status") {
        return plugin.Success("🤖 Bot is active!")
    }
    
    return plugin.Success("") // Silent processing
}

func (w *WildcardPlugin) GetHelp() plugin.HelpOutput {
    return plugin.NewHelpOutput("*", "Wildcard logger", "Auto", []string{}, "utility")
}
```

## Use Cases

1. **Message Logging**: Log all conversations for audit or analytics
2. **Passive Monitoring**: Watch for keywords without explicit commands
3. **Natural Language**: Respond to conversational messages
4. **Auto-responses**: React to greetings, thanks, etc.
5. **Content Analysis**: Analyze sentiment, detect patterns
6. **Integration Bridges**: Forward to other systems

## Best Practices

### Selective Response
```go
// Good: Only respond to specific triggers
if strings.Contains(message, "specific trigger") {
    return plugin.Success("Response")
}
return plugin.Success("") // Silent processing

// Bad: Responding to everything
return plugin.Success("I got: " + message)
```

### Performance
```go
// Good: Quick checks first
if !strings.Contains(message, "keyword") {
    return plugin.Success("") // Early return
}
// Expensive processing only when needed
```

### Error Handling
```go
func (w *WildcardHandler) HandleMessage(msg *events.Message) error {
    defer func() {
        if r := recover(); r != nil {
            fmt.Printf("Wildcard handler panic: %v\n", r)
        }
    }()
    // Processing logic
    return nil
}
```

## Testing

Added comprehensive tests in `internal/bot/bot_test.go`:
- `TestWildcardHandler`: Verifies wildcard handler receives messages
- `TestWildcardHandlerWithCommandMessage`: Confirms both wildcard and command handlers execute

## Files Added/Modified

### Core Implementation
- `internal/bot/bot.go` - Added wildcard message handling
- `internal/handlers/wasm_handler.go` - Enhanced for wildcard support

### Examples
- `internal/bot/handlers/wildcard.go` - Example built-in wildcard handler
- `examples/wildcard-example.go` - Complete usage example
- `examples/plugins/wildcard-logger/` - WASM plugin example
- `examples/wildcard-handlers.md` - Detailed documentation

### Tests
- `internal/bot/bot_test.go` - Wildcard handler tests

## Usage Examples

### Register Built-in Handler
```go
bot := New()
bot.RegisterHandler("*", &MyWildcardHandler{})
```

### WASM Plugin
1. Create plugin with name "*" in GetHelp()
2. Build with `tinygo build -o plugin.wasm -target wasi main.go`
3. Place in `~/.local/share/sup/plugins/`

### Full Example
See `examples/wildcard-example.go` for a complete working example that demonstrates:
- Message logging and counting
- Natural language responses
- Coexistence with command handlers

## Compatibility

- ✅ Works with existing command handlers
- ✅ Compatible with WASM plugins
- ✅ Maintains all existing functionality
- ✅ Backward compatible with current bots
- ✅ All tests pass

The wildcard handler implementation provides a powerful foundation for building more intelligent and responsive WhatsApp bots.