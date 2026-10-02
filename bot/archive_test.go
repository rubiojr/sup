package bot

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type recordingArchive struct {
	ids []string
	err error
}

func (a *recordingArchive) Record(_ context.Context, msg *events.Message) error {
	a.ids = append(a.ids, msg.Info.ID)
	return a.err
}

func (*recordingArchive) SetChatNames(context.Context, map[types.JID]string) error { return nil }
func (*recordingArchive) Run(ctx context.Context) error                            { <-ctx.Done(); return nil }

func TestArchiveDoesNotAuthorizeMessages(t *testing.T) {
	a := &recordingArchive{}
	b, err := newTestBot(t, WithArchive(a))
	require.NoError(t, err)
	command, wildcard := &mockHandler{}, &mockWildcardHandler{}
	require.NoError(t, b.RegisterHandler(command))
	require.NoError(t, b.RegisterHandler(wildcard))
	for _, server := range []string{types.DefaultUserServer, types.GroupServer} {
		msg := createMockMessage(".sup test", "123")
		msg.Info.Chat.Server = server
		msg.Info.ID = server
		b.eventHandler(t.Context(), msg, ".sup")
	}
	media := createMockMessage("", "123")
	media.Info.ID = "media"
	media.Message = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
	b.eventHandler(t.Context(), media, ".sup")
	assert.Equal(t, []string{types.DefaultUserServer, types.GroupServer, "media"}, a.ids)
	assert.False(t, command.called)
	assert.False(t, wildcard.called)
	// Archive failures don't change permissions or prevent explicitly allowed commands.
	a.err = errors.New("disk full")
	b.eventHandler(t.Context(), createMockMessage(".sup test", "123"), ".sup")
	assert.False(t, command.called)
	b.allowedUsers = map[string]struct{}{"123@s.whatsapp.net": {}}
	b.eventHandler(t.Context(), createMockMessage(".sup test", "123"), ".sup")
	assert.True(t, command.called)
}
