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
```bash
./sup send-clipboard -t 120363123456789 --group
```

**Command options:**
- `-t, --to`: Recipient (phone number for users, group JID for groups)
- `-g, --group`: Flag to indicate sending to a group

**Note:** Requires `wl-paste` to be installed (Wayland clipboard utility).

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
