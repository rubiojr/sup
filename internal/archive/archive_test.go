package archive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type downloadFunc func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error

func (f downloadFunc) DownloadToFile(ctx context.Context, msg whatsmeow.DownloadableMessage, file whatsmeow.File) error {
	return f(ctx, msg, file)
}

func archiveConfig(t *testing.T) Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Dir = t.TempDir()
	return cfg
}

func openTestArchive(t *testing.T, cfg Config, data []byte) *Archive {
	t.Helper()
	a, err := Open(t.Context(), cfg, downloadFunc(func(_ context.Context, _ whatsmeow.DownloadableMessage, file whatsmeow.File) error {
		_, err := io.Copy(file, bytes.NewReader(data))
		return err
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	return a
}

func eventMessage(id string) *events.Message {
	jid := types.NewJID("123", types.DefaultUserServer)
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: jid, Sender: jid},
			ID:            id, PushName: "Alice", Timestamp: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC),
		},
		Message: &waE2E.Message{Conversation: proto.String("hello")},
	}
}

func documentEvent(id, name string, data []byte) *events.Message {
	event := eventMessage(id)
	hash := sha256.Sum256(data)
	event.Message = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
		DirectPath: proto.String("/attachment"), FileName: proto.String(name), Mimetype: proto.String("application/pdf"),
		FileLength: proto.Uint64(uint64(len(data))), FileSHA256: hash[:], Caption: proto.String("the attachment"),
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("quoted-message")},
	}}
	return event
}

func runNext(t *testing.T, a *Archive) *downloadJob {
	t.Helper()
	job, err := a.nextJob(t.Context())
	require.NoError(t, err)
	require.NotNil(t, job)
	require.NoError(t, a.processJob(t.Context(), job))
	return job
}

func TestArchiveConversationAndAttachments(t *testing.T) {
	data := []byte("opaque attachment")
	a := openTestArchive(t, archiveConfig(t), data)
	event := documentEvent("first", "../../escape.pdf", data)
	require.NoError(t, a.Record(t.Context(), event))
	require.NoError(t, a.Record(t.Context(), event))
	job := runNext(t, a)
	var text, replyTo, original, state, path string
	var count int
	require.NoError(t, a.db.QueryRow("SELECT count(*) FROM messages").Scan(&count))
	assert.Equal(t, 1, count)
	require.NoError(t, a.db.QueryRow("SELECT text, reply_to FROM messages").Scan(&text, &replyTo))
	assert.Equal(t, "the attachment", text)
	assert.Equal(t, "quoted-message", replyTo)
	require.NoError(t, a.db.QueryRow("SELECT original_name, state, path FROM attachments").Scan(&original, &state, &path))
	assert.Equal(t, "../../escape.pdf", original)
	assert.Equal(t, "ready", state)
	assert.NotContains(t, path, "escape")
	stored, err := a.root.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, stored)
	assert.Equal(t, int64(len(data)), a.mediaBytes)
	require.NoError(t, a.Record(t.Context(), event))
	next, err := a.nextJob(t.Context())
	require.NoError(t, err)
	assert.Nil(t, next, "redelivery must not download again")
	info, err := os.Stat(filepath.Join(a.cfg.Dir, job.path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	info, err = os.Stat(filepath.Join(a.cfg.Dir, "archive.db"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestArchiveScopeAndIdentity(t *testing.T) {
	for _, scope := range []string{"all", "direct", "groups"} {
		t.Run(scope, func(t *testing.T) {
			cfg := archiveConfig(t)
			cfg.Scope = scope
			a := openTestArchive(t, cfg, nil)
			for _, server := range []string{types.DefaultUserServer, types.GroupServer, types.BroadcastServer, types.NewsletterServer} {
				event := eventMessage(server)
				event.Info.Chat.Server = server
				require.NoError(t, a.Record(t.Context(), event))
			}
			var count int
			require.NoError(t, a.db.QueryRow("SELECT count(*) FROM messages").Scan(&count))
			want := 1
			if scope == "all" {
				want = 2
			}
			assert.Equal(t, want, count)
		})
	}
	t.Run("both sides aliases and edits", func(t *testing.T) {
		a := openTestArchive(t, archiveConfig(t), nil)
		incoming := eventMessage("message")
		require.NoError(t, a.Record(t.Context(), incoming))
		incoming.Info.SenderAlt = incoming.Info.Sender
		incoming.Info.Chat = types.NewJID("456", types.HiddenUserServer)
		incoming.Info.Sender = incoming.Info.Chat
		require.NoError(t, a.Record(t.Context(), incoming))
		outgoing := eventMessage("outgoing")
		outgoing.Info.IsFromMe = true
		outgoing.Info.Sender = types.NewJID("999", types.DefaultUserServer)
		require.NoError(t, a.Record(t.Context(), outgoing))
		outgoing.Info.RecipientAlt = outgoing.Info.Chat
		outgoing.Info.Chat = types.NewJID("456", types.HiddenUserServer)
		outgoing.Info.Sender = types.NewJID("888", types.HiddenUserServer)
		require.NoError(t, a.Record(t.Context(), outgoing))
		incoming.IsEdit = true
		incoming.Message.Conversation = proto.String("edited")
		require.NoError(t, a.Record(t.Context(), incoming))
		var count, chats, fromMe, edited int
		require.NoError(t, a.db.QueryRow("SELECT count(*), sum(from_me), sum(edited) FROM messages").Scan(&count, &fromMe, &edited))
		require.NoError(t, a.db.QueryRow("SELECT count(*) FROM chats").Scan(&chats))
		assert.Equal(t, 3, count)
		assert.Equal(t, 1, chats)
		assert.Equal(t, 1, fromMe)
		assert.Equal(t, 1, edited)
	})
}

func TestArchiveBounds(t *testing.T) {
	t.Run("pending queue", func(t *testing.T) {
		cfg := archiveConfig(t)
		cfg.MaxPending = 1
		a := openTestArchive(t, cfg, []byte("x"))
		require.NoError(t, a.Record(t.Context(), documentEvent("one", "1.pdf", []byte("x"))))
		require.NoError(t, a.Record(t.Context(), documentEvent("two", "2.pdf", []byte("x"))))
		var queued, skipped int
		require.NoError(t, a.db.QueryRow("SELECT count(*) FROM attachments WHERE state='pending'").Scan(&queued))
		require.NoError(t, a.db.QueryRow("SELECT count(*) FROM attachments WHERE last_error='queue_full'").Scan(&skipped))
		assert.Equal(t, 1, queued)
		assert.Equal(t, 1, skipped)
	})
	t.Run("actual bytes override advertised size", func(t *testing.T) {
		cfg := archiveConfig(t)
		cfg.MaxFileBytes = 8
		a := openTestArchive(t, cfg, bytes.Repeat([]byte("x"), 100))
		require.NoError(t, a.Record(t.Context(), documentEvent("lie", "file.pdf", []byte("x"))))
		job := runNext(t, a)
		var state, reason string
		require.NoError(t, a.db.QueryRow("SELECT state, last_error FROM attachments").Scan(&state, &reason))
		assert.Equal(t, "skipped", state)
		assert.Equal(t, "file_size_limit", reason)
		assert.NoFileExists(t, filepath.Join(cfg.Dir, job.path))
		assert.NoFileExists(t, filepath.Join(cfg.Dir, job.path+".part"))
		assert.Zero(t, a.mediaBytes)
	})
	t.Run("media quota", func(t *testing.T) {
		cfg := archiveConfig(t)
		cfg.MaxFileBytes, cfg.MaxMediaBytes = 64, 128
		data := bytes.Repeat([]byte("x"), 64)
		a := openTestArchive(t, cfg, data)
		for _, id := range []string{"one", "two", "three"} {
			require.NoError(t, a.Record(t.Context(), documentEvent(id, id, data)))
			runNext(t, a)
		}
		var ready, skipped int
		require.NoError(t, a.db.QueryRow("SELECT count(*) FROM attachments WHERE state='ready'").Scan(&ready))
		require.NoError(t, a.db.QueryRow("SELECT count(*) FROM attachments WHERE last_error='media_storage_limit'").Scan(&skipped))
		assert.Equal(t, 2, ready)
		assert.Equal(t, 1, skipped)
		assert.EqualValues(t, 128, a.mediaBytes)
	})
	t.Run("database quota survives replacement connection", func(t *testing.T) {
		cfg := archiveConfig(t)
		cfg.MaxDBBytes = 1 << 20
		a := openTestArchive(t, cfg, nil)
		a.db.SetMaxIdleConns(0)
		var saveErr error
		for _, id := range []string{"one", "two", "three", "four"} {
			msg := eventMessage(id)
			msg.Message.Conversation = proto.String(strings.Repeat("x", 300000))
			if saveErr = a.Record(t.Context(), msg); saveErr != nil {
				break
			}
		}
		require.Error(t, saveErr)
		info, err := os.Stat(filepath.Join(cfg.Dir, "archive.db"))
		require.NoError(t, err)
		assert.LessOrEqual(t, info.Size(), cfg.MaxDBBytes)
	})
}

func TestArchiveRestartAndRetry(t *testing.T) {
	cfg := archiveConfig(t)
	data := []byte("attachment")
	a := openTestArchive(t, cfg, data)
	require.NoError(t, a.Record(t.Context(), documentEvent("retry", "file", data)))
	a.downloader = downloadFunc(func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error {
		return errors.New("temporary failure")
	})
	job := runNext(t, a)
	require.NoError(t, a.Close())
	b := openTestArchive(t, cfg, data)
	b.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	runNext(t, b)
	var state string
	var attempts int
	require.NoError(t, b.db.QueryRow("SELECT state, attempts FROM attachments").Scan(&state, &attempts))
	assert.Equal(t, "ready", state)
	assert.Equal(t, 2, attempts)
	// Simulate a crash after rename but before the final database update.
	_, err := b.db.Exec("UPDATE attachments SET state='downloading', attempts=3")
	require.NoError(t, err)
	require.NoError(t, b.Close())
	c := openTestArchive(t, cfg, nil)
	c.downloader = downloadFunc(func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error {
		t.Error("already verified file should be recovered without downloading")
		return errors.New("unexpected download")
	})
	c.now = func() time.Time { return time.Now().Add(10 * time.Minute) }
	runNext(t, c)
	stored, err := c.root.ReadFile(job.path)
	require.NoError(t, err)
	assert.Equal(t, data, stored)
}

func TestArchiveCancellationAndWriterLock(t *testing.T) {
	cfg := archiveConfig(t)
	a := openTestArchive(t, cfg, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err := Open(ctx, cfg, a.downloader)
	require.Error(t, err, "a second process must not bypass media quota accounting")
	started := make(chan struct{})
	a.downloader = downloadFunc(func(ctx context.Context, _ whatsmeow.DownloadableMessage, _ whatsmeow.File) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	require.NoError(t, a.Record(t.Context(), documentEvent("cancel", "file", []byte("x"))))
	runCtx, stop := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- a.Run(runCtx) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not start")
	}
	stop()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
	var state string
	require.NoError(t, a.db.QueryRow("SELECT state FROM attachments").Scan(&state))
	assert.Equal(t, "retry", state)
	require.NoError(t, a.Close())
	openTestArchive(t, cfg, nil) // Closing releases the writer lock.
}

func TestArchiveSymlinkConfinement(t *testing.T) {
	cfg := archiveConfig(t)
	outside := t.TempDir()
	a := openTestArchive(t, cfg, []byte("x"))
	msg := documentEvent("symlink", "../../outside", []byte("x"))
	require.NoError(t, a.Record(t.Context(), msg))
	require.NoError(t, os.Symlink(outside, filepath.Join(cfg.Dir, "chats", digest(msg.Info.Chat.String()))))
	var calls atomic.Int32
	a.downloader = downloadFunc(func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error { calls.Add(1); return nil })
	runNext(t, a)
	assert.Zero(t, calls.Load())
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestArchiveMediaTypesAndRawMessages(t *testing.T) {
	data := []byte("payload")
	a := openTestArchive(t, archiveConfig(t), data)
	for _, tc := range []struct {
		kind string
		body *waE2E.Message
	}{
		{"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}},
		{"video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{}}},
		{"video_note", &waE2E.Message{PtvMessage: &waE2E.VideoMessage{}}},
		{"audio", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true)}}},
		{"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}},
		{"sticker_pack", &waE2E.Message{StickerPackMessage: &waE2E.StickerPackMessage{}}},
		{"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Mimetype: proto.String("application/x-unknown")}}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			event := eventMessage(tc.kind)
			event.Message = tc.body
			require.NoError(t, a.Record(t.Context(), event))
			job := runNext(t, a)
			assert.NotEmpty(t, job.descriptor.MediaType)
			var kind, state string
			require.NoError(t, a.db.QueryRow("SELECT kind, state FROM attachments WHERE key=?", job.key).Scan(&kind, &state))
			assert.Equal(t, tc.kind, kind)
			assert.Equal(t, "ready", state)
		})
	}
	contact := eventMessage("contact")
	contact.Message = &waE2E.Message{ContactMessage: &waE2E.ContactMessage{DisplayName: proto.String("Bob"), Vcard: proto.String("BEGIN:VCARD\nEND:VCARD")}}
	require.NoError(t, a.Record(t.Context(), contact))
	var payload []byte
	require.NoError(t, a.db.QueryRow("SELECT payload FROM messages WHERE message_id='contact'").Scan(&payload))
	var body waE2E.Message
	require.NoError(t, proto.Unmarshal(payload, &body))
	assert.True(t, proto.Equal(contact.Message, &body))
	reaction := eventMessage("reaction")
	reaction.Message = &waE2E.Message{
		MessageContextInfo: &waE2E.MessageContextInfo{},
		ReactionMessage:    &waE2E.ReactionMessage{Text: proto.String("👍")},
	}
	require.NoError(t, a.Record(t.Context(), reaction))
	var kind string
	require.NoError(t, a.db.QueryRow("SELECT kind FROM messages WHERE message_id='reaction'").Scan(&kind))
	assert.Equal(t, "reactionMessage", kind)
}

func TestArchiveMetadataLimitsAndNames(t *testing.T) {
	cfg := archiveConfig(t)
	cfg.MaxFileBytes = 4
	a := openTestArchive(t, cfg, nil)
	msg := documentEvent("large-file", "file", []byte("too large"))
	require.NoError(t, a.Record(t.Context(), msg))
	job, err := a.nextJob(t.Context())
	require.NoError(t, err)
	assert.Nil(t, job)
	var reason string
	require.NoError(t, a.db.QueryRow("SELECT last_error FROM attachments").Scan(&reason))
	assert.Equal(t, "file_size_limit", reason)
	msg = eventMessage("large-message")
	msg.Message.Conversation = proto.String(strings.Repeat("x", maxMessageBytes+1))
	require.Error(t, a.Record(t.Context(), msg))
	msg = eventMessage("")
	require.Error(t, a.Record(t.Context(), msg))
	msg = eventMessage("protocol")
	msg.Message = &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{}}
	require.NoError(t, a.Record(t.Context(), msg))
	group := types.NewJID("456", types.GroupServer)
	require.NoError(t, a.SetChatNames(t.Context(), map[types.JID]string{group: "Friends"}))
	msg = eventMessage("group")
	msg.Info.Chat = group
	require.NoError(t, a.Record(t.Context(), msg))
	var name string
	require.NoError(t, a.db.QueryRow("SELECT name FROM chats WHERE jid=?", group.String()).Scan(&name))
	assert.Equal(t, "Friends", name)
}

func TestLimitedFileBounds(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "bounded")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	bounded := &limitedFile{file: f, limit: 8}
	_, err = io.Copy(bounded, strings.NewReader("123456789"))
	require.ErrorIs(t, err, errFileLimit)
	_, err = bounded.WriteAt([]byte("x"), 8)
	require.ErrorIs(t, err, errFileLimit)
	_, err = bounded.WriteAt([]byte("x"), -1)
	require.ErrorIs(t, err, errFileLimit)
	require.ErrorIs(t, bounded.Truncate(9), errFileLimit)
	_, err = bounded.Seek(100, io.SeekStart)
	require.NoError(t, err)
	_, err = bounded.Write([]byte("x"))
	require.ErrorIs(t, err, errFileLimit)
	info, err := bounded.Stat()
	require.NoError(t, err)
	assert.Zero(t, info.Size())
}

func TestArchiveRetryLimit(t *testing.T) {
	a := openTestArchive(t, archiveConfig(t), nil)
	var calls int
	a.downloader = downloadFunc(func(context.Context, whatsmeow.DownloadableMessage, whatsmeow.File) error {
		calls++
		return errors.New("unavailable")
	})
	now := time.Now()
	a.now = func() time.Time { return now }
	require.NoError(t, a.Record(t.Context(), documentEvent("failure", "file", []byte("x"))))
	for range 3 {
		runNext(t, a)
		now = now.Add(10 * time.Minute)
	}
	job, err := a.nextJob(t.Context())
	require.NoError(t, err)
	assert.Nil(t, job)
	assert.Equal(t, 3, calls)
	var state string
	require.NoError(t, a.db.QueryRow("SELECT state FROM attachments").Scan(&state))
	assert.Equal(t, "failed", state)
}
