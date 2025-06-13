# Reminders Handler

A WhatsApp bot handler that creates and manages active time-based reminders using natural language. This handler uses the bot's store functionality to ensure reminders persist across bot restarts and automatically checks for due reminders every second using a cron scheduler.

## Features

- **Natural Language Time Parsing**: Use phrases like "tomorrow 3pm", "in 2 hours", "next friday"
- **Active Notifications**: Automatically sends reminder messages when they become due
- **Persistent Storage**: Reminders are stored permanently using the bot's store
- **Smart Context Handling**: Individual reminders for private chats, shared reminders for group chats
- **Group Collaboration**: Any user in a group can create, view, and manage reminders for that group
- **Automatic Cleanup**: Past reminders are automatically garbage collected
- **JSON Storage**: Reminders are stored in structured JSON format
- **Cron Scheduling**: Background task checks for due reminders every second
- **Flexible Management**: List, delete individual reminders, or clear all

## Commands

All commands start with `.sup rem`:

- `.sup rem <description> @ <time>` - Create a new reminder
- `.sup rem` or `.sup rem list` - List all active reminders
- `.sup rem delete <id>` - Delete a specific reminder (use first 8 chars of ID)
- `.sup rem clear` - Clear all reminders
- `.sup rem check` - Manually check for due reminders

## Format

The handler uses the format: `<description> @ <time>`

- **Description**: Any text describing what the reminder is for
- **@ Symbol**: Required separator between description and time
- **Time**: Natural language time expression parsed by the `github.com/olebedev/when` library

### Time Format Examples

- `tomorrow 3pm` - Tomorrow at 3:00 PM
- `in 2 hours` - 2 hours from now
- `in 30 minutes` - 30 minutes from now
- `in 10 seconds` - 10 seconds from now
- `in 1 second` - 1 second from now
- `next friday` - Next Friday (default time)
- `monday 9am` - Next Monday at 9:00 AM
- `december 25 10am` - December 25th at 10:00 AM

### Complete Examples

- `.sup rem Meeting with John @ tomorrow 3pm`
- `.sup rem Call mom @ in 2 hours`
- `.sup rem Dentist appointment @ next friday 10am`
- `.sup rem Pick up groceries @ in 30 minutes`
- `.sup rem Check the oven @ in 10 seconds`
- `.sup rem Test reminder @ in 5 seconds`

## Usage Examples

### Individual Chat
```
.sup rem Meeting with John @ tomorrow 3pm
# Output: ✅ Reminder set for Tuesday, January 16, 2024 at 3:00 PM: Meeting with John

.sup rem Call mom @ in 2 hours
# Output: ✅ Reminder set for Monday, January 15, 2024 at 4:30 PM: Call mom

.sup rem Check the oven @ in 30 seconds
# Output: ✅ Reminder set for Monday, January 15, 2024 at 2:30:30 PM: Check the oven

.sup rem list
# Output: 📝 Active reminders (3):
# • Mon Jan 15, 2:30 PM [99887766]: Check the oven
# • Mon Jan 15, 4:30 PM [12345678]: Call mom
# • Tue Jan 16, 3:00 PM [87654321]: Meeting with John

.sup rem delete 12345678
# Output: ✅ Reminder deleted

.sup rem clear
# Output: ✅ All reminders cleared
```

### Group Chat
```
.sup rem Team standup @ tomorrow 9am
# Output: ✅ Reminder set for Tuesday, January 16, 2024 at 9:00 AM: Team standup

.sup rem Project deadline @ friday 5pm
# Output: ✅ Reminder set for Friday, January 19, 2024 at 5:00 PM: Project deadline

.sup rem list
# Output: 📝 Group reminders (2):
# • Tue Jan 16, 9:00 AM [87654321]: Team standup
# • Fri Jan 19, 5:00 PM [12345678]: Project deadline

.sup rem delete 87654321
# Output: ✅ Reminder deleted

.sup rem clear
# Output: ✅ All group reminders cleared
```

**Note**: In group chats, any member can see and manage all group reminders, regardless of who created them.

## Active Notifications

When a reminder becomes due, the bot automatically sends a notification **to the original chat where the reminder was created**:

```
🔔 Reminder: Meeting with John
```

**Important**: Reminders are always delivered to the same chat (individual or group) where they were originally created, regardless of where you are when the reminder triggers.

### Group vs Individual Reminders

- **Individual Chats**: Each user has their own private set of reminders
- **Group Chats**: All group members share the same pool of reminders
  - Any group member can create reminders for the group
  - Any group member can view all group reminders
  - Any group member can delete or clear group reminders
  - Group reminders are delivered to the group chat where they were created

## Storage Details

- **Persistence**: Uses the bot's store (no expiry) for permanent storage
- **Format**: JSON-encoded reminder objects
- **Individual Storage**: `{sender}:reminders` for private chats
- **Group Storage**: `group:{chat_id}:reminders` for group chats
- **Index**: Maintains a `reminder_keys_index` to track all active reminder keys
- **Cleanup**: Automatically removes reminders older than 10 seconds every 10 seconds

## Reminder Structure

Each reminder is stored as a JSON object with the following fields:

```json
{
  "id": "1642234567890123456",
  "description": "Meeting with John",
  "remind_at": "2024-01-16T15:00:00Z",
  "created_at": "2024-01-15T14:30:00Z",
  "triggered": false,
  "chat_id": "1234567890@s.whatsapp.net"
}
```

## Technical Details

- **Language**: Go
- **Storage**: Bot's permanent store (SQLite-based via rubiojr/kv)
- **Time Parser**: [github.com/olebedev/when](https://github.com/olebedev/when)
- **Scheduler**: [github.com/robfig/cron/v3](https://github.com/robfig/cron)
- **Cron Schedule**: `*/10 * * * * *` (every 10 seconds for monitoring and cleanup)
- **Second Precision**: Supports reminders down to 1-second accuracy
- **Key Format**: `{sender}:reminders`

## Architecture

### Handler Registration

The handler is registered in `cmd/sup/bot.go`:

```go
store, err := b.Store()
if err != nil {
    return err
}
if err := b.RegisterHandler(handlers.NewRemindersHandler(store.Namespace("reminders"))); err != nil {
    return err
}
```

### Background Processing

The handler uses `robfig/cron/v3` to run a background task every second:

```go
handler.cron.AddFunc("* * * * * *", handler.checkAllReminders)
handler.cron.Start()
```

### Reminder Key Index Management

Since the store doesn't support key enumeration, the handler maintains a separate reminder key index:

- `reminder_keys_index` - JSON array of all reminder keys (both individual and group)
- Individual keys: `{sender}` (e.g., "user@example.com")
- Group keys: `group:{chat_id}` (e.g., "group:123456@g.us") 
- Updated when reminders are added/removed
- Used by the cron job to know which keys to check for due reminders

## Dependencies

- `github.com/olebedev/when` - Natural language date/time parsing
- `github.com/robfig/cron/v3` - Cron scheduling for background tasks
- `github.com/rubiojr/sup/store` - Persistent storage interface

## Error Handling

- Missing @ separator shows format help: "Please use format: description @ time"
- Empty descriptions are rejected: "Please provide a description for the reminder"
- Empty time expressions are rejected: "Please provide a time for the reminder"
- Invalid time formats show helpful error messages with examples
- Past times are rejected with clear feedback
- Storage errors are logged and reported to users
- Cron job errors are logged but don't crash the bot
- Missing or invalid chat IDs are logged and safely skipped
- Chat ID validation prevents reminders from being created without proper delivery context
- Smart cleanup removes reminders older than 10 seconds while preserving recent ones

## Limitations

- Time parsing is in English only
- No timezone support (uses system timezone)
- Background checking runs every 10 seconds (balances performance with responsiveness)
- User enumeration requires maintaining a separate index
- No recurring reminder support
- Past reminders are kept for 10 seconds then removed (brief history for user confirmation)
- Very short reminders (under 1 second) are not supported

## Development Notes

The handler demonstrates several advanced patterns:

1. **Background Processing**: Using cron for scheduled tasks
2. **State Management**: Maintaining user indexes for efficient lookups  
3. **Error Recovery**: Graceful handling of invalid data and missing resources
4. **Natural Language Processing**: Parsing human-readable time expressions
5. **Cross-Chat Communication**: Sending reminders to the original chat context
6. **Chat Context Preservation**: Storing and validating chat IDs to ensure delivery to the correct location

### Chat ID Handling

Each reminder stores the `chat_id` field from where it was created:
- Individual chats: `1234567890@s.whatsapp.net`
- Group chats: `1234567890-5678901234@g.us`

The handler validates chat IDs during creation and logs delivery attempts for debugging.

This handler shows how to build stateful, time-aware bot functionality that operates independently of user interactions while maintaining proper chat context.