# Sup

WhatsApp CLI and Bot framework.

## Features

### WhatsApp command line interface

- List all WhatsApp groups and contacts
- Send files, images and clipboard to individual users or groups
- QR code authentication for WhatsApp Web
- A sample but useful bot

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

# Send clipboard content to a group
sup send-clipboard -t 120363123456789@g.us --group

# Plugin management
# List available plugins from registry
sup plugins plugin-list

# Download and install a plugin
sup plugins plugin-download weather

# Download specific version of a plugin
sup plugins plugin-download weather 1.0.0

# Remove an installed plugin
sup plugins plugin-remove weather

# List currently loaded plugins
sup plugins list

# Reload all plugins after installing/removing
sup plugins reload

# Build a plugin registry index from a directory structure
sup index-registry /path/to/plugins https://registry-url.com

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
