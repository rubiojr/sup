package main

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"

	"github.com/rubiojr/sup/pkg/plugin"
)

type RuletaPlugin struct{}

func (h *RuletaPlugin) Name() string {
	return "ruleta"
}

func (h *RuletaPlugin) Topics() []string {
	return []string{"ruleta"}
}

func (h *RuletaPlugin) HandleMessage(input plugin.Input) plugin.Output {
	message := strings.TrimSpace(input.Message)

	// Determine which directory to use
	searchDir := "."
	if message != "" {
		searchDir = message
	}

	// Get all image files from the directory (including subdirectories)
	imageFiles, err := h.findImageFiles(searchDir)
	if err != nil {
		return plugin.Error(fmt.Sprintf("Failed to scan directory '%s': %s", searchDir, err.Error()))
	}

	if len(imageFiles) == 0 {
		return plugin.Error(fmt.Sprintf("No image files found in directory '%s'.\n\nSupported formats: .jpg, .jpeg, .png, .gif, .webp", searchDir))
	}

	// Pick a random image
	randomIndex := rand.IntN(len(imageFiles))
	selectedImage := imageFiles[randomIndex]

	// Send the selected image
	err = plugin.SendImage(input.Sender, selectedImage)
	if err != nil {
		return plugin.Error(fmt.Sprintf("Failed to send image '%s': %s", selectedImage, err.Error()))
	}

	return plugin.Success("🎰 Ruleta rulz!")
}

func (h *RuletaPlugin) findImageFiles(rootDir string) ([]string, error) {
	var imageFiles []string
	validExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}

	// Helper function to check if a file is an image
	isImageFile := func(filename string) bool {
		ext := strings.ToLower(filepath.Ext(filename))
		for _, validExt := range validExts {
			if ext == validExt {
				return true
			}
		}
		return false
	}

	// Recursively scan the directory
	if err := h.scanDirectory(rootDir, "", &imageFiles, isImageFile); err != nil {
		return nil, err
	}

	return imageFiles, nil
}

func (h *RuletaPlugin) scanDirectory(baseDir, currentPath string, imageFiles *[]string, isImageFile func(string) bool) error {
	// Construct the full path to scan
	var scanPath string
	if currentPath == "" {
		scanPath = baseDir
	} else {
		scanPath = filepath.Join(baseDir, currentPath)
	}

	// List directory contents
	files, err := plugin.ListDirectory(scanPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		// Construct the relative path from baseDir
		var filePath string
		if currentPath == "" {
			filePath = file
		} else {
			filePath = filepath.Join(currentPath, file)
		}

		// Full path for checking if it's a directory
		fullPath := filepath.Join(baseDir, filePath)

		// Check if it's a directory by trying to list it
		if subFiles, err := plugin.ListDirectory(fullPath); err == nil {
			// It's a directory, scan it recursively
			if err := h.scanDirectory(baseDir, filePath, imageFiles, isImageFile); err != nil {
				// If we can't scan a subdirectory, continue with other files
				continue
			}
			_ = subFiles // Prevent unused variable warning
		} else {
			// It's a file, check if it's an image
			if isImageFile(file) {
				// Store the path relative to baseDir
				*imageFiles = append(*imageFiles, fullPath)
			}
		}
	}

	return nil
}

func (h *RuletaPlugin) GetHelp() plugin.HelpOutput {
	return plugin.NewHelpOutput(
		"ruleta",
		"Random image roulette - picks and sends a random image from the plugin directory",
		".sup ruleta [directory]",
		[]string{
			".sup ruleta",
			".sup ruleta images",
			".sup ruleta memes/funny",
		},
		"fun",
	)
}

func (h *RuletaPlugin) GetRequiredEnvVars() []string {
	return []string{}
}

func (h *RuletaPlugin) Version() string {
	return "0.1.0"
}

func init() {
	plugin.RegisterPlugin(&RuletaPlugin{})
}

func main() {}
