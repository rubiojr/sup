# Sup Bot Mode

Start bot mode to listen for messages and run command handlers:

```bash
sup bot
```

**With custom handler prefix:**
```bash
sup bot --handler ".mybot"
```

**Command options:**
- `--handler, -p`: Command prefix to trigger bot handlers (default: ".sup")

This starts an interactive bot that can respond to messages with various commands. By default, the bot responds to messages starting with ".sup", but you can customize this prefix.
Any message sent to you or to a group where you are present will be processed by the handlers.

**Built-in bot handlers:**
- `.sup ping` - Responds with "pong"
- `.sup meteo` - [Aemet](https://www.aemet.es/) weather forecast
- `.sup help` - Shows available commands

## Plugin Management

Sup supports WASM plugins that extend bot functionality. Plugins can be installed from a registry or loaded locally.

### Installing Plugins from Registry

**List available plugins:**
```bash
sup registry list
```

**Install a plugin:**
```bash
sup registry install <plugin-name>
```

**Install a specific version:**
```bash
sup registry install <plugin-name> <version>
```

**List only installed plugins:**
```bash
sup registry list --installed-only
```

**List only available (not installed) plugins:**
```bash
sup registry list --available-only
```

### Managing Local Plugins

**List all loaded plugins and handlers:**
```bash
sup plugins list
```

**Load plugins from a directory:**
```bash
sup plugins load --dir /path/to/plugins
```

**Reload all plugins:**
```bash
sup plugins reload
```

**Get detailed information about a plugin:**
```bash
sup plugins info <plugin-name>
```

**Remove an installed plugin:**
```bash
sup plugins remove <plugin-name>
```

### Plugin Storage

By default, plugins are installed to `~/.local/share/sup/plugins/`. Each plugin is a WASM file with the `.wasm` extension.

### Registry Configuration

The default registry URL can be overridden using the `--registry` flag:

```bash
sup registry list --registry https://my-custom-registry.com
sup registry install plugin-name --registry https://my-custom-registry.com
```
