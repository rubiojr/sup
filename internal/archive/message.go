package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const maxMessageBytes = 1 << 20

type messageRecord struct {
	key, chat, chatName, id, sender, senderName, kind, text, replyTo string
	timestamp                                                        int64
	group, fromMe, edited, viewOnce, ephemeral                       bool
	payload                                                          []byte
}

type mediaDescriptor struct {
	DirectPath    string              `json:"direct_path"`
	MediaKey      []byte              `json:"media_key"`
	FileHash      []byte              `json:"file_hash"`
	EncryptedHash []byte              `json:"encrypted_hash"`
	MediaType     whatsmeow.MediaType `json:"media_type"`
}

func (m *mediaDescriptor) GetDirectPath() string             { return m.DirectPath }
func (m *mediaDescriptor) GetMediaKey() []byte               { return m.MediaKey }
func (m *mediaDescriptor) GetFileSHA256() []byte             { return m.FileHash }
func (m *mediaDescriptor) GetFileEncSHA256() []byte          { return m.EncryptedHash }
func (m *mediaDescriptor) GetMediaType() whatsmeow.MediaType { return m.MediaType }

type mediaItem struct {
	kind, mime, name, caption string
	size                      uint64
	context                   *waE2E.ContextInfo
	descriptor                mediaDescriptor
}

type downloadable interface {
	whatsmeow.DownloadableMessage
	GetFileLength() uint64
}

func snapshot(event *events.Message) (messageRecord, []mediaItem, error) {
	var rec messageRecord
	if event.Info.ID == "" || len(event.Info.ID) > 1024 || len(event.Info.PushName) > 1024 {
		return rec, nil, errors.New("archive message has missing or oversized identity metadata")
	}
	if proto.Size(event.Message) > maxMessageBytes {
		return rec, nil, errors.New("archive message payload exceeds 1 MiB")
	}
	chat, sender := canonicalSource(event.Info.MessageSource)
	if len(chat.String()) > 1024 || len(sender.String()) > 1024 {
		return rec, nil, errors.New("archive message JID exceeds 1024 bytes")
	}
	rec = messageRecord{
		chat: chat.String(), id: event.Info.ID, sender: sender.String(), senderName: event.Info.PushName,
		timestamp: event.Info.Timestamp.UnixMilli(), group: chat.Server == types.GroupServer,
		fromMe: event.Info.IsFromMe, edited: event.IsEdit || event.Info.Edit != "",
		viewOnce: event.IsViewOnce, ephemeral: event.IsEphemeral,
	}
	if !rec.group && !rec.fromMe {
		rec.chatName = rec.senderName
	}
	senderKey := rec.sender
	if rec.fromMe {
		senderKey = "self" // Own-device PN/LID aliases identify the same sender.
	}
	var revision int64
	if rec.edited {
		revision = rec.timestamp
	}
	rec.key = digest(fmt.Sprintf("%q/%q/%q/%d", rec.chat, rec.id, senderKey, revision))
	var err error
	rec.payload, err = proto.Marshal(event.Message)
	if err != nil {
		return rec, nil, fmt.Errorf("encoding archive message: %w", err)
	}
	media := extractMedia(event.Message)
	rec.kind, rec.text, rec.replyTo = messageContent(event.Message, media)
	return rec, media, nil
}

func canonicalSource(source types.MessageSource) (types.JID, types.JID) {
	chat, sender := source.Chat.ToNonAD(), source.Sender.ToNonAD()
	alt := source.SenderAlt.ToNonAD()
	if sender.Server == types.HiddenUserServer && alt.Server == types.DefaultUserServer && alt.User != "" {
		sender = alt
	}
	if source.IsFromMe {
		alt = source.RecipientAlt.ToNonAD()
	}
	if chat.Server == types.HiddenUserServer && alt.Server == types.DefaultUserServer && alt.User != "" {
		chat = alt
	}
	return chat, sender
}

func extractMedia(msg *waE2E.Message) []mediaItem {
	var media []mediaItem
	if msg.ImageMessage != nil {
		media = append(media, describeMedia("image", msg.ImageMessage))
	}
	if msg.VideoMessage != nil {
		media = append(media, describeMedia("video", msg.VideoMessage))
	}
	if msg.PtvMessage != nil {
		media = append(media, describeMedia("video_note", msg.PtvMessage))
	}
	if msg.AudioMessage != nil {
		media = append(media, describeMedia("audio", msg.AudioMessage))
	}
	if msg.DocumentMessage != nil {
		media = append(media, describeMedia("document", msg.DocumentMessage))
	}
	if msg.StickerMessage != nil {
		media = append(media, describeMedia("sticker", msg.StickerMessage))
	}
	if msg.StickerPackMessage != nil {
		media = append(media, describeMedia("sticker_pack", msg.StickerPackMessage))
	}
	return media
}

func describeMedia(kind string, msg downloadable) mediaItem {
	item := mediaItem{kind: kind, size: msg.GetFileLength(), descriptor: mediaDescriptor{
		DirectPath: msg.GetDirectPath(), MediaKey: msg.GetMediaKey(), FileHash: msg.GetFileSHA256(),
		EncryptedHash: msg.GetFileEncSHA256(), MediaType: whatsmeow.GetMediaType(msg),
	}}
	if m, ok := msg.(interface{ GetMimetype() string }); ok {
		item.mime = m.GetMimetype()
	}
	if m, ok := msg.(interface{ GetFileName() string }); ok {
		item.name = m.GetFileName()
	}
	if m, ok := msg.(interface{ GetCaption() string }); ok {
		item.caption = m.GetCaption()
	}
	if m, ok := msg.(interface{ GetContextInfo() *waE2E.ContextInfo }); ok {
		item.context = m.GetContextInfo()
	}
	return item
}

func messageContent(msg *waE2E.Message, media []mediaItem) (kind, text, replyTo string) {
	if len(media) != 0 {
		return media[0].kind, media[0].caption, media[0].context.GetStanzaID()
	}
	if msg.Conversation != nil {
		return "text", msg.GetConversation(), ""
	}
	if msg.ExtendedTextMessage != nil {
		return "text", msg.ExtendedTextMessage.GetText(), msg.ExtendedTextMessage.GetContextInfo().GetStanzaID()
	}
	// Preserve less common message types as protobuf, with their type indexed.
	kind = "unknown"
	msg.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Kind() != protoreflect.MessageKind || field.IsList() || field.IsMap() || field.Name() == "messageContextInfo" {
			return true
		}
		kind = string(field.Name())
		if m, ok := value.Message().Interface().(interface{ GetContextInfo() *waE2E.ContextInfo }); ok {
			replyTo = m.GetContextInfo().GetStanzaID()
		}
		return false
	})
	if msg.ContactMessage != nil {
		text = msg.ContactMessage.GetDisplayName()
	}
	if msg.LocationMessage != nil {
		text = fmt.Sprintf("%f,%f", msg.LocationMessage.GetDegreesLatitude(), msg.LocationMessage.GetDegreesLongitude())
	}
	if msg.ReactionMessage != nil {
		text, replyTo = msg.ReactionMessage.GetText(), msg.ReactionMessage.GetKey().GetID()
	}
	return
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func mediaExtension(kind, contentType string) string {
	parsed, _, _ := mime.ParseMediaType(contentType)
	known := map[string]string{
		"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp",
		"video/mp4": ".mp4", "audio/ogg": ".ogg", "audio/mpeg": ".mp3", "audio/mp4": ".m4a",
		"audio/wav": ".wav", "application/pdf": ".pdf", "application/zip": ".zip", "text/plain": ".txt",
	}
	if ext := known[parsed]; ext != "" {
		return ext
	}
	if kind == "sticker" {
		return ".webp"
	}
	return ".bin"
}
