package handlers

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestGetUniqueFilepath(t *testing.T) {
	handler := &ImageDownloaderHandler{
		downloadDir: t.TempDir(),
	}

	tests := []struct {
		name           string
		filename       string
		content1       []byte
		content2       []byte
		expectedSuffix bool
		description    string
	}{
		{
			name:           "new_file",
			filename:       "test.jpg",
			content1:       []byte("image content"),
			content2:       nil,
			expectedSuffix: false,
			description:    "Should return original path when file doesn't exist",
		},
		{
			name:           "same_content",
			filename:       "test.jpg",
			content1:       []byte("same image content"),
			content2:       []byte("same image content"),
			expectedSuffix: false,
			description:    "Should return empty string when content is identical (ignored)",
		},
		{
			name:           "different_content",
			filename:       "test.jpg",
			content1:       []byte("first image content"),
			content2:       []byte("different image content"),
			expectedSuffix: true,
			description:    "Should return path with suffix when content differs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalPath := filepath.Join(handler.downloadDir, tt.filename)

			if tt.content2 == nil {
				result := handler.getUniqueFilepath(handler.downloadDir, tt.filename, tt.content1)
				if result != originalPath {
					t.Errorf("Expected %s, got %s", originalPath, result)
				}
				return
			}

			err := os.WriteFile(originalPath, tt.content1, 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result := handler.getUniqueFilepath(handler.downloadDir, tt.filename, tt.content2)

			if tt.expectedSuffix {
				if result == originalPath {
					t.Errorf("Expected path with suffix, got original path: %s", result)
				}
				expectedPattern := filepath.Join(handler.downloadDir, "test_1.jpg")
				if result != expectedPattern {
					t.Errorf("Expected %s, got %s", expectedPattern, result)
				}
			} else {
				// For same content, we now expect empty string (ignored)
				if tt.name == "same_content" {
					if result != "" {
						t.Errorf("Expected empty string for duplicate content, got %s", result)
					}
				} else {
					if result != originalPath {
						t.Errorf("Expected original path %s, got %s", originalPath, result)
					}
				}
			}
		})
	}
}

func TestGetUniqueFilepathMultipleDuplicates(t *testing.T) {
	handler := &ImageDownloaderHandler{
		downloadDir: t.TempDir(),
	}

	filename := "test.jpg"
	content1 := []byte("first content")
	content2 := []byte("second content")
	content3 := []byte("third content")

	originalPath := filepath.Join(handler.downloadDir, filename)
	err := os.WriteFile(originalPath, content1, 0644)
	if err != nil {
		t.Fatalf("Failed to create first test file: %v", err)
	}

	path1 := handler.getUniqueFilepath(handler.downloadDir, filename, content2)
	expectedPath1 := filepath.Join(handler.downloadDir, "test_1.jpg")
	if path1 != expectedPath1 {
		t.Errorf("First duplicate: expected %s, got %s", expectedPath1, path1)
	}

	err = os.WriteFile(path1, content2, 0644)
	if err != nil {
		t.Fatalf("Failed to create second test file: %v", err)
	}

	path2 := handler.getUniqueFilepath(handler.downloadDir, filename, content3)
	expectedPath2 := filepath.Join(handler.downloadDir, "test_2.jpg")
	if path2 != expectedPath2 {
		t.Errorf("Second duplicate: expected %s, got %s", expectedPath2, path2)
	}

	pathSame := handler.getUniqueFilepath(handler.downloadDir, filename, content2)
	if pathSame != "" {
		t.Errorf("Same content should return empty string (ignored): expected empty, got %s", pathSame)
	}
}

func TestGetUniqueFilepathWithDifferentExtensions(t *testing.T) {
	handler := &ImageDownloaderHandler{
		downloadDir: t.TempDir(),
	}

	testCases := []struct {
		filename string
		expected string
	}{
		{"image.png", "image_1.png"},
		{"document.pdf", "document_1.pdf"},
		{"file", "file_1"},
		{"name.with.dots.jpg", "name.with.dots_1.jpg"},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			originalPath := filepath.Join(handler.downloadDir, tc.filename)
			err := os.WriteFile(originalPath, []byte("original content"), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result := handler.getUniqueFilepath(handler.downloadDir, tc.filename, []byte("different content"))
			expected := filepath.Join(handler.downloadDir, tc.expected)
			if result != expected {
				t.Errorf("Expected %s, got %s", expected, result)
			}
		})
	}
}

func TestContentHashing(t *testing.T) {
	content1 := []byte("test content 1")
	content2 := []byte("test content 2")
	content3 := []byte("test content 1")

	hash1 := sha256.Sum256(content1)
	hash2 := sha256.Sum256(content2)
	hash3 := sha256.Sum256(content3)

	if hash1 == hash2 {
		t.Error("Different content should produce different hashes")
	}

	if hash1 != hash3 {
		t.Error("Same content should produce same hashes")
	}
}
