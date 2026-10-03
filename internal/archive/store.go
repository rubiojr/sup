package archive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

func (a *Archive) initSchema(ctx context.Context) error {
	var version int64
	if err := a.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return fmt.Errorf("unsupported archive schema version %d", version)
	}
	if _, err := a.db.ExecContext(ctx, "PRAGMA journal_mode=DELETE"); err != nil {
		return err
	}
	_, err := a.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS chats (
			jid TEXT PRIMARY KEY, name TEXT NOT NULL, is_group INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS messages (
			key TEXT PRIMARY KEY, chat_jid TEXT NOT NULL REFERENCES chats(jid),
			message_id TEXT NOT NULL, sender_jid TEXT NOT NULL, sender_name TEXT NOT NULL,
			timestamp INTEGER NOT NULL, from_me INTEGER NOT NULL, kind TEXT NOT NULL,
			text TEXT NOT NULL, reply_to TEXT NOT NULL, edited INTEGER NOT NULL,
			view_once INTEGER NOT NULL, ephemeral INTEGER NOT NULL, payload BLOB NOT NULL
		);
		CREATE INDEX IF NOT EXISTS messages_chat_time ON messages(chat_jid, timestamp);
		CREATE TABLE IF NOT EXISTS attachments (
			key TEXT PRIMARY KEY, message_key TEXT NOT NULL REFERENCES messages(key),
			kind TEXT NOT NULL, mime_type TEXT NOT NULL, original_name TEXT NOT NULL,
			declared_size TEXT NOT NULL, path TEXT NOT NULL, descriptor BLOB NOT NULL,
			state TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0, sha256 TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS attachments_pending ON attachments(state, next_attempt);
		PRAGMA user_version=1;
		UPDATE attachments SET state='pending', next_attempt=0 WHERE state='downloading';
		UPDATE attachments SET state='pending', attempts=0, next_attempt=0, last_error=''
		WHERE state='skipped' AND last_error IN ('file_size_limit', 'media_storage_limit', 'queue_full');
	`)
	return err
}

// Record persists a bounded snapshot before waking the media worker. It never
// invokes bot handlers and does not perform network I/O.
func (a *Archive) Record(ctx context.Context, event *events.Message) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.closed {
		return errors.New("archive is closed")
	}
	if event == nil || !a.accepts(event.Info.Chat) || event.Message == nil || event.Message.GetProtocolMessage() != nil {
		return nil
	}
	rec, media, err := snapshot(event)
	if err != nil {
		return err
	}
	tx, err := a.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertMessage(ctx, tx, rec); err != nil {
		return err
	}
	for index, item := range media {
		if err := insertAttachment(ctx, tx, rec, item, index); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("saving archive message: %w", err)
	}
	select {
	case a.wake <- struct{}{}:
	default: // The durable queue, not this notification, owns pending downloads.
	}
	return nil
}

func insertMessage(ctx context.Context, tx *sql.Tx, rec messageRecord) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, name, is_group) VALUES (?, ?, ?)
		ON CONFLICT(jid) DO UPDATE SET name = CASE WHEN chats.name = '' THEN excluded.name ELSE chats.name END`,
		rec.chat, rec.chatName, rec.group)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO messages
		(key, chat_jid, message_id, sender_jid, sender_name, timestamp, from_me, kind, text, reply_to, edited, view_once, ephemeral, payload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(key) DO NOTHING`,
		rec.key, rec.chat, rec.id, rec.sender, rec.senderName, rec.timestamp, rec.fromMe, rec.kind,
		rec.text, rec.replyTo, rec.edited, rec.viewOnce, rec.ephemeral, rec.payload)
	return err
}

func insertAttachment(ctx context.Context, tx *sql.Tx, rec messageRecord, item mediaItem, index int) error {
	key := digest(rec.key + "/" + strconv.Itoa(index))
	path := "chats/" + digest(rec.chat) + "/media/" + key + mediaExtension(item.kind, item.mime)
	descriptor, err := json.Marshal(item.descriptor)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO attachments
		(key, message_key, kind, mime_type, original_name, declared_size, path, descriptor, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending') ON CONFLICT(key) DO NOTHING`,
		key, rec.key, item.kind, item.mime, item.name, strconv.FormatUint(item.size, 10), path, descriptor)
	return err
}

type downloadJob struct {
	key, path  string
	descriptor mediaDescriptor
	attempts   int
}

func (a *Archive) nextJob(ctx context.Context) (*downloadJob, error) {
	var job downloadJob
	var encoded []byte
	err := a.db.QueryRowContext(ctx, `SELECT key, path, descriptor, attempts FROM attachments
		WHERE state IN ('pending', 'retry') AND next_attempt <= ? ORDER BY rowid LIMIT 1`, a.now().Unix()).
		Scan(&job.key, &job.path, &encoded, &job.attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(encoded, &job.descriptor); err != nil {
		return nil, fmt.Errorf("decoding archive media descriptor: %w", err)
	}
	_, err = a.db.ExecContext(ctx, "UPDATE attachments SET state='downloading', attempts=attempts+1 WHERE key=?", job.key)
	job.attempts++
	return &job, err
}

func (a *Archive) finishJob(job *downloadJob, state, reason string, size int64, hash string) error {
	// Preserve retry state even when shutdown cancelled the download context.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var nextAttempt int64
	if state == "retry" {
		nextAttempt = a.now().Add(time.Duration(job.attempts) * time.Minute).Unix()
	}
	_, err := a.db.ExecContext(ctx, `UPDATE attachments SET state=?, last_error=?, size_bytes=?, sha256=?, next_attempt=? WHERE key=?`,
		state, reason, size, hash, nextAttempt, job.key)
	return err
}
