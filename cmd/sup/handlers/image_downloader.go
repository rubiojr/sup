package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/cmd/sup/version"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
)

type ImageDownloaderHandler struct {
	downloadDir   string
	downloadCount int
}

func (i *ImageDownloaderHandler) Name() string {
	return "image-downloader"
}

func (i *ImageDownloaderHandler) Topics() []string {
	return []string{"*"}
}

func NewImageDownloaderHandler(downloadDir string) *ImageDownloaderHandler {
	// Ensure download directory exists
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Warn("Failed to create download directory", "dir", downloadDir, "error", err)
	}

	return &ImageDownloaderHandler{
		downloadDir: downloadDir,
	}
}

func (i *ImageDownloaderHandler) HandleMessage(msg *events.Message) error {
	// Check if message contains an image
	if msg.Message.GetImageMessage() != nil {
		return i.downloadImage(msg)
	}

	// Check if message contains a document that might be an image
	if msg.Message.GetDocumentMessage() != nil {
		return i.downloadDocument(msg)
	}

	// Check if message contains a sticker
	if msg.Message.GetStickerMessage() != nil {
		return i.downloadSticker(msg)
	}

	// Check if message contains a video (including GIFs)
	if msg.Message.GetVideoMessage() != nil {
		return i.downloadVideo(msg)
	}

	// No media to download
	return nil
}

func (i *ImageDownloaderHandler) downloadImage(msg *events.Message) error {
	imageMsg := msg.Message.GetImageMessage()
	if imageMsg == nil {
		return nil
	}

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	// Download the image data
	imageData, err := c.Download(imageMsg)
	if err != nil {
		log.Debug("Failed to download image", "chat", msg.Info.Chat.String(), "error", err)
		return nil // Don't return error to avoid stopping other handlers
	}

	// Determine file extension
	mimeType := imageMsg.GetMimetype()
	ext := i.getExtensionFromMimeType(mimeType)
	if ext == "" {
		ext = "jpg" // Default extension
	}

	// Create filename
	filename := i.createFilename(msg, "image", ext)
	filepath := i.getUniqueFilepath(i.downloadDir, filename, imageData)

	// If filepath is empty, it means we found a duplicate and ignored it
	if filepath == "" {
		log.Debug("Duplicate image found", "chat", msg.Info.Chat.String(), "filename", filename)
		return nil
	}

	// Save the image
	if err := os.WriteFile(filepath, imageData, 0644); err != nil {
		log.Debug("Failed to save image", "path", filepath, "error", err)
		return nil
	}

	i.downloadCount++
	actualFilename := filepath[len(i.downloadDir)+1:] // Get just the filename part
	log.Debug("Downloaded image", "count", i.downloadCount, "filename", actualFilename, "from", msg.Info.PushName)

	return nil
}

func (i *ImageDownloaderHandler) downloadDocument(msg *events.Message) error {
	docMsg := msg.Message.GetDocumentMessage()
	if docMsg == nil {
		return nil
	}

	// Check if document is an image
	mimeType := docMsg.GetMimetype()
	if !strings.HasPrefix(mimeType, "image/") {
		return nil // Not an image
	}

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	// Download the document data
	docData, err := c.Download(docMsg)
	if err != nil {
		log.Debug("Failed to download document", "chat", msg.Info.Chat.String(), "error", err)
		return nil
	}

	// Use the original filename if available
	filename := docMsg.GetFileName()
	if filename == "" {
		ext := i.getExtensionFromMimeType(mimeType)
		if ext == "" {
			ext = "jpg"
		}
		filename = i.createFilename(msg, "document", ext)
	}

	filepath := i.getUniqueFilepath(i.downloadDir, filename, docData)

	// If filepath is empty, it means we found a duplicate and ignored it
	if filepath == "" {
		log.Debug("Duplicate image found", "chat", msg.Info.Chat.String(), "filename", filename)
		return nil
	}

	// Save the document
	if err := os.WriteFile(filepath, docData, 0644); err != nil {
		log.Debug("Failed to save document", "path", filepath, "error", err)
		return nil
	}

	i.downloadCount++
	actualFilename := filepath[len(i.downloadDir)+1:] // Get just the filename part
	log.Debug("Downloaded document image", "count", i.downloadCount, "filename", actualFilename, "from", msg.Info.PushName)

	return nil
}

func (i *ImageDownloaderHandler) downloadSticker(msg *events.Message) error {
	stickerMsg := msg.Message.GetStickerMessage()
	if stickerMsg == nil {
		return nil
	}

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	// Download the sticker data
	stickerData, err := c.Download(stickerMsg)
	if err != nil {
		log.Debug("Failed to download sticker", "chat", msg.Info.Chat.String(), "error", err)
		return nil
	}

	// Determine file extension from mime type
	mimeType := stickerMsg.GetMimetype()
	ext := i.getExtensionFromMimeType(mimeType)
	if ext == "" {
		ext = "webp" // Default for stickers
	}

	// Create filename
	filename := i.createFilename(msg, "sticker", ext)
	filepath := i.getUniqueFilepath(i.downloadDir, filename, stickerData)

	// If filepath is empty, it means we found a duplicate and ignored it
	if filepath == "" {
		log.Debug("Duplicate image found", "chat", msg.Info.Chat.String(), "filename", filename)
		return nil
	}

	// Save the sticker
	if err := os.WriteFile(filepath, stickerData, 0644); err != nil {
		log.Debug("Failed to save sticker", "path", filepath, "error", err)
		return nil
	}

	i.downloadCount++
	actualFilename := filepath[len(i.downloadDir)+1:] // Get just the filename part
	log.Debug("Downloaded sticker", "count", i.downloadCount, "filename", actualFilename, "from", msg.Info.PushName)

	return nil
}

func (i *ImageDownloaderHandler) downloadVideo(msg *events.Message) error {
	videoMsg := msg.Message.GetVideoMessage()
	if videoMsg == nil {
		return nil
	}

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	// Download the video data
	videoData, err := c.Download(videoMsg)
	if err != nil {
		log.Debug("Failed to download video", "chat", msg.Info.Chat.String(), "error", err)
		return nil
	}

	// Determine file extension from mime type
	mimeType := videoMsg.GetMimetype()
	ext := i.getExtensionFromMimeType(mimeType)
	if ext == "" {
		ext = "mp4" // Default for videos
	}

	// Create filename - use "gif" as media type for GIF videos, otherwise "video"
	mediaType := "video"
	if strings.Contains(mimeType, "gif") || ext == "gif" {
		mediaType = "gif"
	}

	filename := i.createFilename(msg, mediaType, ext)
	filepath := i.getUniqueFilepath(i.downloadDir, filename, videoData)

	// If filepath is empty, it means we found a duplicate and ignored it
	if filepath == "" {
		log.Debug("Duplicate video found", "chat", msg.Info.Chat.String(), "filename", filename)
		return nil
	}

	// Save the video
	if err := os.WriteFile(filepath, videoData, 0644); err != nil {
		log.Debug("Failed to save video", "path", filepath, "error", err)
		return nil
	}

	i.downloadCount++
	actualFilename := filepath[len(i.downloadDir)+1:] // Get just the filename part
	log.Debug("Downloaded video/gif", "count", i.downloadCount, "filename", actualFilename, "from", msg.Info.PushName)

	return nil
}

func (i *ImageDownloaderHandler) createFilename(msg *events.Message, mediaType, ext string) string {
	// Create a unique filename based on message info
	timestamp := msg.Info.Timestamp.Format("20060102_150405")
	sender := i.sanitizeFilename(msg.Info.PushName)
	if sender == "" {
		sender = "unknown"
	}

	// Create hash of message ID for uniqueness
	hash := sha256.Sum256([]byte(msg.Info.ID))
	shortHash := fmt.Sprintf("%x", hash[:4])

	// Construct filename
	filename := fmt.Sprintf("%s_%s_%s_%s.%s", timestamp, sender, mediaType, shortHash, ext)

	return filename
}

func (i *ImageDownloaderHandler) sanitizeFilename(name string) string {
	// Replace invalid filename characters
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Limit length
	if len(result) > 20 {
		result = result[:20]
	}

	return result
}

func (i *ImageDownloaderHandler) getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/bmp":
		return "bmp"
	case "image/tiff":
		return "tiff"
	case "image/svg+xml":
		return "svg"
	case "video/mp4":
		return "mp4"
	case "image/gif", "video/gif":
		return "gif"
	case "video/webm":
		return "webm"
	case "video/quicktime":
		return "mov"
	case "video/x-msvideo":
		return "avi"
	default:
		return ""
	}
}

func (i *ImageDownloaderHandler) getUniqueFilepath(dir, filename string, content []byte) string {
	originalPath := filepath.Join(dir, filename)

	// If file doesn't exist, return original path
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return originalPath
	}

	// File exists, check if content is the same
	existingContent, err := os.ReadFile(originalPath)
	if err != nil {
		log.Warn("Failed to read existing file", "path", originalPath, "error", err)
		return originalPath
	}

	// Compare hashes
	newHash := sha256.Sum256(content)
	existingHash := sha256.Sum256(existingContent)

	// If content is the same, ignore and log
	if newHash == existingHash {
		log.Debug("Duplicate content detected, ignoring", "filename", filename, "sha256", fmt.Sprintf("%x", newHash[:4]))
		return ""
	}

	// Content is different, create new filename with suffix
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	log.Debug("File exists with different content, creating new version", "filename", filename)

	// Find available suffix
	counter := 1
	for {
		newFilename := fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		newPath := filepath.Join(dir, newFilename)

		// If this path doesn't exist, use it
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			log.Debug("Creating new file", "filename", newFilename)
			return newPath
		}

		// If it exists, check if content matches
		existingContent, err := os.ReadFile(newPath)
		if err != nil {
			// Can't read, try next counter
			counter++
			continue
		}

		existingHash := sha256.Sum256(existingContent)
		if newHash == existingHash {
			// Same content found in suffixed file, ignore
			log.Debug("Duplicate content detected in existing file, ignoring", "filename", newFilename, "sha256", fmt.Sprintf("%x", newHash[:4]))
			return ""
		}

		// Different content, try next counter
		counter++
	}
}

func (i *ImageDownloaderHandler) downloadFromURL(url, filename string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get the image
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download from URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer out.Close()

	// Copy data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file %s: %w", filename, err)
	}

	return nil
}

func (i *ImageDownloaderHandler) GetStats() (int, string) {
	return i.downloadCount, i.downloadDir
}

func (i *ImageDownloaderHandler) GetHelp() handlers.HandlerHelp {
	return handlers.HandlerHelp{
		Name:        "image-downloader",
		Description: "Automatically downloads all images, documents, stickers, and GIFs from messages",
		Usage:       "Automatically processes all messages with media",
		Examples: []string{
			"Send any image - it will be automatically downloaded",
			"Send any document with image content - it will be downloaded",
			"Send any sticker - it will be downloaded",
			"Send any GIF - it will be automatically downloaded",
		},
		Category: "utility",
	}
}

func (i *ImageDownloaderHandler) Version() string {
	return version.String
}
