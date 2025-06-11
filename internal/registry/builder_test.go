package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewBuilder(t *testing.T) {
	baseDir := "/tmp/test"
	baseURL := "https://example.com"
	
	builder := NewBuilder(baseDir, baseURL)
	
	if builder.baseDir != baseDir {
		t.Errorf("Expected baseDir %s, got %s", baseDir, builder.baseDir)
	}
	
	if builder.baseURL != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, builder.baseURL)
	}
	
	expectedPluginsDir := filepath.Join(baseDir, "plugins")
	if builder.pluginsDir != expectedPluginsDir {
		t.Errorf("Expected pluginsDir %s, got %s", expectedPluginsDir, builder.pluginsDir)
	}
}

func TestBuildIndexNoPluginsDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sup-builder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	builder := NewBuilder(tempDir, "https://example.com")
	
	_, err = builder.BuildIndex()
	if err == nil {
		t.Error("Expected error when plugins directory doesn't exist")
	}
}

func TestBuildIndexEmptyDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sup-builder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	pluginsDir := filepath.Join(tempDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("Failed to create plugins dir: %v", err)
	}
	
	builder := NewBuilder(tempDir, "https://example.com")
	
	index, err := builder.BuildIndex()
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	
	if len(index.Plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(index.Plugins))
	}
}

func TestBuildIndexWithPlugins(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sup-builder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create plugin structure
	pluginDir := filepath.Join(tempDir, "plugins", "test-plugin", "1.0.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("Failed to create plugin dir: %v", err)
	}
	
	// Create WASM file
	wasmPath := filepath.Join(pluginDir, "test-plugin.wasm")
	wasmData := []byte("fake wasm data for testing")
	if err := os.WriteFile(wasmPath, wasmData, 0644); err != nil {
		t.Fatalf("Failed to write WASM file: %v", err)
	}
	
	// Create metadata file
	metadata := PluginMetadata{
		Name:        "test-plugin",
		Description: "A test plugin",
		Author:      "Test Author",
		HomeURL:     "https://github.com/test/plugin",
		Category:    "test",
		Tags:        []string{"test", "example"},
	}
	
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}
	
	metadataPath := filepath.Join(pluginDir, "metadata.json")
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		t.Fatalf("Failed to write metadata file: %v", err)
	}
	
	builder := NewBuilder(tempDir, "https://example.com")
	
	index, err := builder.BuildIndex()
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	
	if len(index.Plugins) != 1 {
		t.Errorf("Expected 1 plugin, got %d", len(index.Plugins))
	}
	
	plugin, exists := index.Plugins["test-plugin"]
	if !exists {
		t.Fatal("Expected test-plugin to exist")
	}
	
	if plugin.Name != "test-plugin" {
		t.Errorf("Expected plugin name 'test-plugin', got %s", plugin.Name)
	}
	
	if plugin.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %s", plugin.Author)
	}
	
	if plugin.Latest != "1.0.0" {
		t.Errorf("Expected latest version '1.0.0', got %s", plugin.Latest)
	}
	
	if len(plugin.Versions) != 1 {
		t.Errorf("Expected 1 version, got %d", len(plugin.Versions))
	}
	
	version, exists := plugin.Versions["1.0.0"]
	if !exists {
		t.Fatal("Expected version 1.0.0 to exist")
	}
	
	if version.Size != int64(len(wasmData)) {
		t.Errorf("Expected size %d, got %d", len(wasmData), version.Size)
	}
	
	expectedURL := "https://example.com/plugins/test-plugin/1.0.0/test-plugin.wasm"
	if version.DownloadURL != expectedURL {
		t.Errorf("Expected download URL %s, got %s", expectedURL, version.DownloadURL)
	}
}

func TestBuildIndexMultipleVersions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sup-builder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create plugin with multiple versions
	pluginName := "multi-version-plugin"
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	
	for i, version := range versions {
		pluginDir := filepath.Join(tempDir, "plugins", pluginName, version)
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatalf("Failed to create plugin dir: %v", err)
		}
		
		wasmPath := filepath.Join(pluginDir, pluginName+".wasm")
		wasmData := []byte("fake wasm data version " + version)
		if err := os.WriteFile(wasmPath, wasmData, 0644); err != nil {
			t.Fatalf("Failed to write WASM file: %v", err)
		}
		
		// Sleep to ensure different modification times
		if i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	builder := NewBuilder(tempDir, "https://example.com")
	
	index, err := builder.BuildIndex()
	if err != nil {
		t.Fatalf("BuildIndex failed: %v", err)
	}
	
	plugin, exists := index.Plugins[pluginName]
	if !exists {
		t.Fatal("Expected plugin to exist")
	}
	
	if len(plugin.Versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(plugin.Versions))
	}
	
	for _, version := range versions {
		if _, exists := plugin.Versions[version]; !exists {
			t.Errorf("Expected version %s to exist", version)
		}
	}
	
	// Latest should be the most recently modified
	if plugin.Latest == "" {
		t.Error("Expected latest version to be set")
	}
}

func TestLoadMetadataNoFile(t *testing.T) {
	builder := NewBuilder("/tmp", "https://example.com")
	
	metadata, err := builder.loadMetadata("/nonexistent/path", "test-plugin")
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	
	if metadata.Name != "test-plugin" {
		t.Errorf("Expected name 'test-plugin', got %s", metadata.Name)
	}
	
	if metadata.Author != "Unknown" {
		t.Errorf("Expected author 'Unknown', got %s", metadata.Author)
	}
	
	if metadata.Category != "utility" {
		t.Errorf("Expected category 'utility', got %s", metadata.Category)
	}
}

func TestLoadMetadataWithFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sup-builder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	metadata := PluginMetadata{
		Name:        "custom-plugin",
		Description: "Custom description",
		Author:      "Custom Author",
		HomeURL:     "https://custom.url",
		Category:    "custom",
		Tags:        []string{"custom", "test"},
	}
	
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}
	
	metadataPath := filepath.Join(tempDir, "metadata.json")
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		t.Fatalf("Failed to write metadata file: %v", err)
	}
	
	builder := NewBuilder("/tmp", "https://example.com")
	
	loadedMetadata, err := builder.loadMetadata(metadataPath, "fallback-name")
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	
	if loadedMetadata.Name != "custom-plugin" {
		t.Errorf("Expected name 'custom-plugin', got %s", loadedMetadata.Name)
	}
	
	if loadedMetadata.Author != "Custom Author" {
		t.Errorf("Expected author 'Custom Author', got %s", loadedMetadata.Author)
	}
	
	if loadedMetadata.Category != "custom" {
		t.Errorf("Expected category 'custom', got %s", loadedMetadata.Category)
	}
	
	if len(loadedMetadata.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(loadedMetadata.Tags))
	}
}

func TestWriteIndex(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sup-builder-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	builder := NewBuilder("/tmp", "https://example.com")
	
	index := &Index{
		Version:   "1.0.0",
		UpdatedAt: time.Now(),
		Plugins: map[string]*Plugin{
			"test-plugin": {
				Name:        "test-plugin",
				Description: "Test plugin",
				Author:      "Test Author",
				Latest:      "1.0.0",
				Versions: map[string]*Version{
					"1.0.0": {
						Version:     "1.0.0",
						ReleaseDate: time.Now(),
						DownloadURL: "https://example.com/test.wasm",
						SHA256:      "abcd1234",
						Size:        1024,
					},
				},
			},
		},
	}
	
	outputDir := filepath.Join(tempDir, "output")
	
	err = builder.WriteIndex(index, outputDir)
	if err != nil {
		t.Fatalf("WriteIndex failed: %v", err)
	}
	
	// Check that all files were created
	files := []string{"index.json", "index.json.gz", "index.json.gz.sha256"}
	for _, file := range files {
		path := filepath.Join(outputDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", file)
		}
	}
	
	// Verify JSON content
	jsonPath := filepath.Join(outputDir, "index.json")
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read index.json: %v", err)
	}
	
	var loadedIndex Index
	if err := json.Unmarshal(jsonData, &loadedIndex); err != nil {
		t.Fatalf("Failed to parse index.json: %v", err)
	}
	
	if len(loadedIndex.Plugins) != 1 {
		t.Errorf("Expected 1 plugin in loaded index, got %d", len(loadedIndex.Plugins))
	}
	
	// Verify checksum file format
	checksumPath := filepath.Join(outputDir, "index.json.gz.sha256")
	checksumData, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatalf("Failed to read checksum file: %v", err)
	}
	
	checksumContent := string(checksumData)
	if !strings.Contains(checksumContent, "index.json.gz") {
		t.Error("Expected checksum file to contain filename")
	}
	
	if len(strings.Fields(checksumContent)) != 2 {
		t.Error("Expected checksum file to have hash and filename")
	}
}