package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	kverrors "github.com/rubiojr/kv/errors"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/store"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const autoReplyMaxBytes = 4096

// Keep this wire format in sync with plugins/autoreply's settings.
type autoReplySettings struct {
	Enabled         bool   `json:"enabled"`
	Scope           string `json:"scope,omitempty"`
	DryRun          bool   `json:"dry_run"`
	CooldownSeconds int64  `json:"cooldown_seconds"`
	TestUnlimited   bool   `json:"test_unlimited"`
}

// autoReplyGuard owns the send ledger outside the plugin's writable storage.
// SQLite serializes reservations across reloads and concurrent bot processes.
type autoReplyGuard struct {
	db      *sql.DB
	store   store.Store
	started time.Time
	now     func() time.Time
	send    func(context.Context, types.JID, string, string) error
}

func newAutoReplyGuard(path string, s store.Store) (*autoReplyGuard, error) {
	u := url.URL{Scheme: "file", Path: path}
	u.RawQuery = url.Values{"_pragma": {"busy_timeout(5000)"}}.Encode()
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, fmt.Errorf("opening auto-reply ledger: %w", err)
	}
	db.SetMaxOpenConns(1)
	// The first connection also compiles the driver's SQLite WASM runtime.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS replies (
		incoming_id TEXT PRIMARY KEY,
		outgoing_id TEXT NOT NULL UNIQUE,
		chat TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		dry_run INTEGER NOT NULL,
		exempt INTEGER NOT NULL
	)`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initializing auto-reply ledger: %w", err)
	}
	return &autoReplyGuard{
		db: db, store: s, started: time.Now(), now: time.Now,
		send: func(ctx context.Context, chat types.JID, text, id string) error {
			c, err := client.GetClient()
			if err != nil {
				return err
			}
			return c.SendTextWithID(ctx, chat, text, id)
		},
	}, nil
}

func (g *autoReplyGuard) settings() (autoReplySettings, error) {
	cfg := autoReplySettings{CooldownSeconds: 3600, Scope: "allow-list"}
	data, err := g.store.Get([]byte("settings"))
	if errors.Is(err, kverrors.ErrKeyNotFound) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("reading auto-reply settings: %w", err)
	}
	if len(data) > 4096 {
		return cfg, fmt.Errorf("auto-reply settings exceed 4096 bytes")
	}
	if len(data) != 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("decoding auto-reply settings: %w", err)
		}
	}
	if cfg.CooldownSeconds < 1 || cfg.CooldownSeconds > 86400 {
		return cfg, fmt.Errorf("auto-reply cooldown must be between 1s and 24h")
	}
	if cfg.Scope != "allow-list" && cfg.Scope != "all" {
		return cfg, fmt.Errorf("auto-reply scope must be allow-list or all")
	}
	return cfg, nil
}

func isDirectChat(msg *events.Message) bool {
	chat := msg.Info.Chat
	return !msg.Info.IsGroup && !msg.Info.Multicast && !msg.Info.IsIncomingBroadcast() &&
		msg.Info.BroadcastListOwner.IsEmpty() && len(msg.Info.BroadcastRecipients) == 0 &&
		chat.User != "" && (chat.Server == types.DefaultUserServer || chat.Server == types.HiddenUserServer)
}

func isSelfChat(msg *events.Message) bool {
	chat := msg.Info.Chat.ToNonAD()
	recipientAlt := msg.Info.RecipientAlt.ToNonAD()
	sender := msg.Info.Sender.ToNonAD()
	return msg.Info.IsFromMe && isDirectChat(msg) &&
		(chat == sender || chat == msg.Info.SenderAlt.ToNonAD() ||
			(!recipientAlt.IsEmpty() && recipientAlt == sender))
}

func autoReplyChat(msg *events.Message) string {
	chat := msg.Info.Chat.ToNonAD()
	// Use the phone-number identity where available so PN/LID delivery doesn't
	// create a second cooldown bucket for the same person.
	if chat.Server == types.HiddenUserServer {
		alt := msg.Info.SenderAlt.ToNonAD()
		if msg.Info.IsFromMe {
			alt = msg.Info.RecipientAlt.ToNonAD()
		}
		if alt.User != "" && alt.Server == types.DefaultUserServer {
			return alt.String()
		}
	}
	return chat.String()
}

func autoReplyContent(msg *waE2E.Message) bool {
	return msg.GetConversation() != "" || msg.GetExtendedTextMessage() != nil ||
		msg.GetImageMessage() != nil || msg.GetVideoMessage() != nil ||
		msg.GetAudioMessage() != nil || msg.GetDocumentMessage() != nil ||
		msg.GetStickerMessage() != nil || msg.GetContactMessage() != nil ||
		msg.GetContactsArrayMessage() != nil || msg.GetLocationMessage() != nil ||
		msg.GetLiveLocationMessage() != nil || msg.GetPollCreationMessage() != nil ||
		msg.GetPollCreationMessageV2() != nil || msg.GetPollCreationMessageV3() != nil
}

func (g *autoReplyGuard) skipReason(msg *events.Message, cfg autoReplySettings, allowListed bool) string {
	switch {
	case !cfg.Enabled:
		return "disabled"
	case !isDirectChat(msg):
		return "not a direct message"
	case msg.Info.IsFromMe && !isSelfChat(msg):
		return "outgoing message to another chat"
	case !allowListed && cfg.Scope != "all":
		return "chat is not allow-listed"
	case msg.Info.ID == "" || msg.Message == nil:
		return "missing message or ID"
	case msg.SourceWebMsg != nil || msg.UnavailableRequestID != "" || msg.IsEdit || msg.Info.Edit != "":
		return "history, recovered message, or edit"
	case msg.Message.GetProtocolMessage() != nil || msg.Message.GetReactionMessage() != nil || msg.Message.GetEncReactionMessage() != nil:
		return "protocol message or reaction"
	case !autoReplyContent(msg.Message):
		return "not a supported user message"
	case msg.Info.Timestamp.Before(g.started), g.now().Sub(msg.Info.Timestamp) > 2*time.Minute,
		msg.Info.Timestamp.After(g.now().Add(30 * time.Second)):
		return "stale or future message"
	}
	return ""
}

// IsAutoReply identifies messages generated by this plugin before the bot routes
// them to any handler. In particular, a template starting with the bot trigger
// must never execute commands when it appears in the self-chat.
func (w *WasmHandler) IsAutoReply(id string) (bool, error) {
	if w.autoReply == nil {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var generated bool
	err := w.autoReply.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM replies WHERE outgoing_id = ?)", id).Scan(&generated)
	return generated, err
}

func (g *autoReplyGuard) eligible(msg *events.Message, allowListed bool) (bool, error) {
	cfg, err := g.settings()
	if err != nil {
		return false, err
	}
	if reason := g.skipReason(msg, cfg, allowListed); reason != "" {
		g.logSkip(reason, cfg)
		return false, nil
	}
	return true, nil
}

func (g *autoReplyGuard) logSkip(reason string, cfg autoReplySettings) {
	if cfg.DryRun {
		log.Info("Auto-reply skipped", "reason", reason)
	} else {
		log.Debug("Auto-reply skipped", "reason", reason)
	}
}

func (g *autoReplyGuard) reply(msg *events.Message, text string, allowListed bool) error {
	// Recheck settings after plugin execution so disabling takes effect before
	// the send, even if template rendering was slow.
	cfg, err := g.settings()
	if err != nil {
		return err
	}
	if reason := g.skipReason(msg, cfg, allowListed); reason != "" {
		g.logSkip(reason, cfg)
		return nil
	}
	if strings.TrimSpace(text) == "" || len(text) > autoReplyMaxBytes || !utf8.ValidString(text) {
		return fmt.Errorf("auto-reply must be nonempty UTF-8 text of at most %d bytes", autoReplyMaxBytes)
	}
	var randomID [16]byte
	if _, err := rand.Read(randomID[:]); err != nil {
		return fmt.Errorf("generating auto-reply ID: %w", err)
	}
	id := strings.ToUpper(hex.EncodeToString(randomID[:]))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reason, err := g.reserve(ctx, msg, cfg, id)
	if err != nil {
		return fmt.Errorf("reserving auto-reply: %w", err)
	}
	if reason != "" {
		g.logSkip(reason, cfg)
		return nil
	}
	if cfg.DryRun {
		log.Info("Auto-reply would reply", "bytes", len(text), "self_chat", isSelfChat(msg))
		return nil
	}
	// A failed or uncertain send still consumes its reservation. Never retry it.
	if err := g.send(ctx, msg.Info.Chat.ToNonAD(), text, id); err != nil {
		return fmt.Errorf("sending auto-reply (attempt retained, no retry): %w", err)
	}
	log.Info("Auto-reply sent", "self_chat", isSelfChat(msg))
	return nil
}

func (g *autoReplyGuard) reserve(ctx context.Context, msg *events.Message, cfg autoReplySettings, id string) (string, error) {
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return "", err
	}
	defer tx.Rollback() // No-op after commit; every failure keeps the send blocked.
	now := g.now().Unix()
	if _, err := tx.ExecContext(ctx, "DELETE FROM replies WHERE created_at <= ?", now-86400); err != nil {
		return "", err
	}
	var duplicate bool
	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM replies WHERE incoming_id = ? OR outgoing_id = ?)",
		msg.Info.ID, msg.Info.ID).Scan(&duplicate); err != nil {
		return "", err
	}
	if duplicate {
		return "duplicate or auto-generated message", nil
	}
	exempt := cfg.TestUnlimited && isSelfChat(msg)
	reason, err := autoReplyLimit(ctx, tx, autoReplyChat(msg), now, cfg, exempt)
	if err != nil || reason != "" {
		return reason, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO replies
		(incoming_id, outgoing_id, chat, created_at, dry_run, exempt) VALUES (?, ?, ?, ?, ?, ?)`,
		msg.Info.ID, id, autoReplyChat(msg), now, cfg.DryRun, exempt)
	if err != nil {
		return "", err
	}
	return "", tx.Commit()
}

func autoReplyLimit(ctx context.Context, tx *sql.Tx, chat string, now int64, cfg autoReplySettings, exempt bool) (string, error) {
	var total int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM replies").Scan(&total); err != nil {
		return "", err
	}
	// Bound storage even in unlimited test mode. Retention covers the maximum
	// cooldown and is much longer than the accepted incoming-message age.
	if total >= 10000 {
		return "daily ledger capacity reached", nil
	}
	if exempt {
		return "", nil
	}
	var minute, hour, chatMinute, chatHour, cooldown int
	err := tx.QueryRowContext(ctx, `SELECT
		count(CASE WHEN created_at > ? THEN 1 END),
		count(CASE WHEN created_at > ? THEN 1 END),
		count(CASE WHEN chat = ? AND created_at > ? THEN 1 END),
		count(CASE WHEN chat = ? AND created_at > ? THEN 1 END),
		count(CASE WHEN chat = ? AND created_at > ? THEN 1 END)
		FROM replies WHERE dry_run = ? AND exempt = 0`,
		now-60, now-3600, chat, now-60, chat, now-3600, chat, now-cfg.CooldownSeconds, cfg.DryRun,
	).Scan(&minute, &hour, &chatMinute, &chatHour, &cooldown)
	if err != nil {
		return "", err
	}
	switch {
	case cooldown > 0:
		return "chat cooldown", nil
	case chatMinute >= 1 || chatHour >= 5:
		return "per-chat safety limit", nil
	case minute >= 3 || hour >= 20:
		return "global safety limit", nil
	}
	return "", nil
}
