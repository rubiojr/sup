# Sup CLI

## Usage

### Authentication

Before using any messaging commands, you need to register and authenticate with WhatsApp:

```bash
./sup register
```

This will display a QR code in your terminal. Scan it with WhatsApp on your phone to authenticate.

You can check your registration status at any time:

```bash
./sup status
```

### Commands

#### Register

Register and authenticate with WhatsApp by scanning a QR code:

```bash
./sup register
```

This command will display a QR code that you need to scan with WhatsApp on your phone. You only need to do this once.

#### Check Status

Check if you're already registered and authenticated:

```bash
./sup status
# or use the short alias
./sup s
```

#### List Groups

List all WhatsApp groups you're a member of:

```bash
./sup list-groups
# or use the short alias
./sup lg
```

#### List Contacts

List all your WhatsApp contacts:

```bash
./sup list-contacts
# or use the short alias
./sup lc
```

#### Send Text Message

Send a text message to a user or group:

**Send to a user (by phone number):**
```bash
./sup send -t 1234567890 -m "Hello World!"
```

**Send to a group (by group JID):**
```bash
./sup send -t 120363123456789 -m "Hello everyone!" --group
```

**Command options:**
- `-t, --to`: Recipient (phone number for users, group JID for groups)
- `-m, --message`: Text message to send
- `-g, --group`: Flag to indicate sending to a group

#### Send File

Send a file to a user or group:

**Send to a user (by phone number):**
```bash
./sup send-file -t 1234567890 -f /path/to/file.txt
```

**Send to a group (by group JID):**
```bash
./sup send-file -t 120363123456789 -f /path/to/file.pdf --group
```

**Command options:**
- `-t, --to`: Recipient (phone number for users, group JID for groups)
- `-f, --file`: Path to the file to send
- `-g, --group`: Flag to indicate sending to a group

#### Send Clipboard

Send clipboard content as a file to a user or group. This command grabs the current clipboard content using `wl-paste`, detects the file type, and sends it with the appropriate extension:

**Send clipboard to a user:**
```bash
./sup send-clipboard -t 1234567890
```

**Send clipboard to a group:**
**Send to a group (by group JID):**
```bash
./sup send-clipboard -t 120363123456789 --group
```

**Command options:**
- `-t, --to`: Recipient (phone number for users, group JID for groups)
- `-g, --group`: Flag to indicate sending to a group

**Note:** Requires `wl-paste` to be installed (Wayland clipboard utility).

#### Send Image

Send an image file to a user or group:

**Send to a user (by phone number):**
```bash
./sup send-image -t 1234567890 -i /path/to/image.jpg
```

**Send to a group (by group JID):**
```bash
./sup send-image -t 120363123456789 -i /path/to/image.png --group
```

**Command options:**
- `-t, --to`: Recipient (phone number for users, group JID for groups)
- `-i, --image`: Path to the image file to send
- `-g, --group`: Flag to indicate sending to a group

**Supported image formats:** jpg, jpeg, png, gif, webp

#### Send Audio

Send an audio file to a user or group:

**Send to a user (by phone number):**
```bash
./sup send-audio -t 1234567890 -a /path/to/audio.mp3
```

**Send to a group (by group JID):**
```bash
./sup send-audio -t 120363123456789 -a /path/to/music.wav --group
```

**Command options:**
- `-t, --to`: Recipient (phone number for users, group JID for groups)
- `-a, --audio`: Path to the audio file to send
- `-g, --group`: Flag to indicate sending to a group

**Supported audio formats:** mp3, wav, m4a, ogg, aac, flac

#### Bot Mode

Start bot mode to listen for messages and run command handlers:

```bash
./sup bot
```

**Command options:**
- `-t, --trigger`: Command prefix to trigger bot handlers (default: ".sup")
- `-d, --debug`: Enable debug level logging

The bot mode allows the CLI to respond to incoming messages with various handlers for weather, reminders, file downloads, and more.

#### Plugin Management

Manage WASM plugins:

**List all plugins:**
```bash
./sup plugins list
```

**Show plugin information:**
```bash
./sup plugins info <plugin-name>
```

**Remove an installed plugin:**
```bash
./sup plugins remove <plugin-name>
```

#### Registry Management

Manage the plugin registry:

**List available plugins:**
```bash
./sup registry list
```

**Install a plugin from registry:**
```bash
./sup registry install <plugin-name> [version]
```

**Build registry index:**
```bash
./sup registry index <plugins-directory>
```

**Registry command options:**
- `--registry`: Specify registry URL (for list and install commands)
- `--installed-only`: Show only installed plugins (for list command)
- `--available-only`: Show only available plugins (for list command)
- `--output`: Output directory for index files (for index command)
- `--verbose`: Enable verbose output (for index command)
- `--debug`: Enable debugging (for install command)

#### Version Information

Display version and build information:

```bash
./sup version
# or use the short alias
./sup v
```

### Finding Recipients

#### For Users
Use phone numbers in international format without the '+' sign:
- US number +1-555-123-4567 becomes `15551234567`
- UK number +44-20-1234-5678 becomes `442012345678`

#### For Groups
1. Run `./sup list-groups` to see all group JIDs
2. Group JIDs look like: `120363123456789`

### Supported File Types

The CLI automatically detects MIME types for common file extensions:
- Text files: `.txt`
- Documents: `.pdf`, `.doc`, `.docx`
- Images: `.jpg`, `.jpeg`, `.png`, `.gif`
- Videos: `.mp4`
- Audio: `.mp3`
- Archives: `.zip`
