# Counter Plugin

A WhatsApp bot plugin that manages counters for users. This plugin uses the bot's store functionality to ensure counters never expire and persist across bot restarts.

## Features

- **Persistent Storage**: Counters are stored permanently using the bot's store and never expire
- **Per-User Counters**: Each user can have their own set of counters
- **Named Counters**: Support for multiple named counters per user
- **Basic Operations**: Increment, decrement, reset, and view counter values

## Commands

All commands start with `.sup counter`:

- `.sup counter` - Show the default counter value
- `.sup counter [name]` - Show the value of a named counter
- `.sup counter increment [name]` - Increment counter (aliases: `inc`, `+`)
- `.sup counter decrement [name]` - Decrement counter (aliases: `dec`, `-`)
- `.sup counter reset [name]` - Reset counter to 0
- `.sup counter list` - List all counters (not yet implemented)

If no counter name is provided, the plugin uses "default" as the counter name.

## Examples

```
.sup counter
# Output: Counter value: 0 (not set)

.sup counter increment
# Output: Counter incremented to 1

.sup counter + mycount
# Output: Counter incremented to 1

.sup counter mycount
# Output: Counter value: 1

.sup counter reset mycount
# Output: Counter reset to 0
```

## Storage Details

- **Persistence**: Uses the bot's store (no expiry) instead of cache (with expiry)
- **Storage**: Data is stored in the bot's permanent store database
- **Reliability**: Counters survive bot restarts and system reboots

## Building and Installation

### Prerequisites

- [TinyGo](https://tinygo.org/) for building WASM modules
- Go 1.21 or later

### Build

```bash
make build
```

This creates `counter.wasm` in the current directory.

### Install

```bash
make install
```

This builds the plugin and copies it to `~/.local/share/sup/plugins/`.

### Test

```bash
make test
```

Requires the [Extism CLI](https://extism.org/docs/install) to be installed.

## Technical Details

- **Language**: Go
- **Target**: WebAssembly (WASI)
- **Storage**: Bot's permanent store (SQLite-based)
- **Namespace**: Each user gets their own namespace in the store
- **Key Format**: `{sender}:{counter_name}`

## Development

The plugin demonstrates how to use the sup bot's store functionality:

```go
// Get a value from the store
data, err := plugin.GetStore(key)

// Set a value in the store
err := plugin.SetStore(key, []byte(value))
```

The store provides permanent storage without expiration, making it ideal for data that should persist across bot restarts.