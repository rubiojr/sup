package client

import (
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

func TestSentMessageObserver(t *testing.T) {
	c := &Client{}
	var observed []*events.Message
	c.SetSentMessageObserver(func(msg *events.Message) { observed = append(observed, msg) })
	to := types.NewJID("123", types.DefaultUserServer)
	body := &waE2E.Message{Conversation: proto.String("reply")}
	resp := whatsmeow.SendResponse{ID: "reserved-id", Sender: types.NewJID("999", types.DefaultUserServer), Timestamp: time.Now()}
	c.notifySent(to, body, resp)
	require.Len(t, observed, 1)
	assert.Equal(t, resp.ID, observed[0].Info.ID)
	assert.Equal(t, to, observed[0].Info.Chat)
	assert.True(t, observed[0].Info.IsFromMe)
	assert.Equal(t, resp.Timestamp, observed[0].Info.Timestamp)
	assert.True(t, proto.Equal(body, observed[0].Message))
	// The nil WhatsApp connection returns a send failure without any I/O.
	require.Error(t, c.SendTextWithID(t.Context(), to, "failed", "failed-id"))
	assert.Len(t, observed, 1, "failed sends must not become successful archive messages")
	c.SetSentMessageObserver(nil)
	c.notifySent(to, body, resp)
	assert.Len(t, observed, 1)
}
