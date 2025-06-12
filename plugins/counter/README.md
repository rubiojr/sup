# Counter Plugin

A simple plugin for the [sup](https://github.com/rubiojr/sup) WhatsApp bot that demonstrates the use of the cache API to create persistent counters.

## Features

- Create and manage multiple named counters
- Increment, decrement, and reset counters
- Persistent storage using the sup cache system
- Per-user counters (each user has their own set of counters)

## Installation

### Using the Makefile

```bash
# Build the plugin
make build

# Install the plugin
make install

# Or use the sup script to install
make script-install
```

### Manual Installation

```bash
# Build the plugin
tinygo build -target=wasi -o counter.wasm

# Copy to the plugins directory
mkdir -p ~/.local/share/sup/plugins
cp counter.wasm ~/.local/share/sup/plugins/
```

## Usage

The counter plugin supports the following commands:

| Command | Description |
|---------|-------------|
| `.sup counter` | Display the value of your default counter |
| `.sup counter [name]` | Display the value of a named counter |
| `.sup counter increment [name]` | Increment a counter (name is optional) |
| `.sup counter inc [name]` | Shorthand for increment |
| `.sup counter + [name]` | Shorthand for increment |
| `.sup counter decrement [name]` | Decrement a counter (name is optional) |
| `.sup counter dec [name]` | Shorthand for decrement |
| `.sup counter - [name]` | Shorthand for decrement |
| `.sup counter reset [name]` | Reset a counter to zero (name is optional) |

## Examples

```
.sup counter                     # Show default counter
.sup counter mycount             # Show counter named "mycount"
.sup counter increment           # Increment default counter
.sup counter + mycount           # Increment counter named "mycount"
.sup counter decrement mycount   # Decrement counter named "mycount"
.sup counter - mycount           # Decrement counter named "mycount"
.sup counter reset mycount       # Reset counter named "mycount" to zero
```

## Technical Details

This plugin demonstrates the use of the cache API provided by the sup bot framework. The cache is used to store counter values with the following characteristics:

- Each counter has a unique key in the format `counter:{user_id}:{counter_name}`
- Counter values are stored as simple strings
- The cache has an automatic expiry time (default is 12 hours)
- Each user has their own set of counters, isolated from other users

The plugin uses `plugin.GetCache` and `plugin.SetCache` functions from the `github.com/rubiojr/sup/pkg/plugin` package to interact with the cache.

## Contributing

Feel free to contribute to this plugin by opening issues or pull requests on the [sup GitHub repository](https://github.com/rubiojr/sup).