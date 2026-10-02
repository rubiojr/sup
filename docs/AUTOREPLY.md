# Auto-reply

The `autoreply` plugin replies to fresh direct messages with a text template.
It supports WhatsApp's **Message yourself** chat. Groups, broadcasts, status
updates, and channels never receive auto-replies.

## Setup

Build an updated Sup binary and install the plugins:

```sh
go install ./cmd/sup
script/install-plugins
sup plugins run autoreply status
```

The plugin must be installed as `autoreply.wasm`. It starts disabled and requires
the updated host's send protections. The `status` command also creates its data
directory on first use.

Create `~/.local/share/sup/plugin-data/autoreply/reply.txt` containing your reply:

```text
Hi {{name}}, I'm away right now. I'll get back to you later.
```

`{{name}}` expands to the sender's display name, or `there` when it's missing.
It's the only supported placeholder. Templates and rendered replies must be
nonempty UTF-8 text, at most 4096 bytes. The plugin reads the file for each reply,
so edits take effect without restarting the bot.

To answer everyone who messages you directly, preview the template, choose the
`all` scope, and enable auto-reply:

```sh
sup plugins run autoreply preview Alice
sup plugins run autoreply scope all
sup plugins run autoreply enable
sup bot run
```

## Who gets a reply

Auto-reply has its own recipient scope:

- **`all`** answers incoming direct messages from anyone, including people you
  haven't added as contacts. Your self-chat works too, without an allow-list entry.
- **`allow-list`** answers only chats listed in the bot's `allow.users` settings.
  This is the default, including for existing configurations after upgrading.

Change it at any time without restarting the bot:

```sh
sup plugins run autoreply scope all
sup plugins run autoreply scope allow-list
```

Both modes retain cooldowns, host safety caps, and the group exclusion. The
bot's allow-list still controls access to commands and other handlers. A message
from an unlisted person can trigger only the auto-reply, even if it starts with
`.sup`.

To use `allow-list` mode, add the direct chats you want to answer:

```sh
sup bot allow-list edit
sup bot run
```

In this mode, an empty allow-list permits nobody. For self-chat testing in
`allow-list` mode, include your own JID in `allow.users` in
`~/.config/sup/bot.toml`, for example:

```toml
[[allow.users]]
jid = "15551234567@s.whatsapp.net"
name = "Myself"
```

Use your actual JID. WhatsApp also uses `@lid` addresses; the allow-list matches
the chat JID shown by the bot's debug logs. Restart the bot after allow-list edits.

## Controls

These local CLI commands update persistent plugin settings. Changes take effect
without restarting the bot:

```sh
sup plugins run autoreply status
sup plugins run autoreply cooldown 2h
sup plugins run autoreply dry-run on
sup plugins run autoreply dry-run off
sup plugins run autoreply disable
```

Enabling validates the template first. Template or storage errors stop replies.
WhatsApp messages cannot change these settings.

Dry-run logs whether the bot would reply and why it skips messages. It simulates
rate limits with separate counters, so it doesn't consume the live send quota.
Use `preview` to see the actual text. Normal-mode skip reasons are debug logs.
With `scope allow-list`, messages outside the bot allow-list are skipped.

## Message yourself

Enable auto-reply and send yourself a WhatsApp message. Use `scope all` or add
your own chat to the allow-list.
Your normal outgoing messages to other people don't trigger auto-replies.

For repeated self-chat tests:

```sh
sup plugins run autoreply test-unlimited on
```

This bypasses cooldowns and rate limits **only in your self-chat**. Other direct
chats retain all limits. Duplicate suppression, stale-message filtering, and
loop prevention remain active. Turn it off when finished:

```sh
sup plugins run autoreply test-unlimited off
```

## Limits and delivery

- Default cooldown: **one reply per chat per hour**. Configurable from 1 second
  to 24 hours, in whole seconds.
- Host safety caps: **1/minute and 5/hour per chat**, plus **3/minute and 20/hour
  globally**, using rolling windows. Lowering the cooldown doesn't bypass these.
- Self-chat tests with `test-unlimited` don't consume other chats' quotas.
- Attempts and generated message IDs are recorded before sending in
  `~/.local/share/sup/autoreply.db`, outside plugin-writable storage. Restarting
  or reloading the plugin doesn't reset them. Concurrent reservations are atomic.
- Failed or uncertain sends consume their reservation and aren't retried.
- History sync, recovered messages, edits, reactions, and protocol events are
  ignored. Messages must be newer than plugin startup and no more than two
  minutes old. Text, common media, contacts, locations, and polls are supported.
- Auto-generated replies are ignored before command routing, including templates
  that start with `.sup`. Each incoming message can produce at most one attempt.
- Records expire after 24 hours. The ledger is capped at 10,000 records even in
  test mode; reaching that cap stops further attempts until records expire.

Sup enforces these checks at the auto-reply send boundary. The plugin cannot use
the image-sending or external-command host functions to bypass that boundary.
