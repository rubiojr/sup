# Writing Handlers

This document explains how to write handlers for the Sup WhatsApp bot. Handlers are Go structs that implement the `Handler` interface and respond to specific commands or messages.

As opposed to plugins, handlers are not sandboxed, and they run in the same process as the bot. This means they have access to the operating system services and cand interact with the network, the local filesystem, etc.

Handlers are ideal candidates when you want to extend your bot and give the extension unrestricted access.

## Handler Interface

All handlers must implement the `Handler` interface defined in `bot/handlers/handlers.go`:

```go
type Handler interface {
    HandleMessage(msg *events.Message) error
    GetHelp() HandlerHelp
    Name() string
    Topics() []string
}
```

### Interface Methods

- **`HandleMessage(msg *events.Message) error`**: Processes incoming messages
- **`GetHelp() HandlerHelp`**: Returns help information for the handler
- **`Name() string`**: Returns the handler's unique name
- **`Topics() []string`**: Returns topics this handler subscribes to

### HandlerHelp Structure

```go
type HandlerHelp struct {
    Name        string
    Description string
    Usage       string
    Examples    []string
    Category    string
}
```

## Basic Handler Example

Here's a minimal handler that responds to a command:

```go
package handlers

import (
    "fmt"
    "go.mau.fi/whatsmeow/types/events"
    "github.com/rubiojr/sup/internal/client"
)

type EchoHandler struct{}

func (h *EchoHandler) Name() string {
    return "echo"
}

func (h *EchoHandler) Topics() []string {
    return []string{"echo"}
}

func (h *EchoHandler) HandleMessage(msg *events.Message) error {
    // Extract message text
    var messageText string
    if msg.Message.GetConversation() != "" {
        messageText = msg.Message.GetConversation()
    } else if msg.Message.GetExtendedTextMessage() != nil {
        messageText = msg.Message.GetExtendedTextMessage().GetText()
    }

    // Parse command arguments
    parts := strings.Fields(messageText)
    var text string
    if len(parts) > 2 {
        text = strings.Join(parts[2:], " ")
    }

    // Get WhatsApp client
    c, err := client.GetClient()
    if err != nil {
        return fmt.Errorf("error getting client: %w", err)
    }

    // Send response
    if text == "" {
        c.SendText(msg.Info.Chat, "Please provide text to echo")
    } else {
        c.SendText(msg.Info.Chat, fmt.Sprintf("You said: %s", text))
    }

    return nil
}

func (h *EchoHandler) GetHelp() HandlerHelp {
    return HandlerHelp{
        Name:        "echo",
        Description: "Echo back the provided text",
        Usage:       ".sup echo <text>",
        Examples:    []string{".sup echo hello world"},
        Category:    "utility",
    }
}
```

## Handler Registration

To register your handler with the bot:

```go
bot := New()
bot.RegisterHandler(&EchoHandler{})
```

## Common Patterns

### Extracting Message Text

```go
func extractMessageText(msg *events.Message) string {
    if msg.Message.GetConversation() != "" {
        return msg.Message.GetConversation()
    }
    if msg.Message.GetExtendedTextMessage() != nil {
        return msg.Message.GetExtendedTextMessage().GetText()
    }
    return ""
}
```

### Parsing Command Arguments

```go
func parseArgs(messageText string) []string {
    parts := strings.Fields(messageText)
    if len(parts) > 2 {
        return parts[2:] // Skip ".sup" and command name
    }
    return []string{}
}

func parseArgsAsString(messageText string) string {
    parts := strings.Fields(messageText)
    if len(parts) > 2 {
        return strings.Join(parts[2:], " ")
    }
    return ""
}
```

### Sending Responses

```go
func sendResponse(msg *events.Message, response string) error {
    c, err := client.GetClient()
    if err != nil {
        return fmt.Errorf("error getting client: %w", err)
    }
    return c.SendText(msg.Info.Chat, response)
}
```

## Advanced Handler Example

Here's a more complex handler that demonstrates argument parsing and error handling:

```go
type CalculatorHandler struct{}

func (h *CalculatorHandler) Name() string {
    return "calc"
}

func (h *CalculatorHandler) Topics() []string {
    return []string{"calc", "calculate"}
}

func (h *CalculatorHandler) HandleMessage(msg *events.Message) error {
    messageText := extractMessageText(msg)
    args := parseArgs(messageText)

    c, err := client.GetClient()
    if err != nil {
        return fmt.Errorf("error getting client: %w", err)
    }

    if len(args) < 3 {
        c.SendText(msg.Info.Chat, "Usage: .sup calc <number> <operator> <number>\nExample: .sup calc 5 + 3")
        return nil
    }

    num1, err := strconv.ParseFloat(args[0], 64)
    if err != nil {
        c.SendText(msg.Info.Chat, fmt.Sprintf("Invalid first number: %s", args[0]))
        return nil
    }

    operator := args[1]

    num2, err := strconv.ParseFloat(args[2], 64)
    if err != nil {
        c.SendText(msg.Info.Chat, fmt.Sprintf("Invalid second number: %s", args[2]))
        return nil
    }

    var result float64
    switch operator {
    case "+":
        result = num1 + num2
    case "-":
        result = num1 - num2
    case "*":
        result = num1 * num2
    case "/":
        if num2 == 0 {
            c.SendText(msg.Info.Chat, "Error: Division by zero")
            return nil
        }
        result = num1 / num2
    default:
        c.SendText(msg.Info.Chat, fmt.Sprintf("Unknown operator: %s\nSupported: +, -, *, /", operator))
        return nil
    }

    c.SendText(msg.Info.Chat, fmt.Sprintf("%.2f %s %.2f = %.2f", num1, operator, num2, result))
    return nil
}

func (h *CalculatorHandler) GetHelp() HandlerHelp {
    return HandlerHelp{
        Name:        "calc",
        Description: "Perform basic mathematical calculations",
        Usage:       ".sup calc <number> <operator> <number>",
        Examples: []string{
            ".sup calc 5 + 3",
            ".sup calc 10 - 4",
            ".sup calc 6 * 7",
            ".sup calc 15 / 3",
        },
        Category: "utility",
    }
}
```

## Client Methods

The WhatsApp client provides several methods for sending content:

### Send Text Message
```go
c.SendText(recipientJID, "Hello, World!")
```

### Send Image
```go
c.SendImage(recipientJID, "/path/to/image.jpg")
```

### Send File
```go
c.SendFile(recipientJID, "/path/to/document.pdf")
```

## Message Information

The `events.Message` struct contains useful information:

```go
// Chat information
chatJID := msg.Info.Chat        // JID of the chat
senderJID := msg.Info.Sender    // JID of the sender
pushName := msg.Info.PushName   // Display name of sender
timestamp := msg.Info.Timestamp // Message timestamp

// Check if it's a group chat
isGroup := msg.Info.Chat.Server == types.GroupServer
```

## Topic Subscription

Handlers subscribe to messages using the `Topics()` method:

### Command Handler
```go
func (h *MyHandler) Topics() []string {
    return []string{"mycommand"}  // Only receives ".sup mycommand"
}
```

### Multi-Command Handler
```go
func (h *MyHandler) Topics() []string {
    return []string{"cmd1", "cmd2", "cmd3"}  // Receives multiple commands
}
```

### Wildcard Handler
```go
func (h *MyHandler) Topics() []string {
    return []string{"*"}  // Receives ALL messages
}
```

## Categories

Use these standard categories in your `GetHelp()` method:

- `"basic"` - Essential bot commands (ping, help)
- `"utility"` - Useful tools and utilities
- `"fun"` - Entertainment and games

## Error Handling

Always handle errors gracefully:

```go
func (h *MyHandler) HandleMessage(msg *events.Message) error {
    c, err := client.GetClient()
    if err != nil {
        return fmt.Errorf("error getting client: %w", err)
    }

    // Handle business logic errors by sending user-friendly messages
    if someCondition {
        c.SendText(msg.Info.Chat, "❌ Something went wrong. Please try again.")
        return nil // Don't return error for user input issues
    }

    // Return errors only for system/infrastructure issues
    err = c.SendText(msg.Info.Chat, "Success!")
    if err != nil {
        return fmt.Errorf("error sending response: %w", err)
    }

    return nil
}
```

## Testing Your Handler

Create a simple test to verify your handler works:

```go
func TestMyHandler(t *testing.T) {
    handler := &MyHandler{}

    // Test handler metadata
    if handler.Name() != "expected-name" {
        t.Errorf("Expected name 'expected-name', got '%s'", handler.Name())
    }

    topics := handler.Topics()
    if len(topics) != 1 || topics[0] != "expected-topic" {
        t.Errorf("Expected topics ['expected-topic'], got %v", topics)
    }

    help := handler.GetHelp()
    if help.Name == "" {
        t.Error("Help name should not be empty")
    }
}
```

## Best Practices

1. **Keep handlers focused**: Each handler should have a single, clear purpose
2. **Validate input**: Always validate user input and provide helpful error messages
3. **Handle errors gracefully**: Send user-friendly messages for user errors, return system errors
4. **Use appropriate categories**: Choose the right category for your handler
5. **Provide good help text**: Include clear usage instructions and examples
6. **Test your handlers**: Write tests to ensure your handlers work correctly
7. **Log important events**: Use `fmt.Printf` for debugging and important events
8. **Be mindful of rate limits**: Don't send too many messages too quickly

## File Organization

Place your handlers in appropriate locations:

- **Built-in handlers**: `bot/handlers/`
- **CLI-specific handlers**: `cmd/sup/handlers/`
- **Plugin handlers**: Use WASM plugins instead

Register built-in handlers in the bot's `RegisterDefaultHandlers()` method, or register them manually when creating your bot instance.
