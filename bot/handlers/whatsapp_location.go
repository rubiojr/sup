package handlers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/epheo/anytype-go"
	_ "github.com/epheo/anytype-go/client"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/cmd/sup/version"
	"github.com/rubiojr/sup/internal/log"
)

// WhatsAppLocationHandler automatically captures WhatsApp location messages
// and stores them in Anytype with coordinates, accuracy, and metadata.
//
// Environment Variables Required:
//   - ANYTYPE_API_KEY: The Anytype AppKey for authentication
//   - ANYTYPE_SPACE: The Anytype Space ID where locations will be stored
//
// The handler automatically creates a "WhatsAppLocation" type in Anytype
// if it doesn't exist, with fields for latitude, longitude, accuracy,
// sender, chat ID, and timestamp.
type WhatsAppLocationHandler struct {
	client  anytype.Client
	spaceID string
}

const typeName = "whatsapp_location"
const pageTemplateID = "bafyreictrp3obmnf6dwejy5o4p7bderaaia4bdg2psxbfzf44yya5uutge"

func NewWhatsAppLocationHandler() *WhatsAppLocationHandler {
	return &WhatsAppLocationHandler{}
}

func (h *WhatsAppLocationHandler) HandleMessage(msg *events.Message) error {
	if !h.isAnytypeAvailable() {
		log.Debug("Anytype environment variables not available, ignoring")
		return nil
	}

	loc := msg.Message.GetLocationMessage()
	if loc == nil {
		return nil
	}

	log.Debug("Received WhatsApp location message",
		"sender", msg.Info.Sender.String(),
		"chat", msg.Info.Chat.String(),
		"latitude", loc.GetDegreesLatitude(),
		"longitude", loc.GetDegreesLongitude(),
		"accuracy", loc.AccuracyInMeters)

	if h.client == nil {
		if err := h.initializeAnytype(); err != nil {
			return fmt.Errorf("failed to initialize Anytype client: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.ensureWhatsAppLocationType(ctx); err != nil {
		return fmt.Errorf("failed to ensure WhatsApp location type exists: %w", err)
	}

	if err := h.storeLocation(ctx, loc, msg); err != nil {
		return fmt.Errorf("failed to store location: %w", err)
	}

	log.Info("Successfully stored WhatsApp location",
		"latitude", loc.GetDegreesLatitude(),
		"longitude", loc.GetDegreesLongitude(),
		"sender", msg.Info.Sender.String())

	return nil
}

func (*WhatsAppLocationHandler) isAnytypeAvailable() bool {
	if os.Getenv("ANYTYPE_API_KEY") == "" {
		return false
	}

	if os.Getenv("ANYTYPE_SPACE") == "" {
		return false
	}

	return true
}

func (h *WhatsAppLocationHandler) initializeAnytype() error {
	apiKey := os.Getenv("ANYTYPE_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANYTYPE_API_KEY environment variable is required")
	}

	spaceID := os.Getenv("ANYTYPE_SPACE")
	if spaceID == "" {
		return fmt.Errorf("ANYTYPE_SPACE environment variable is required")
	}

	client := anytype.NewClient(
		anytype.WithBaseURL("http://localhost:31009"),
		anytype.WithAppKey(apiKey),
	)

	h.client = client
	h.spaceID = spaceID

	return nil
}

func (h *WhatsAppLocationHandler) ensureWhatsAppLocationType(ctx context.Context) error {
	types, err := h.client.Space(h.spaceID).Types().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list types: %w", err)
	}

	for _, objType := range types {
		fmt.Println("ensuring")
		fmt.Println(objType.Key)
		if objType.Key == typeName {
			return nil
		}
	}

	createTypeReq := anytype.CreateTypeRequest{
		Name:   "WhatsApp Location",
		Key:    typeName,
		Layout: "basic",
		Icon: &anytype.Icon{
			Format: anytype.IconFormatEmoji,
			Emoji:  "📍",
		},
		PluralName: "WhatsApp Locations",
		Properties: []anytype.PropertyDefinition{
			{
				Key:    "user",
				Name:   "User",
				Format: "text",
			},
			{
				Key:    "latitude",
				Name:   "Latitude",
				Format: "number",
			},
			{
				Key:    "longitude",
				Name:   "Longitude",
				Format: "number",
			},
			{
				Key:    "accuracy",
				Name:   "Accuracy",
				Format: "number",
			},
		},
	}

	_, err = h.client.Space(h.spaceID).Types().Create(ctx, createTypeReq)
	if err != nil {
		return fmt.Errorf("failed to create WhatsApp location type: %w", err)
	}

	return nil
}

func (h *WhatsAppLocationHandler) storeLocation(ctx context.Context, loc *waE2E.LocationMessage, msg *events.Message) error {
	latitude := loc.GetDegreesLatitude()
	longitude := loc.GetDegreesLongitude()
	accuracy := loc.AccuracyInMeters
	//sender := msg.Info.Sender.String()
	user := msg.Info.Sender.User
	//chatID := msg.Info.Chat.String()
	//timestamp := msg.Info.Timestamp.Format("2006-01-02 15:04:05")

	createReq := anytype.CreateObjectRequest{
		TypeKey:    typeName,
		Name:       fmt.Sprintf("Location from %s", user),
		Body:       "",
		TemplateID: pageTemplateID,
		Icon: &anytype.Icon{
			Format: anytype.IconFormatEmoji,
			Emoji:  "📍",
		},
		Properties: []map[string]any{
			{
				"key":  "user",
				"text": user,
			},
			{
				"key":    "latitude",
				"number": latitude,
			},
			{
				"key":    "longitude",
				"number": longitude,
			},
			{
				"key":    "accuracy",
				"number": accuracy,
			},
		},
	}

	_, err := h.client.Space(h.spaceID).Objects().Create(ctx, createReq)
	if err != nil {
		return fmt.Errorf("failed to create location object: %w", err)
	}

	return nil
}

func (h *WhatsAppLocationHandler) Name() string {
	return "whatsapp_location"
}

func (h *WhatsAppLocationHandler) Topics() []string {
	return []string{"*"}
}

func (h *WhatsAppLocationHandler) GetHelp() HandlerHelp {
	return HandlerHelp{
		Name:        "WhatsApp Location",
		Description: "Automatically stores WhatsApp location messages in Anytype",
		Usage:       "Send a location message to WhatsApp",
		Examples:    []string{"Share your location in any chat"},
		Category:    "storage",
	}
}

func (h *WhatsAppLocationHandler) Version() string {
	return version.String
}
