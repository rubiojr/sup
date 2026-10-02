package handlers

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func TestAutoReplyPlugin(t *testing.T) {
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo required for WASM integration test")
	}
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "plugins")
	require.NoError(t, os.Mkdir(pluginDir, 0o750))
	wasmPath := filepath.Join(pluginDir, "autoreply.wasm")
	cmd := exec.CommandContext(t.Context(), "tinygo", "build", "-o", wasmPath, "-target", "wasi", ".")
	cmd.Dir = "../../plugins/autoreply"
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	c, err := cache.NewCache(filepath.Join(dir, "cache.db"))
	require.NoError(t, err)
	s, err := store.NewStore(filepath.Join(dir, "store.db"))
	require.NoError(t, err)
	w, err := NewWasmHandler(wasmPath, c.Namespace("autoreply"), s.Namespace("autoreply"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, w.Close()) })
	require.Equal(t, []string{"*"}, w.Topics())
	now := time.Now().Truncate(time.Second).Add(time.Second)
	w.autoReply.now = func() time.Time { return now }
	w.autoReply.started = now.Add(-time.Second)
	var replies, ids []string
	w.autoReply.send = func(_ context.Context, _ types.JID, text, id string) error {
		replies = append(replies, text)
		ids = append(ids, id)
		return nil
	}

	t.Run("disabled until configured", func(t *testing.T) {
		status, err := w.HandleCLI([]string{"status"})
		require.NoError(t, err)
		assert.Contains(t, status, "enabled: false")
		assert.Contains(t, status, "scope: allow-list")
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "disabled", "123", false)))
		assert.Empty(t, replies)
		_, err = w.HandleCLI([]string{"enable"})
		require.Error(t, err, "cannot enable without a template")
	})

	templatePath := filepath.Join(w.dataDir, "reply.txt")
	require.NoError(t, os.WriteFile(templatePath, []byte("Hi {{name}}, I'm away."), 0o600))
	t.Run("preview and enable", func(t *testing.T) {
		preview, err := w.HandleCLI([]string{"preview", "Alice"})
		require.NoError(t, err)
		assert.Equal(t, "Hi Alice, I'm away.\n", preview)
		preview, err = w.HandleCLI([]string{"preview"})
		require.NoError(t, err)
		assert.Equal(t, "Hi there, I'm away.\n", preview)
		_, err = w.HandleCLI([]string{"enable"})
		require.NoError(t, err)
	})

	t.Run("direct messages and cooldown", func(t *testing.T) {
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "first", "123", false)))
		require.Equal(t, []string{"Hi Alice, I'm away."}, replies)
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "second", "123", false)))
		assert.Len(t, replies, 1)
	})

	t.Run("self testing without a reply loop", func(t *testing.T) {
		_, err := w.HandleCLI([]string{"test-unlimited", "on"})
		require.NoError(t, err)
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "self1", "999", true)))
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "self2", "999", true)))
		require.Len(t, replies, 3)
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, ids[2], "999", true)))
		assert.Len(t, replies, 3)
	})

	t.Run("groups and outgoing DMs never receive replies", func(t *testing.T) {
		msg := autoReplyMessage(now, "group", "456", true)
		msg.Info.Chat.Server = types.GroupServer
		require.NoError(t, w.HandleMessage(msg))
		msg = autoReplyMessage(now, "outgoing", "456", true)
		msg.Info.Sender = types.NewJID("999", types.DefaultUserServer)
		require.NoError(t, w.HandleMessage(msg))
		assert.Len(t, replies, 3)
	})

	t.Run("template validation and reload", func(t *testing.T) {
		for _, text := range []string{" ", "{{unknown}}", "{{name", strings.Repeat("x", 4097)} {
			require.NoError(t, os.WriteFile(templatePath, []byte(text), 0o600))
			_, err := w.HandleCLI([]string{"preview"})
			require.Error(t, err)
		}
		require.NoError(t, os.WriteFile(templatePath, []byte("Updated, {{name}}."), 0o600))
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "self3", "999", true)))
		assert.Equal(t, "Updated, Alice.", replies[len(replies)-1])
	})

	t.Run("host protection is mandatory", func(t *testing.T) {
		input, err := json.Marshal(WasmInput{Sender: "123@s.whatsapp.net", Message: "hello"})
		require.NoError(t, err)
		_, output, err := w.call("handle_message", input)
		require.NoError(t, err)
		var result WasmOutput
		require.NoError(t, json.Unmarshal(output, &result))
		assert.False(t, result.Success)
		assert.Empty(t, result.Reply)
	})

	t.Run("scope controls unlisted direct messages", func(t *testing.T) {
		before := len(replies)
		msg := autoReplyMessage(now, "unlisted", "456", false)
		require.NoError(t, w.HandleUnlistedMessage(msg))
		assert.Len(t, replies, before)
		_, err := w.HandleCLI([]string{"scope", "everyone"})
		require.Error(t, err)
		status, err := w.HandleCLI([]string{"scope", "all"})
		require.NoError(t, err)
		assert.Contains(t, status, "scope: all")
		require.NoError(t, w.HandleUnlistedMessage(msg))
		assert.Len(t, replies, before+1)
		require.NoError(t, w.HandleUnlistedMessage(autoReplyMessage(now, "unlisted-cooldown", "456", false)))
		assert.Len(t, replies, before+1, "all scope must retain cooldowns")
		msg.Info.Chat.Server = types.GroupServer
		msg.Info.ID = "unlisted-group"
		require.NoError(t, w.HandleUnlistedMessage(msg))
		assert.Len(t, replies, before+1, "all scope must exclude groups")
		require.NoError(t, w.HandleUnlistedMessage(autoReplyMessage(now, "unlisted-self", "999", true)))
		assert.Len(t, replies, before+2)
		_, err = w.HandleCLI([]string{"scope", "allow-list"})
		require.NoError(t, err)
		require.NoError(t, w.HandleUnlistedMessage(autoReplyMessage(now, "unlisted-self-blocked", "999", true)))
		assert.Len(t, replies, before+2, "test-unlimited must not bypass scope")
	})

	t.Run("other plugins reject unlisted messages", func(t *testing.T) {
		other := &WasmHandler{name: "echo"}
		require.NoError(t, other.HandleUnlistedMessage(autoReplyMessage(now, "other-plugin", "456", false)))
	})

	t.Run("disable immediately", func(t *testing.T) {
		_, err := w.HandleCLI([]string{"disable"})
		require.NoError(t, err)
		before := len(replies)
		require.NoError(t, w.HandleMessage(autoReplyMessage(now, "disabled-again", "999", true)))
		assert.Len(t, replies, before)
	})
}

func TestWasmWildcardText(t *testing.T) {
	msg := autoReplyMessage(time.Now(), "test", "123", false)
	msg.Message.Conversation = proto.String("hello  there\nfriend")
	assert.Equal(t, "hello  there\nfriend", wasmMessageText(msg, []string{"logger", "*"}))
	msg.Message = &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(".sup echo hello there")}}
	assert.Equal(t, ".sup echo hello there", wasmMessageText(msg, []string{"*"}))
	assert.Equal(t, "hello there", wasmMessageText(msg, []string{"echo"}))
}
