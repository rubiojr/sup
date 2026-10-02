package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rubiojr/sup/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func testAutoReplyGuard(t *testing.T) (*autoReplyGuard, *atomic.Int32, *time.Time) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewStore(filepath.Join(dir, "settings.db"))
	require.NoError(t, err)
	g, err := newAutoReplyGuard(filepath.Join(dir, "ledger.db"), s)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, g.db.Close()) })
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return now }
	g.started = now.Add(-time.Second)
	var sends atomic.Int32
	g.send = func(context.Context, types.JID, string, string) error { sends.Add(1); return nil }
	setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 3600})
	return g, &sends, &now
}

func setAutoReplySettings(t *testing.T, g *autoReplyGuard, cfg autoReplySettings) {
	t.Helper()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, g.store.Put([]byte("settings"), data))
}

func autoReplyMessage(now time.Time, id, user string, self bool) *events.Message {
	chat := types.NewJID(user, types.DefaultUserServer)
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat, IsFromMe: self},
			ID:            id, Timestamp: now, PushName: "Alice",
		},
		Message: &waE2E.Message{Conversation: proto.String("hello")},
	}
}

func TestAutoReplyEligibility(t *testing.T) {
	cases := []struct {
		name   string
		change func(*events.Message)
		want   int32
	}{
		{"direct", func(m *events.Message) {}, 1},
		{"self", func(m *events.Message) { m.Info.IsFromMe = true }, 1},
		{"self LID", func(m *events.Message) {
			m.Info.IsFromMe = true
			m.Info.RecipientAlt = m.Info.Chat
			m.Info.Chat = types.NewJID("456", types.HiddenUserServer)
		}, 1},
		{"self PN from LID device", func(m *events.Message) {
			m.Info.IsFromMe = true
			m.Info.Sender = types.NewJID("456", types.HiddenUserServer)
			m.Info.RecipientAlt = m.Info.Sender
		}, 1},
		{"outgoing to someone else", func(m *events.Message) {
			m.Info.IsFromMe = true
			m.Info.Sender = types.NewJID("999", types.DefaultUserServer)
		}, 0},
		{"group", func(m *events.Message) { m.Info.Chat.Server = types.GroupServer }, 0},
		{"group flag", func(m *events.Message) { m.Info.IsGroup = true }, 0},
		{"broadcast", func(m *events.Message) { m.Info.Chat.Server = types.BroadcastServer }, 0},
		{"newsletter", func(m *events.Message) { m.Info.Chat.Server = types.NewsletterServer }, 0},
		{"unknown server", func(m *events.Message) { m.Info.Chat.Server = "unknown" }, 0},
		{"multicast", func(m *events.Message) { m.Info.Multicast = true }, 0},
		{"broadcast owner", func(m *events.Message) { m.Info.BroadcastListOwner = m.Info.Chat }, 0},
		{"missing ID", func(m *events.Message) { m.Info.ID = "" }, 0},
		{"missing body", func(m *events.Message) { m.Message = nil }, 0},
		{"empty body", func(m *events.Message) { m.Message = &waE2E.Message{} }, 0},
		{"history", func(m *events.Message) { m.SourceWebMsg = &waWeb.WebMessageInfo{} }, 0},
		{"recovered", func(m *events.Message) { m.UnavailableRequestID = "retry" }, 0},
		{"edit", func(m *events.Message) { m.IsEdit = true }, 0},
		{"protocol", func(m *events.Message) { m.Message.ProtocolMessage = &waE2E.ProtocolMessage{} }, 0},
		{"reaction", func(m *events.Message) { m.Message.ReactionMessage = &waE2E.ReactionMessage{} }, 0},
		{"before startup", func(m *events.Message) { m.Info.Timestamp = m.Info.Timestamp.Add(-time.Minute) }, 0},
		{"future", func(m *events.Message) { m.Info.Timestamp = m.Info.Timestamp.Add(time.Minute) }, 0},
		{"photo", func(m *events.Message) { m.Message = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}} }, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, sends, now := testAutoReplyGuard(t)
			// Testing override must never bypass the eligibility rules.
			setAutoReplySettings(t, g, autoReplySettings{Enabled: true, Scope: "all", CooldownSeconds: 1, TestUnlimited: true})
			m := autoReplyMessage(*now, "input", "123", false)
			tc.change(m)
			require.NoError(t, g.reply(m, "away", false))
			assert.Equal(t, tc.want, sends.Load())
		})
	}
}

func TestAutoReplyScope(t *testing.T) {
	for _, tc := range []struct {
		name        string
		scope       string
		allowListed bool
		wantSends   int32
	}{
		{"legacy settings allow listed", "", true, 1},
		{"legacy settings block unlisted", "", false, 0},
		{"allow-list allows listed", "allow-list", true, 1},
		{"allow-list blocks unlisted", "allow-list", false, 0},
		{"all allows listed", "all", true, 1},
		{"all allows unlisted", "all", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, sends, now := testAutoReplyGuard(t)
			setAutoReplySettings(t, g, autoReplySettings{Enabled: true, Scope: tc.scope, CooldownSeconds: 3600})
			require.NoError(t, g.reply(autoReplyMessage(*now, "scope", "123", false), "away", tc.allowListed))
			assert.Equal(t, tc.wantSends, sends.Load())
		})
	}
	t.Run("scope is rechecked before send", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, Scope: "all", CooldownSeconds: 3600})
		msg := autoReplyMessage(*now, "changed", "123", false)
		eligible, err := g.eligible(msg, false)
		require.NoError(t, err)
		require.True(t, eligible)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, Scope: "allow-list", CooldownSeconds: 3600})
		require.NoError(t, g.reply(msg, "away", false))
		assert.Zero(t, sends.Load())
	})
	t.Run("invalid scope blocks even listed senders", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, Scope: "everyone", CooldownSeconds: 3600})
		require.Error(t, g.reply(autoReplyMessage(*now, "invalid", "123", false), "away", true))
		assert.Zero(t, sends.Load())
	})
}

func TestAutoReplyLimits(t *testing.T) {
	t.Run("per chat minute cap", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 1})
		require.NoError(t, g.reply(autoReplyMessage(*now, "first", "123", false), "away", true))
		*now = now.Add(time.Second)
		require.NoError(t, g.reply(autoReplyMessage(*now, "second", "123", false), "away", true))
		assert.EqualValues(t, 1, sends.Load())
		*now = now.Add(59 * time.Second)
		require.NoError(t, g.reply(autoReplyMessage(*now, "third", "123", false), "away", true))
		assert.EqualValues(t, 2, sends.Load())
	})
	t.Run("cooldown survives restart", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		require.NoError(t, g.reply(autoReplyMessage(*now, "first", "123", false), "away", true))
		// A second independent connection represents a restarted or parallel bot.
		var path string
		require.NoError(t, g.db.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&path))
		restarted, err := newAutoReplyGuard(path, g.store)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, restarted.db.Close()) })
		restarted.now, restarted.started, restarted.send = g.now, g.started, g.send
		require.NoError(t, restarted.reply(autoReplyMessage(*now, "second", "123", false), "away", true))
		assert.EqualValues(t, 1, sends.Load())
		*now = now.Add(time.Hour)
		require.NoError(t, restarted.reply(autoReplyMessage(*now, "third", "123", false), "away", true))
		assert.EqualValues(t, 2, sends.Load())
	})
	t.Run("per chat caps despite short cooldown", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 1})
		for i := range 6 {
			require.NoError(t, g.reply(autoReplyMessage(*now, fmt.Sprint(i), "123", false), "away", true))
			*now = now.Add(time.Minute)
		}
		assert.EqualValues(t, 5, sends.Load())
	})
	t.Run("global minute and hour caps", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, Scope: "all", CooldownSeconds: 3600})
		for minute := range 8 {
			for n := range 4 {
				id := fmt.Sprintf("%d-%d", minute, n)
				require.NoError(t, g.reply(autoReplyMessage(*now, id, id, false), "away", false))
			}
			if minute == 0 {
				assert.EqualValues(t, 3, sends.Load())
			}
			*now = now.Add(time.Minute)
		}
		assert.EqualValues(t, 20, sends.Load())
	})
	t.Run("self override retains dedup and loop prevention", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 3600, TestUnlimited: true})
		var outgoingID string
		g.send = func(_ context.Context, _ types.JID, _, id string) error {
			outgoingID = id
			sends.Add(1)
			return nil
		}
		for i := range 25 {
			msg := autoReplyMessage(*now, fmt.Sprint(i), "123", true)
			require.NoError(t, g.reply(msg, ".sup ping", true))
			require.NoError(t, g.reply(msg, ".sup ping", true))
			require.NoError(t, g.reply(autoReplyMessage(*now, outgoingID, "123", true), ".sup ping", true))
		}
		assert.EqualValues(t, 25, sends.Load())
		handler := &WasmHandler{autoReply: g}
		generated, err := handler.IsAutoReply(outgoingID)
		require.NoError(t, err)
		assert.True(t, generated)
		// The test override neither exempts others nor consumes their quota.
		for i := range 4 {
			id := fmt.Sprintf("other%d", i)
			require.NoError(t, g.reply(autoReplyMessage(*now, id, id, false), "away", true))
		}
		assert.EqualValues(t, 28, sends.Load())
	})
	t.Run("PN and LID share cooldown", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		require.NoError(t, g.reply(autoReplyMessage(*now, "pn", "123", false), "away", true))
		msg := autoReplyMessage(*now, "lid", "456", false)
		msg.Info.Chat.Server = types.HiddenUserServer
		msg.Info.SenderAlt = types.NewJID("123", types.DefaultUserServer)
		require.NoError(t, g.reply(msg, "away", true))
		assert.EqualValues(t, 1, sends.Load())
	})
}

func TestAutoReplyFailureAndDryRun(t *testing.T) {
	t.Run("ledger remains bounded in test mode", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 1, TestUnlimited: true})
		_, err := g.db.Exec(`WITH RECURSIVE seq(n) AS (
			SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n < 10000
		) INSERT INTO replies SELECT 'in' || n, 'out' || n, '123', ?, 0, 1 FROM seq`, now.Unix())
		require.NoError(t, err)
		require.NoError(t, g.reply(autoReplyMessage(*now, "full", "123", true), "away", true))
		assert.Zero(t, sends.Load())
		*now = now.Add(24 * time.Hour)
		require.NoError(t, g.reply(autoReplyMessage(*now, "expired", "123", true), "away", true))
		assert.EqualValues(t, 1, sends.Load())
	})
	t.Run("failed delivery is never retried", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		g.send = func(context.Context, types.JID, string, string) error {
			sends.Add(1)
			return errors.New("uncertain delivery")
		}
		msg := autoReplyMessage(*now, "first", "123", false)
		require.ErrorContains(t, g.reply(msg, "away", true), "no retry")
		require.NoError(t, g.reply(msg, "away", true))
		require.NoError(t, g.reply(autoReplyMessage(*now, "next", "123", false), "away", true))
		assert.EqualValues(t, 1, sends.Load())
	})
	t.Run("dry run simulates cooldown without using live quota", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 3600, DryRun: true})
		require.NoError(t, g.reply(autoReplyMessage(*now, "dry", "123", false), "away", true))
		require.NoError(t, g.reply(autoReplyMessage(*now, "dry2", "123", false), "away", true))
		assert.Zero(t, sends.Load())
		var rows int
		require.NoError(t, g.db.QueryRow("SELECT count(*) FROM replies").Scan(&rows))
		assert.Equal(t, 1, rows)
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: 3600})
		require.NoError(t, g.reply(autoReplyMessage(*now, "live", "123", false), "away", true))
		assert.EqualValues(t, 1, sends.Load())
	})
	t.Run("invalid settings and output fail closed", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		msg := autoReplyMessage(*now, "first", "123", true)
		for _, text := range []string{" ", strings.Repeat("x", 4097), string([]byte{0xff})} {
			require.Error(t, g.reply(msg, text, true))
		}
		require.NoError(t, g.store.Put([]byte("settings"), []byte("invalid")))
		require.Error(t, g.reply(msg, "away", true))
		setAutoReplySettings(t, g, autoReplySettings{Enabled: false, CooldownSeconds: 3600})
		require.NoError(t, g.reply(msg, "away", true))
		setAutoReplySettings(t, g, autoReplySettings{Enabled: true, CooldownSeconds: -1, TestUnlimited: true})
		require.Error(t, g.reply(msg, "away", true))
		assert.Zero(t, sends.Load())
	})
	t.Run("unavailable ledger blocks sending", func(t *testing.T) {
		g, sends, now := testAutoReplyGuard(t)
		require.NoError(t, g.db.Close())
		require.Error(t, g.reply(autoReplyMessage(*now, "first", "123", true), "away", true))
		assert.Zero(t, sends.Load())
	})
}

func TestAutoReplyConcurrentReservations(t *testing.T) {
	g, sends, now := testAutoReplyGuard(t)
	var path string
	require.NoError(t, g.db.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&path))
	other, err := newAutoReplyGuard(path, g.store)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, other.db.Close()) })
	other.now, other.started, other.send = g.now, g.started, g.send
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := range 20 {
		wg.Go(func() {
			host := g
			if i%2 == 0 {
				host = other
			}
			errs <- host.reply(autoReplyMessage(*now, fmt.Sprint(i), "123", false), "away", true)
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, sends.Load())
}
