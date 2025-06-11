package registry

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("")
	if client.registryURL != DefaultRegistryURL {
		t.Errorf("Expected default registry URL %s, got %s", DefaultRegistryURL, client.registryURL)
	}

	customURL := "https://custom-registry.example.com"
	client = NewClient(customURL)
	if client.registryURL != customURL {
		t.Errorf("Expected custom registry URL %s, got %s", customURL, client.registryURL)
	}
}

func TestFetchIndex(t *testing.T) {
	index := &Index{
		Version:   "1.0.0",
		UpdatedAt: time.Now(),
		Plugins: map[string]*Plugin{
			"test-plugin": {
				Name:        "test-plugin",
				Description: "A test plugin",
				Author:      "Test Author",
				HomeURL:     "https://github.com/test/plugin",
				Category:    "test",
				Tags:        []string{"test", "example"},
				Latest:      "1.0.0",
				Versions: map[string]*Version{
					"1.0.0": {
						Version:     "1.0.0",
						ReleaseDate: time.Now(),
						DownloadURL: "https://example.com/test-plugin.wasm",
						SHA256:      "abcd1234",
						Size:        1024,
					},
				},
			},
		},
	}

	indexJSON, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Failed to marshal test index: %v", err)
	}

	var compressedIndex strings.Builder
	gzipWriter := gzip.NewWriter(&compressedIndex)
	if _, err := gzipWriter.Write(indexJSON); err != nil {
		t.Fatalf("Failed to compress index: %v", err)
	}
	gzipWriter.Close()

	compressedData := compressedIndex.String()
	hasher := sha256.New()
	hasher.Write([]byte(compressedData))
	checksum := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json.gz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte(compressedData))
		case "/index.json.gz.sha256":
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "%s  index.json.gz", checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	
	tempDir, err := os.MkdirTemp("", "sup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	client.cacheDir = tempDir

	fetchedIndex, err := client.FetchIndex()
	if err != nil {
		t.Fatalf("FetchIndex failed: %v", err)
	}

	if fetchedIndex.Version != index.Version {
		t.Errorf("Expected version %s, got %s", index.Version, fetchedIndex.Version)
	}

	if len(fetchedIndex.Plugins) != len(index.Plugins) {
		t.Errorf("Expected %d plugins, got %d", len(index.Plugins), len(fetchedIndex.Plugins))
	}

	testPlugin, exists := fetchedIndex.Plugins["test-plugin"]
	if !exists {
		t.Error("Expected test-plugin to exist in fetched index")
	} else {
		if testPlugin.Name != "test-plugin" {
			t.Errorf("Expected plugin name 'test-plugin', got %s", testPlugin.Name)
		}
		if testPlugin.Author != "Test Author" {
			t.Errorf("Expected author 'Test Author', got %s", testPlugin.Author)
		}
	}
}

func TestDownloadPlugin(t *testing.T) {
	pluginData := []byte("fake wasm plugin data")
	hasher := sha256.New()
	hasher.Write(pluginData)
	pluginChecksum := hex.EncodeToString(hasher.Sum(nil))

	index := &Index{
		Version: "1.0.0",
		Plugins: map[string]*Plugin{
			"test-plugin": {
				Name:   "test-plugin",
				Latest: "1.0.0",
				Versions: map[string]*Version{
					"1.0.0": {
						Version:     "1.0.0",
						DownloadURL: "REPLACE_WITH_SERVER_URL/test-plugin.wasm",
						SHA256:      pluginChecksum,
						Size:        int64(len(pluginData)),
					},
				},
			},
		},
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json.gz":
			index.Plugins["test-plugin"].Versions["1.0.0"].DownloadURL = server.URL + "/test-plugin.wasm"
			
			indexJSON, _ := json.Marshal(index)
			var compressedIndex strings.Builder
			gzipWriter := gzip.NewWriter(&compressedIndex)
			gzipWriter.Write(indexJSON)
			gzipWriter.Close()
			
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte(compressedIndex.String()))
		case "/index.json.gz.sha256":
			index.Plugins["test-plugin"].Versions["1.0.0"].DownloadURL = server.URL + "/test-plugin.wasm"
			
			indexJSON, _ := json.Marshal(index)
			var compressedIndex strings.Builder
			gzipWriter := gzip.NewWriter(&compressedIndex)
			gzipWriter.Write(indexJSON)
			gzipWriter.Close()
			
			hasher := sha256.New()
			hasher.Write([]byte(compressedIndex.String()))
			checksum := hex.EncodeToString(hasher.Sum(nil))
			fmt.Fprintf(w, "%s  index.json.gz", checksum)
		case "/test-plugin.wasm":
			w.Header().Set("Content-Type", "application/wasm")
			w.Write(pluginData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	
	tempDir, err := os.MkdirTemp("", "sup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	client.cacheDir = tempDir

	targetDir := filepath.Join(tempDir, "plugins")
	
	err = client.DownloadPlugin("test-plugin", "", targetDir)
	if err != nil {
		t.Fatalf("DownloadPlugin failed: %v", err)
	}

	pluginPath := filepath.Join(targetDir, "test-plugin.wasm")
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Error("Plugin file was not created")
	}

	downloadedData, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded plugin: %v", err)
	}

	if string(downloadedData) != string(pluginData) {
		t.Error("Downloaded plugin data doesn't match expected data")
	}
}

func TestDownloadPluginNotFound(t *testing.T) {
	index := &Index{
		Version: "1.0.0",
		Plugins: map[string]*Plugin{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json.gz":
			indexJSON, _ := json.Marshal(index)
			var compressedIndex strings.Builder
			gzipWriter := gzip.NewWriter(&compressedIndex)
			gzipWriter.Write(indexJSON)
			gzipWriter.Close()
			
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte(compressedIndex.String()))
		case "/index.json.gz.sha256":
			indexJSON, _ := json.Marshal(index)
			var compressedIndex strings.Builder
			gzipWriter := gzip.NewWriter(&compressedIndex)
			gzipWriter.Write(indexJSON)
			gzipWriter.Close()
			
			hasher := sha256.New()
			hasher.Write([]byte(compressedIndex.String()))
			checksum := hex.EncodeToString(hasher.Sum(nil))
			fmt.Fprintf(w, "%s  index.json.gz", checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	
	tempDir, err := os.MkdirTemp("", "sup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	client.cacheDir = tempDir

	err = client.DownloadPlugin("nonexistent-plugin", "", tempDir)
	if err == nil {
		t.Error("Expected error when downloading nonexistent plugin")
	}

	expectedError := "plugin 'nonexistent-plugin' not found in registry"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got: %v", expectedError, err)
	}
}

func TestVerifyChecksum(t *testing.T) {
	client := NewClient("")
	data := []byte("test data")
	
	hasher := sha256.New()
	hasher.Write(data)
	correctChecksum := hex.EncodeToString(hasher.Sum(nil))
	
	err := client.verifyChecksum(data, correctChecksum)
	if err != nil {
		t.Errorf("Expected checksum verification to pass, got error: %v", err)
	}
	
	err = client.verifyChecksum(data, "wrong_checksum")
	if err == nil {
		t.Error("Expected checksum verification to fail with wrong checksum")
	}
}

func TestListPlugins(t *testing.T) {
	index := &Index{
		Version: "1.0.0",
		Plugins: map[string]*Plugin{
			"plugin1": {
				Name:        "plugin1",
				Description: "First test plugin",
				Author:      "Author 1",
				Latest:      "1.0.0",
				Versions: map[string]*Version{
					"1.0.0": {
						Version: "1.0.0",
					},
				},
			},
			"plugin2": {
				Name:        "plugin2",
				Description: "Second test plugin",
				Author:      "Author 2",
				Latest:      "2.0.0",
				Versions: map[string]*Version{
					"2.0.0": {
						Version: "2.0.0",
					},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json.gz":
			indexJSON, _ := json.Marshal(index)
			var compressedIndex strings.Builder
			gzipWriter := gzip.NewWriter(&compressedIndex)
			gzipWriter.Write(indexJSON)
			gzipWriter.Close()
			
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte(compressedIndex.String()))
		case "/index.json.gz.sha256":
			indexJSON, _ := json.Marshal(index)
			var compressedIndex strings.Builder
			gzipWriter := gzip.NewWriter(&compressedIndex)
			gzipWriter.Write(indexJSON)
			gzipWriter.Close()
			
			hasher := sha256.New()
			hasher.Write([]byte(compressedIndex.String()))
			checksum := hex.EncodeToString(hasher.Sum(nil))
			fmt.Fprintf(w, "%s  index.json.gz", checksum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	
	tempDir, err := os.MkdirTemp("", "sup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	client.cacheDir = tempDir

	plugins, err := client.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}

	if len(plugins) != 2 {
		t.Errorf("Expected 2 plugins, got %d", len(plugins))
	}

	pluginNames := make(map[string]bool)
	for _, plugin := range plugins {
		pluginNames[plugin.Name] = true
		if !plugin.Available {
			t.Errorf("Expected plugin %s to be available", plugin.Name)
		}
		if plugin.Installed {
			t.Errorf("Expected plugin %s to not be installed", plugin.Name)
		}
	}

	if !pluginNames["plugin1"] || !pluginNames["plugin2"] {
		t.Error("Expected both plugin1 and plugin2 to be in the list")
	}
}