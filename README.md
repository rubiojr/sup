# Sup

WhatsApp CLI and Bot framework.

## Features

### WhatsApp command line interface

- List all WhatsApp groups and contacts
- Send files, images and clipboard to individual users or groups
- QR code authentication for WhatsApp Web
- A sample but useful bot extensible via plugins (https://sup-registry.rbel.co)

See [CLI.md](docs/CLI.md) and [BOT.md](docs/BOT.md) for CLI and bot usage.

### WhatsApp bot with a pluggable architecture

- Handlers: with full access to the operating system services, written in Go
- Plugins: Sandboxed WASM modules can be developed in several languages (any language supported by [Extism](https://extism.org/docs/quickstart/plugin-quickstart)).

See [PLUGINS.md](docs/PLUGINS.md) to create your own plugins, and [HANDLERS.md](docs/HANDLERS.md) to create your own handlers.

## Quick Start

See [INSTALL.md](/docs/INSTALL.md) for the build and install instructions.

```bash
# Check version information
sup version

# Register with WhatsApp (first time only)
sup register

# Check registration status
sup status

# List all groups and find the JID you want
sup list-groups

# List contacts to find phone numbers
sup list-contacts

# Send a text message to a contact (country code without '+' plus the contact number)
sup send -t 15551234567 -m "Hello! How are you?"

# Send a text message to a group (group contacts end with @g.us or similar)
sup send -t 120363123456789@g.us -m "Hello everyone!" --group

# Send an image to a contact
sup send-image -t 15551234567 -i photo.jpg

# Send a PDF to a group
sup send-file -t 120363123456789@g.us -f document.pdf --group

# Send any file type
sup send-file -t 15551234567 -f archive.zip

# Send clipboard content as a file to a contact
sup send-clipboard -t 15551234567

# Plugin management
# List available plugins from registry
sup registry list

# Download and install a plugin
sup registry install echo

# Download specific version of a plugin
sup registry install echo 0.1.0

# Remove an installed plugin
sup plugins remove echo

# List currently loaded plugins
sup plugins list

# Getting help
sup help
sup help <command>
```

## Notes

- You must run `sup register` first to authenticate with WhatsApp
- Authentication data is stored in `~/.local/share/sup/sup.db`
- The CLI maintains a persistent connection to WhatsApp Web
- Files are uploaded to WhatsApp's servers before being sent
- Large files may take longer to upload and send
- Use `sup status` to check if you're already registered

## Troubleshooting

**"No existing session found, please run 'sup register' first"**: Run `sup register` and scan the QR code with WhatsApp on your phone

**"Invalid group JID"**: Make sure to use the full group JID and the `--group` flag

**"File does not exist"**: Check the file path is correct and accessible

**"Already registered, session exists"**: You're already authenticated. Use `sup status` to verify

**Connection issues**: Check your internet connection and try again

## Credits

Sup is made possible thanks to the following libraries:

- [whatsmeow](https://github.com/tulir/whatsmeow) - Go library for the WhatsApp Web API
- [Extism](https://extism.org) - WebAssembly plugin system for the bot framework
- [urfave/cli](https://github.com/urfave/cli) - Command line interface framework
- [mdp/qrterminal](https://github.com/mdp/qrterminal) - QR code generator for terminal output
- [ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3) - SQLite driver for database storage
- [gabriel-vasile/mimetype](https://github.com/gabriel-vasile/mimetype) - MIME type detection for files
- [tetratelabs/wazero](https://github.com/tetratelabs/wazero) - WebAssembly runtime for Go
