# Wildcard Handlers Implementation

This document describes the wildcard handler functionality that has been implemented in the Sup WhatsApp bot.

## Overview

Wildcard handlers are special handlers that receive ALL messages sent to the bot, not just commands that start with the trigger prefix (`.sup`). They enable passive monitoring, logging, and natural language responses.

## How It Works

### Registration
Handlers are registered as wildcard handlers by returning `["*"]` from their `Topics()` method:

```go
func (h *MyWildcardHandler) Topics() []string {
    return []string{"*"}
}
```

### Message Flow
1. **All Messages**: When any message is received by the bot
2. **Topic Check**: The registry checks each handler's topics to determine which should receive the message
3. **Wildcard First**: Handlers with `"*"` in their topics receive all messages
4. **Command Check**: If the message starts with the trigger prefix, handlers with matching command topics are also called
5. **Both Execute**: Both wildcard and command handlers can process the same message

### Key Features
- **Receive All Messages**: Gets every message regardless of prefix
- **Full Message Text**: Receives complete message content from WhatsApp events
- **Selective Response**: Can choose when to respond to avoid spam
- **Background Processing**: Perfect for logging and monitoring

## Implementation Details

### Bot Changes (`bot/bot.go`)

The bot processes messages in two phases:
1. `handleRegularMessage()` - Sends messages to wildcard handlers
2. `handleCommand()` - Processes prefixed commands for command handlers

Key methods:
- `eventHandler()` - Main event processing that routes messages
- `handleRegularMessage()` - Handles non-command messages for wildcard handlers  
- `handleCommand()` - Processes command messages for specific handlers

### Handler Interface (`bot/handlers/handlers.go`)

All handlers must implement:
```go
type Handler interface {
    HandleMessage(msg *events.Message) error
    GetHelp() HandlerHelp
    Name() string
    Topics() []string
}
```

For wildcard handlers, `Topics()` must return `["*"]`.

### Registry System (`bot/handlers/registry.go`)

The registry uses the `Topics()` method to determine message routing:
- `GetHandlersForMessage()` - Gets all handlers that should receive a message
- `shouldReceiveMessage()` - Determines if a handler should receive based on topics
- Wildcard handlers (`"*"` topic) receive all messages
- Command handlers receive messages matching their specific topics

### WASM Handler Updates (`bot/handlers/wasm_handler.go`)

Enhanced `WasmHandler` to:
- Call `get_topics` exported function to determine handler subscriptions
- Support wildcard plugins that return `["*"]` from their topics
- Pass full message content to plugins for processing

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
        c, _ := client.GetClient()
        c.SendText(msg.Info.Chat, "👋 Hello there!")
    }
    
    return nil
}

func (h *MyWildcardHandler) Name() string {
    return "wildcard"
}

func (h *MyWildcardHandler) Topics() []string {
    return []string{"*"}
}

func (h *MyWildcardHandler) GetHelp() handlers.HandlerHelp {
    return handlers.HandlerHelp{
        Name: "wildcard",
        Description: "Monitors all messages",
        Usage: "Automatic",
        Examples: []string{"Any message triggers logging"},
        Category: "utility",
    }
}
```

### WASM Plugin Handler

The plugin interface in Go (`pkg/plugin/plugin.go`):

```go
type WildcardPlugin struct{}

func (w *WildcardPlugin) HandleMessage(input plugin.Input) plugin.Output {
    fmt.Printf("Received: %s\n", input.Message)
    
    if strings.Contains(strings.ToLower(input.Message), "bot status") {
        return plugin.Success("🤖 Bot is active!")
    }
    
    return plugin.Success("") // Silent processing
}

func (w *WildcardPlugin) Name() string {
    return "wildcard-logger"
}

func (w *WildcardPlugin) Topics() []string {
    return []string{"*"}
}

func (w *WildcardPlugin) GetHelp() plugin.HelpOutput {
    return plugin.NewHelpOutput(
        "wildcard-logger", 
        "Wildcard logger", 
        "Auto", 
        []string{}, 
        "utility"
    )
}

func (w *WildcardPlugin) GetRequiredEnvVars() []string {
    return []string{}
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

Tests are available in `bot/bot_test.go`:
- `TestWildcardHandler`: Verifies wildcard handler receives messages
- `TestWildcardHandlerWithCommandMessage`: Confirms both wildcard and command handlers execute

## Files Added/Modified

### Core Implementation
- `bot/bot.go` - Main bot with wildcard message handling
- `bot/handlers/handlers.go` - Handler interface including Topics() method
- `bot/handlers/registry.go` - Registry with topic-based message routing
- `bot/handlers/wasm_handler.go` - Enhanced for wildcard support via topics

### Examples
- `bot/handlers/wildcard.go` - Example built-in wildcard handler
- `examples/plugins/wildcard-logger/` - WASM plugin example

### Plugin Interface  
- `pkg/plugin/plugin.go` - Plugin interface with Topics() method support

### Tests
- `bot/bot_test.go` - Wildcard handler tests

## Usage Examples

### Register Built-in Handler
```go
bot := New()
bot.RegisterHandler(&MyWildcardHandler{})
```

### WASM Plugin
1. Create plugin that returns `["*"]` from `Topics()` method
2. Build with `tinygo build -o plugin.wasm -target wasi main.go`
3. Place in `~/.local/share/sup/plugins/`

### Complete Example
See `examples/plugins/wildcard-logger/main.go` for a complete working example that demonstrates:
- Message logging and counting
- Natural language responses  
- Coexistence with command handlers

## Compatibility

- ✅ Works with existing command handlers
- ✅ Compatible with WASM plugins
- ✅ Maintains all existing functionality
- ✅ Backward compatible with current bots
- ✅ All tests pass

The wildcard handler implementation provides a powerful foundation for building more intelligent and responsive WhatsApp bots through the flexible topic subscription system.