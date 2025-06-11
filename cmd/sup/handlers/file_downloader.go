package handlers

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow/types/events"

	"github.com/rubiojr/sup/bot/handlers"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
)

type FileDownloaderHandler struct {
	downloadDir   string
	downloadCount int
}

func (f *FileDownloaderHandler) Name() string {
	return "file-downloader"
}

func (f *FileDownloaderHandler) Topics() []string {
	return []string{"*"}
}

func NewFileDownloaderHandler(downloadDir string) *FileDownloaderHandler {
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Warn("Failed to create download directory", "dir", downloadDir, "error", err)
	}

	return &FileDownloaderHandler{
		downloadDir: downloadDir,
	}
}

func (f *FileDownloaderHandler) HandleMessage(msg *events.Message) error {
	if msg.Message.GetDocumentMessage() != nil {
		return f.downloadDocument(msg)
	}

	return nil
}

func (f *FileDownloaderHandler) downloadDocument(msg *events.Message) error {
	docMsg := msg.Message.GetDocumentMessage()
	if docMsg == nil {
		return nil
	}

	mimeType := docMsg.GetMimetype()
	if f.isImageType(mimeType) {
		return nil
	}

	if !f.isSupportedFileType(mimeType) {
		return nil
	}

	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	docData, err := c.Download(docMsg)
	if err != nil {
		log.Debug("Failed to download document", "chat", msg.Info.Chat.String(), "error", err)
		return nil
	}

	filename := docMsg.GetFileName()
	if filename == "" {
		ext := f.getExtensionFromMimeType(mimeType)
		if ext == "" {
			ext = "bin"
		}
		filename = f.createFilename(msg, "document", ext)
	}

	filepath := f.getUniqueFilepath(f.downloadDir, filename, docData)

	if filepath == "" {
		log.Debug("Duplicate file found", "chat", msg.Info.Chat.String(), "filename", filename)
		return nil
	}

	if err := os.WriteFile(filepath, docData, 0644); err != nil {
		log.Debug("Failed to save document", "path", filepath, "error", err)
		return nil
	}

	f.downloadCount++
	actualFilename := filepath[len(f.downloadDir)+1:]
	log.Debug("Downloaded file", "count", f.downloadCount, "filename", actualFilename, "from", msg.Info.PushName)

	if f.downloadCount%10 == 0 {
		confirmMsg := fmt.Sprintf("📁 Downloaded %d files so far to %s", f.downloadCount, f.downloadDir)
		c.SendText(msg.Info.Chat, confirmMsg)
	}

	return nil
}

func (f *FileDownloaderHandler) isImageType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func (f *FileDownloaderHandler) isSupportedFileType(mimeType string) bool {
	supportedTypes := []string{
		"application/pdf",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/zip",
		"application/x-zip-compressed",
		"application/vnd.rar",
		"application/x-rar-compressed",
		"application/x-7z-compressed",
		"application/gzip",
		"application/x-tar",
		"text/plain",
		"text/csv",
		"application/json",
		"application/xml",
		"text/xml",
		"application/rtf",
		"application/epub+zip",
		"application/vnd.oasis.opendocument.text",
		"application/vnd.oasis.opendocument.spreadsheet",
		"application/vnd.oasis.opendocument.presentation",
	}

	for _, supportedType := range supportedTypes {
		if mimeType == supportedType {
			return true
		}
	}

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	return false
}

func (f *FileDownloaderHandler) createFilename(msg *events.Message, fileType, ext string) string {
	timestamp := msg.Info.Timestamp.Format("20060102_150405")
	sender := f.sanitizeFilename(msg.Info.PushName)
	if sender == "" {
		sender = "unknown"
	}

	hash := sha256.Sum256([]byte(msg.Info.ID))
	shortHash := fmt.Sprintf("%x", hash[:4])

	filename := fmt.Sprintf("%s_%s_%s_%s.%s", timestamp, sender, fileType, shortHash, ext)

	return filename
}

func (f *FileDownloaderHandler) sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}

	if len(result) > 20 {
		result = result[:20]
	}

	return result
}

func (f *FileDownloaderHandler) getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "application/pdf":
		return "pdf"
	case "application/vnd.ms-excel":
		return "xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	case "application/vnd.ms-powerpoint":
		return "ppt"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return "pptx"
	case "application/msword":
		return "doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/zip", "application/x-zip-compressed":
		return "zip"
	case "application/vnd.rar", "application/x-rar-compressed":
		return "rar"
	case "application/x-7z-compressed":
		return "7z"
	case "application/gzip":
		return "gz"
	case "application/x-tar":
		return "tar"
	case "text/plain":
		return "txt"
	case "text/csv":
		return "csv"
	case "application/json":
		return "json"
	case "application/xml", "text/xml":
		return "xml"
	case "application/rtf":
		return "rtf"
	case "application/epub+zip":
		return "epub"
	case "application/vnd.oasis.opendocument.text":
		return "odt"
	case "application/vnd.oasis.opendocument.spreadsheet":
		return "ods"
	case "application/vnd.oasis.opendocument.presentation":
		return "odp"
	default:
		return ""
	}
}

func (f *FileDownloaderHandler) getUniqueFilepath(dir, filename string, content []byte) string {
	originalPath := filepath.Join(dir, filename)

	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		return originalPath
	}

	existingContent, err := os.ReadFile(originalPath)
	if err != nil {
		log.Warn("Failed to read existing file", "path", originalPath, "error", err)
		return originalPath
	}

	newHash := sha256.Sum256(content)
	existingHash := sha256.Sum256(existingContent)

	if newHash == existingHash {
		log.Debug("Duplicate content detected, ignoring", "filename", filename, "sha256", fmt.Sprintf("%x", newHash[:4]))
		return ""
	}

	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	log.Debug("File exists with different content, creating new version", "filename", filename)

	counter := 1
	for {
		newFilename := fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		newPath := filepath.Join(dir, newFilename)

		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			log.Debug("Creating new file", "filename", newFilename)
			return newPath
		}

		existingContent, err := os.ReadFile(newPath)
		if err != nil {
			counter++
			continue
		}

		existingHash := sha256.Sum256(existingContent)
		if newHash == existingHash {
			log.Debug("Duplicate content detected in existing file, ignoring", "filename", newFilename, "sha256", fmt.Sprintf("%x", newHash[:4]))
			return ""
		}

		counter++
	}
}

func (f *FileDownloaderHandler) GetStats() (int, string) {
	return f.downloadCount, f.downloadDir
}

func (h *FileDownloaderHandler) GetHelp() handlers.HandlerHelp {
	return handlers.HandlerHelp{
		Name:        "file-downloader",
		Description: "Automatically downloads documents like PDFs, Office files, archives, and text files",
		Usage:       "Automatically processes all messages with supported file types",
		Examples: []string{
			"Send any PDF - it will be automatically downloaded",
			"Send any Excel/Word/PowerPoint file - it will be downloaded",
			"Send any ZIP/RAR archive - it will be downloaded",
			"Send any text file - it will be automatically downloaded",
		},
		Category: "utility",
	}
}
