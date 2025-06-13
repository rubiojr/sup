# WASM Plugin Support

Sup now supports WASM plugins using [Extism](https://extism.org/), allowing users to write handlers in multiple programming languages that run in a secure sandboxed environment.

## Overview

- **Built-in handlers**: Written in Go, compiled into the main binary
- **WASM plugins**: Written in various languages, compiled to WASM, loaded at runtime
- **Plugin directory**: `$HOME/.local/share/sup/plugins/`
- **Auto-loading**: Plugins are automatically loaded on bot startup
- **Hot reload**: Plugins can be reloaded without restarting the bot

## Quick Start

### 1. Write a Plugin (Go)

```go
package main

import (
    "fmt"
    "github.com/rubiojr/sup/pkg/plugin"
)

type HelloPlugin struct{}

func (h *HelloPlugin) Name() string {
    return "hello"
}

func (h *HelloPlugin) Topics() []string {
    return []string{"hello"}
}

func (h *HelloPlugin) HandleMessage(input plugin.Input) plugin.Output {
    return plugin.Success(fmt.Sprintf("Hello %s! You said: %s",
        input.Info.PushName, input.Message))
}

func (h *HelloPlugin) GetHelp() plugin.HelpOutput {
    return plugin.NewHelpOutput(
        "hello",
        "A simple hello world plugin",
        ".sup hello [message]",
        []string{".sup hello", ".sup hello world"},
        "examples",
    )
}

func (h *HelloPlugin) GetRequiredEnvVars() []string {
    return []string{}
}

func init() {
    plugin.RegisterPlugin(&HelloPlugin{})
}

func main() {}
```

### 2. Build and Install

```bash
# Install TinyGo (if not already installed)
# https://tinygo.org/getting-started/install/

# Build the plugin
tinygo build -o hello.wasm -target wasi main.go

# Install to plugin directory
mkdir -p ~/.local/share/sup/plugins
cp hello.wasm ~/.local/share/sup/plugins/
```

### 3. Use the Plugin

```bash
# List all handlers (built-in + plugins)
sup plugins list

# Use in WhatsApp
.sup hello world
```

## Plugin Management

### CLI Commands

```bash
# List all loaded plugins and handlers
sup plugins list

# Show detailed plugin information
sup plugins info <plugin-name>

# Load plugins from directory
sup plugins load --dir /path/to/plugins

# Reload all plugins
sup plugins reload
```

### Runtime Management

Plugins are automatically loaded when the bot starts. You can also reload plugins without restarting:

```bash
sup plugins reload
```

## Plugin Development

### Plugin Interface

All plugins must implement the complete Plugin interface:

```go
type Plugin interface {
    Name() string                             // Plugin identifier
    Topics() []string                         // Message topics to handle
    HandleMessage(input Input) Output         // Process messages
    GetHelp() HelpOutput                     // Provide help information
    GetRequiredEnvVars() []string            // List required environment variables
}
```

### Plugin Types

```go
// Input - data passed to your plugin
type Input struct {
    Message string      // User's message text
    Sender  string      // WhatsApp JID
    Info    MessageInfo // Message metadata
}

// MessageInfo - metadata about the message
type MessageInfo struct {
    ID        string `json:"id"`        // Unique message ID
    Timestamp int64  `json:"timestamp"` // Unix timestamp
    PushName  string `json:"push_name"` // Display name
    IsGroup   bool   `json:"is_group"`  // True if group chat
}

// Output - your plugin's response
type Output struct {
    Success bool   `json:"success"`           // Operation status
    Error   string `json:"error,omitempty"`   // Error message (if failed)
    Reply   string `json:"reply,omitempty"`   // Reply to send (if success)
}

// HelpOutput - help information structure
type HelpOutput struct {
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Usage       string   `json:"usage"`
    Examples    []string `json:"examples"`
    Category    string   `json:"category"`
}
```

### Helper Functions

```go
// Create responses
plugin.Success("Hello world!")
plugin.Error("Something went wrong")

// Create help information
plugin.NewHelpOutput(name, description, usage, examples, category)
```

### Host Functions

Plugins can interact with the host system through these functions:

```go
// Read files from the host filesystem
data, err := plugin.ReadFile("/path/to/file")

// Send images via WhatsApp
err := plugin.SendImage("recipient@s.whatsapp.net", "/path/to/image.jpg")

// List directory contents
files, err := plugin.ListDirectory("/path/to/directory")

// Key value cache (1h expiration) - for temporary data
err := plugin.SetCache("key", []byte("some value"))
value, err := plugin.GetCache("key")

// Persistent storage (no expiration) - for permanent data
err := plugin.Storage().Set("key", []byte("some value"))
value, err := plugin.Storage().Get("key")
```

#### Storage vs Cache

- **Cache**: Use for temporary data that can expire (1 hour TTL). Good for caching API responses, temporary user state, etc.
- **Storage**: Use for permanent data that should persist across bot restarts. Good for user settings, counters, persistent state, etc.

Both cache and storage are automatically namespaced per plugin, so different plugins cannot interfere with each other's data.

> [!NOTE]
> Plugins only have access to files in their plugin dir. Each WASM plugin has its own data directory under `$HOME/.local/share/sup/plugin-data/<plugin-name>`.

## Build System

### Individual Plugin Build

Each plugin directory should have a `Makefile`:

```makefile
.PHONY: build clean install

PLUGIN_NAME = hello
WASM_FILE = $(PLUGIN_NAME).wasm
PLUGIN_DIR = $(HOME)/.local/share/sup/plugins

build:
	@echo "Building $(PLUGIN_NAME) WASM plugin..."
	tinygo build -o $(WASM_FILE) -target wasi main.go

clean:
	@echo "Cleaning $(PLUGIN_NAME)..."
	rm -f $(WASM_FILE)

install: build
	@echo "Installing $(PLUGIN_NAME) to $(PLUGIN_DIR)..."
	mkdir -p $(PLUGIN_DIR)
	cp $(WASM_FILE) $(PLUGIN_DIR)/
	@echo "✅ $(PLUGIN_NAME) installed successfully"
```

### Bulk Plugin Build

Use the provided script to build and install all plugins:

```bash
script/install-plugins
```

This script:
- Builds all plugins in the `plugins/` directory in parallel
- Uses Makefile if available, otherwise tries direct tinygo build
- Provides detailed build status and error reporting
- Installs successful builds to the plugin directory

## Supported Languages

Thanks to Extism, plugins can be written in many languages:

- **Go** (recommended, with PDK)
- **Rust**
- **JavaScript/TypeScript**
- **C/C++**
- **C#/.NET**
- **Python**
- **Zig**
- **And many more...**

See [Extism documentation](https://extism.org/docs/quickstart/plugin-quickstart) for language-specific guides.

## Examples

The `examples/plugins/` directory contains several example plugins:

- **hello-simple/**: Minimal Go example using the PDK
- **hello-javascript/**: JavaScript implementation example
- **wildcard-logger/**: Advanced example for logging all messages

## Security

- Plugins run in a sandboxed WASM environment
- No direct file system access (except through host functions)
- No direct network access
- Limited to the defined plugin interface
- All communication goes through the Sup framework

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   WhatsApp      │    │   Sup Bot       │    │   WASM Plugin   │
│   Message       │───▶│   Framework     │───▶│   (Extism)      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   Built-in      │
                       │   Handlers      │
                       └─────────────────┘
```

1. WhatsApp message arrives
2. Sup framework routes to appropriate handler based on topics
3. Built-in handlers run directly in Go
4. WASM plugins run in Extism sandbox
5. Response sent back to WhatsApp

## Development Workflow

1. **Create** plugin implementing the full Plugin interface
2. **Build** with TinyGo targeting WASI
3. **Test** with Extism CLI (optional)
4. **Install** to plugin directory using Makefile
5. **Use** in WhatsApp chats

## Troubleshooting

### Common Build Errors

**Missing Name() or Topics() methods:**
```
*HelloPlugin does not implement plugin.Plugin (missing method Name)
```
Solution: Implement all required interface methods.

**Import errors:**
```
cannot find package "github.com/rubiojr/sup/pkg/plugin"
```
Solution: Ensure your plugin's go.mod requires the correct version.

### Plugin Not Loading

1. Check plugin directory: `~/.local/share/sup/plugins/`
2. Verify WASM file permissions
3. Check plugin logs with `sup plugins list`
4. Test plugin compilation with `sup plugins reload`

## Additional documentation

- **Cache service**: [PLUGINS_CACHE.md](docs/PLUGINS_CACHE.md)
