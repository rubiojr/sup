package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type Client struct {
	whatsmeowClient *whatsmeow.Client
}

var (
	clientInstance *Client
	once           sync.Once
	initError      error
)

func GetClient() (*Client, error) {
	once.Do(func() {
		clientInstance, initError = initClient()
	})
	return clientInstance, initError
}

func initClient() (*Client, error) {
	c := &Client{}
	dataDir := c.DataDir()
	dbFile := filepath.Join(dataDir, "sup.db")

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check if database file exists
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("not registered with WhatsApp. Please run 'sup register' first to authenticate")
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbFile), nil)
	if err != nil {
		return nil, fmt.Errorf("not registered with WhatsApp. Please run 'sup register' first to authenticate")
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	whatsmeowClient := whatsmeow.NewClient(deviceStore, nil)

	if whatsmeowClient.Store.ID == nil {
		return nil, fmt.Errorf("not registered with WhatsApp. Please run 'sup register' first to authenticate")
	}

	err = whatsmeowClient.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	c.whatsmeowClient = whatsmeowClient

	return c, nil
}

func (c *Client) DataDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(h, ".local/share/sup")
}

func (c *Client) HandlerDataPath(name string) string {
	p := filepath.Join(c.DataDir(), "handlers", name)
	// create if not exists
	if _, err := os.Stat(p); os.IsNotExist(err) {
		err = os.MkdirAll(p, 0755)
		if err != nil {
			panic(err)
		}
	}
	return p
}

func (c *Client) AddEventHandler(handler whatsmeow.EventHandler) {
	c.whatsmeowClient.AddEventHandler(handler)
}

func (c *Client) Disconnect() {
	if c.whatsmeowClient != nil {
		c.whatsmeowClient.Disconnect()
	}
}

func (c *Client) ResolveRecipient(recipient string, isGroup bool) (types.JID, error) {
	if isGroup {
		return types.ParseJID(recipient)
	}
	return types.NewJID(recipient, types.DefaultUserServer), nil
}

func (c *Client) SendText(recipientJID types.JID, message string) error {
	msg := &waE2E.Message{
		Conversation: proto.String(message),
	}

	_, err := c.whatsmeowClient.SendMessage(context.Background(), recipientJID, msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (c *Client) SendFile(recipientJID types.JID, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	uploaded, err := c.whatsmeowClient.Upload(context.Background(), data, whatsmeow.MediaDocument)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	fileName := filepath.Base(filePath)
	mimeType := getMimeType(filePath)

	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileLength:    proto.Uint64(uint64(len(data))),
			FileSHA256:    uploaded.FileSHA256,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileName:      proto.String(fileName),
			Mimetype:      proto.String(mimeType),
		},
	}

	_, err = c.whatsmeowClient.SendMessage(context.Background(), recipientJID, msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

func (c *Client) SendImage(recipientJID types.JID, imagePath string) error {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}

	uploaded, err := c.whatsmeowClient.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("failed to upload image: %w", err)
	}

	mimeType := getMimeType(imagePath)

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileLength:    proto.Uint64(uint64(len(data))),
			FileSHA256:    uploaded.FileSHA256,
			FileEncSHA256: uploaded.FileEncSHA256,
			Mimetype:      proto.String(mimeType),
		},
	}

	_, err = c.whatsmeowClient.SendMessage(context.Background(), recipientJID, msg)
	if err != nil {
		return fmt.Errorf("failed to send image message: %w", err)
	}

	return nil
}

func (c *Client) SendAudio(recipientJID types.JID, audioPath string) error {
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	uploaded, err := c.whatsmeowClient.Upload(context.Background(), data, whatsmeow.MediaAudio)
	if err != nil {
		return fmt.Errorf("failed to upload audio: %w", err)
	}

	mimeType := getMimeType(audioPath)

	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			FileLength:    proto.Uint64(uint64(len(data))),
			FileSHA256:    uploaded.FileSHA256,
			FileEncSHA256: uploaded.FileEncSHA256,
			Mimetype:      proto.String(mimeType),
			PTT:           proto.Bool(false), // Set to true for voice notes
		},
	}

	_, err = c.whatsmeowClient.SendMessage(context.Background(), recipientJID, msg)
	if err != nil {
		return fmt.Errorf("failed to send audio message: %w", err)
	}

	return nil
}

func (c *Client) GetJoinedGroups() ([]*types.GroupInfo, error) {
	return c.whatsmeowClient.GetJoinedGroups(context.Background())
}

func (c *Client) GetAllContacts() (map[types.JID]types.ContactInfo, error) {
	return c.whatsmeowClient.Store.Contacts.GetAllContacts(context.Background())
}

func (c *Client) Download(msg whatsmeow.DownloadableMessage) ([]byte, error) {
	return c.whatsmeowClient.Download(context.Background(), msg)
}

func (c *Client) Register() error {
	if c.whatsmeowClient.Store.ID != nil {
		return fmt.Errorf("already registered, session exists")
	}

	fmt.Println("Starting WhatsApp registration...")
	qrChan, _ := c.whatsmeowClient.GetQRChannel(context.Background())
	err := c.whatsmeowClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	for evt := range qrChan {
		if evt.Event == "code" {
			fmt.Println("Scan the QR code below with WhatsApp:")
			config := qrterminal.Config{
				HalfBlocks: true,
				Level:      qrterminal.M,
				Writer:     os.Stdout,
			}
			qrterminal.GenerateWithConfig(evt.Code, config)
		} else {
			fmt.Printf("Login event: %s\n", evt.Event)
			if evt.Event == "success" {
				fmt.Println("Successfully registered with WhatsApp!")
				return nil
			}
		}
	}

	return fmt.Errorf("registration failed")
}

func NewClientForRegistration() (*Client, error) {
	c := &Client{}
	dataDir := c.DataDir()
	dbFile := filepath.Join(dataDir, "sup.db")

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	container, err := sqlstore.New(context.Background(), "sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", dbFile), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	whatsmeowClient := whatsmeow.NewClient(deviceStore, nil)
	c.whatsmeowClient = whatsmeowClient

	return c, nil
}

func getMimeType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/m4a"
	case ".ogg":
		return "audio/ogg"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".zip":
		return "application/zip"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "application/octet-stream"
	}
}
