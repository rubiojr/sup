# Passive chat archive

The archive stores observed WhatsApp conversations and attachments independently
of bot command permissions. You can archive everyone's messages while leaving
`allow.users` and `allow.groups` empty.

## Enable

Build and install Sup with `go install ./cmd/sup`. In
`~/.config/sup/bot.toml`, set:

```toml
[archive]
enabled = true
scope = "all"
```

Restart `sup bot run` to apply archive configuration changes. Use `scope =
"direct"` for direct chats only, or `scope = "groups"` for groups only. The
default scope is `all`; archiving starts disabled. Status updates, broadcasts
addressed to broadcast JIDs, and channels aren't archived.

The archive only records data and downloads attachments. It doesn't reply, mark
messages read, execute commands, dispatch plugins, or grant anyone bot access.
Bot commands and plugins use the bot's allow-list. Auto-reply follows its own
configured scope. The built-in download handlers process allow-listed chats.

## What's stored

- Incoming messages and your outgoing messages observed by the bot.
- Successful text and media sends made by the running bot, including auto-replies.
- Chat JIDs and known contact/group names, sender identity, timestamps, message
  IDs, text, captions, and reply-to message IDs.
- Images, videos, video notes, audio/voice notes, documents of any MIME type,
  stickers, and sticker packs.
- Original filenames as metadata. Disk filenames are generated from hashed IDs.
- Bounded protobuf snapshots for message types such as contacts, locations,
  reactions, and polls. These don't require running a bot handler.
- Edit events as separate revisions when WhatsApp identifies them as edits.

Repeated delivery of the same chat/sender/message ID doesn't create another
message or download. Known phone-number/LID aliases are normalized. View-once
and ephemeral flags are retained in the catalog; the local archive is persistent.

Capture starts when the bot attaches its event listener. This isn't a historical
chat export: messages from before capture or while disconnected are only stored
if WhatsApp subsequently delivers them as message events. Separate `sup send`
processes are captured only if the running bot observes their outgoing events.

## Storage

```text
~/.local/share/sup/archive/
├── archive.db
├── writer-lock.db
└── chats/
    └── <chat-jid-hash>/
        └── media/
            └── <attachment-id-hash>.pdf
```

New directories are private (`0700`); databases and attachment files are `0600`.
SQLite tracks three tables:

| Table | Contents |
| --- | --- |
| `chats` | JID, display name, and group flag |
| `messages` | Conversation metadata, text, reply links, and protobuf payload |
| `attachments` | Message reference, MIME type, original filename, relative path, size, hash, and download state |

`messages.timestamp` is Unix time in milliseconds. Attachment paths are relative
to the archive directory. `attachments.message_key` joins to `messages.key`;
`messages.chat_jid` joins to `chats.jid`.

Attachments remain opaque bytes. Sup doesn't open, render, extract, or execute
them. Downloads stream through WhatsApp's attachment API, with integrity checking
and decryption, into an `os.Root`-confined temporary file. A complete
file is renamed into place only after successful download.

Available disk space and the filesystem/SQLite limits determine archive capacity.

The archive configuration is:

```toml
[archive]
enabled = true
scope = "all"
# dir = "/absolute/path/to/archive"
```

## Concurrency and retries

The archive has one download worker. Pending jobs and retry state live in SQLite,
and only one job is loaded at a time, so backlog growth doesn't fill an in-memory
queue. Attachments are streamed and hashed with fixed-size buffers. Shutdown can
interrupt both downloading and hashing large files.

Network failures get up to three archive-level attempts
with increasing delays; WhatsApp's downloader may also retry requests within an
attempt. Each attempt has a two-minute deadline. Shutdown cancels the active
download and preserves retry state. Only one process can own an archive directory.

Message snapshots are limited to 1 MiB to bound per-message memory use; this is
independent of attachment size.

The archive doesn't delete old conversations or media to make space. Attachment
states are `pending`, `downloading`, `retry`, `ready`, or `failed`, with failure
categories in `last_error`. Real I/O failures, such as a full disk, follow the
retry path. If new database records cannot be written, the bot logs the error;
a download-worker database failure stops the bot. Free disk space and restart
to resume capture.
