# Hello JavaScript - Sup WASM Plugin Example

A simple hello world plugin written in JavaScript demonstrating the basic plugin interface for the Sup WhatsApp bot framework.

## Overview

This example shows how to create WASM plugins for Sup using JavaScript, which is officially supported by Extism. JavaScript plugins offer a familiar development experience for web developers and have excellent tooling support.

## Features

- **Simple message handling**: Responds with personalized greetings
- **Input validation**: Handles empty messages gracefully
- **Error handling**: Proper JSON error responses
- **Help system**: Provides usage information
- **Cross-platform**: Runs in WASM sandbox via Extism
- **Fast compilation**: Quick build times with extism-js

## Prerequisites

- **Node.js 16+**
- **npm** (Node package manager)
- **extism-js** (JavaScript to WASM compiler)
- **binaryen**
- **extism CLI** (optional, for testing)

## Quick Start

### 1. Setup Development Environment

```bash
# Install dependencies and build tools
make setup
```

This will install:
- JavaScript dependencies from `package.json`
- `extism-js` compiler for compiling JavaScript to WASM

### 2. Build the Plugin

```bash
make build
```

This compiles `index.js` to `dist/plugin.wasm` using extism-js.

### 3. Install the Plugin

```bash
make install
```

This copies the WASM file to `~/.local/share/sup/plugins/hello-js.wasm`.

### 4. Test the Plugin

```bash
# Test with extism CLI (if installed)
make test

# Or manually test handle_message
echo '{"message":"hello world","sender":"user@s.whatsapp.net","info":{"id":"1","timestamp":1234567890,"push_name":"Alice","is_group":false}}' | \
  extism call dist/plugin.wasm handle_message --input-stdin --wasi

# Test get_help
extism call dist/plugin.wasm get_help --wasi
```

## Usage in WhatsApp

Once installed, use the plugin in WhatsApp:

```
.sup hello
.sup hello world
.sup hello from JavaScript!
```

## Code Structure

### Main Plugin Logic (`index.js`)

```javascript
function handle_message() {
  try {
    // Get and parse JSON input
    const inputData = Host.inputString();
    const data = JSON.parse(inputData);
    const message = data.message || "";
    const info = data.info || {};
    const pushName = info.push_name || "Unknown";
    
    // Generate response
    let reply;
    if (!message) {
      reply = `Hello ${pushName}! How can I help you?`;
    } else {
      reply = `Hello ${pushName}! You said: ${message}`;
    }
    
    // Return JSON response
    const response = { success: true, reply: reply };
    Host.outputString(JSON.stringify(response));
    return 0;
    
  } catch (error) {
    const errorResponse = {
      success: false,
      error: `Plugin error: ${error.message}`
    };
    Host.outputString(JSON.stringify(errorResponse));
    return 1;
  }
}

function get_help() {
  const helpInfo = {
    name: "hello",
    description: "A simple hello world plugin written in JavaScript",
    usage: ".sup hello [message]",
    examples: [".sup hello", ".sup hello world"],
    category: "examples"
  };
  
  Host.outputString(JSON.stringify(helpInfo));
  return 0;
}

// Export functions for Extism
module.exports = { handle_message, get_help };
```

### Key Concepts

1. **Host API**: Use `Host.inputString()` and `Host.outputString()` for I/O
2. **Return Codes**: Return 0 for success, 1 for errors
3. **JSON Protocol**: All communication uses the same JSON format as Go plugins
4. **Error Handling**: Wrap logic in try/catch blocks
5. **Exports**: Use `module.exports` to expose functions

### Type Definitions (`plugin.d.ts`)

```typescript
declare module 'main' {
  export function handle_message(): I32;
  export function get_help(): I32;
}
```

## Input/Output Format

### Input (handle_message)
```json
{
  "message": "user message text",
  "sender": "user@s.whatsapp.net",
  "info": {
    "id": "message_id",
    "timestamp": 1234567890,
    "push_name": "User Display Name",
    "is_group": false
  }
}
```

### Output (handle_message)
```json
{
  "success": true,
  "reply": "Hello User! You said: user message text"
}
```

Or for errors:
```json
{
  "success": false,
  "error": "Error description"
}
```

### Output (get_help)
```json
{
  "name": "hello",
  "description": "A simple hello world plugin written in JavaScript",
  "usage": ".sup hello [message]",
  "examples": [".sup hello", ".sup hello world"],
  "category": "examples"
}
```

## Available Make Targets

```bash
make build    # Build the WASM plugin
make clean    # Remove build artifacts
make install  # Build and install to plugin directory
make test     # Test with extism CLI
make deps     # Install JavaScript dependencies
make setup    # Full development environment setup
make help     # Show all available targets
```

## File Structure

```
hello-javascript/
├── index.js          # Main plugin implementation
├── plugin.d.ts       # TypeScript type definitions
├── package.json      # Node.js dependencies and scripts
├── Makefile         # Build scripts
├── README.md        # This file
└── dist/            # Build output directory
    └── plugin.wasm  # Compiled WASM plugin
```

## Comparison with Go PDK

| Aspect | JavaScript | Go PDK |
|--------|------------|---------|
| **Setup** | Node.js + extism-js | TinyGo |
| **Code** | Familiar JS syntax | Go structs and helpers |
| **Performance** | Good (V8 optimizations) | Faster (native Go) |
| **Ecosystem** | NPM packages (limited) | Go ecosystem |
| **Learning Curve** | Easy for JS developers | Go knowledge required |
| **Build Speed** | Very fast | Fast |
| **File Size** | Medium | Small |

## Advanced Examples

### Conditional Logic

```javascript
function handle_message() {
  const data = JSON.parse(Host.inputString());
  const message = data.message.toLowerCase();
  
  switch (message) {
    case "hello":
      return respond("Hello there!");
    case "help":
      return respond("I can help you with various tasks!");
    default:
      return respond(`You said: ${data.message}`);
  }
}

function respond(text) {
  Host.outputString(JSON.stringify({ success: true, reply: text }));
  return 0;
}
```

### Group vs Private Chat

```javascript
function handle_message() {
  const data = JSON.parse(Host.inputString());
  const { message, info } = data;
  const { push_name, is_group } = info;
  
  let reply;
  if (is_group) {
    reply = `Hello group! ${push_name} said: ${message}`;
  } else {
    reply = `Hello ${push_name}! You said: ${message}`;
  }
  
  Host.outputString(JSON.stringify({ success: true, reply }));
  return 0;
}
```

## Troubleshooting

### Common Issues

**1. extism-js not found**
```bash
curl -O https://raw.githubusercontent.com/extism/js-pdk/main/install.sh
sh install.sh
```

**2. Node.js/npm not available**
Install Node.js from: https://nodejs.org/

**3. extism CLI not available**
Install from: https://extism.org/docs/install

**4. Plugin not loading**
- Check file permissions on `hello-js.wasm`
- Ensure plugin directory exists: `~/.local/share/sup/plugins/`
- Check sup logs with `sup bot --debug`

**5. JSON parsing errors**
- Ensure input/output format matches exactly
- Check for proper error handling in your code
- Use `console.log()` for debugging (output visible in extism logs)

### Debug Mode

Run sup with debug logging:
```bash
sup bot --debug
```

### Manual Testing

Test individual functions:
```bash
# Test handle_message with custom input
echo '{"message":"debug test","sender":"test@s.whatsapp.net","info":{"push_name":"Debug User","is_group":false}}' | \
  extism call dist/plugin.wasm handle_message --input-stdin --wasi

# Test get_help
extism call dist/plugin.wasm get_help --wasi
```

## Performance Tips

1. **Minimize dependencies**: Only use essential npm packages
2. **Optimize JSON parsing**: Cache parsed objects when possible
3. **Use efficient string operations**: Prefer template literals over concatenation
4. **Handle errors gracefully**: Always return proper JSON responses

## Next Steps

1. **Add more functionality**: Implement complex message processing
2. **Use TypeScript**: Convert to TypeScript for better type safety
3. **Add npm packages**: Explore compatible libraries
4. **Optimize performance**: Profile and optimize hot paths
5. **Share with community**: Contribute your plugins

## Resources

- **Sup Documentation**: `../../PLUGINS.md`
- **Go PDK**: `../hello-simple/` (alternative approach)
- **Extism JS Guide**: https://extism.org/docs/quickstart/plugin-quickstart#javascript
- **Extism JS PDK**: https://github.com/extism/js-pdk
- **Node.js**: https://nodejs.org/

## License

Same as the main Sup project.
