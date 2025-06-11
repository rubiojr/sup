package registry

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rubiojr/sup/internal/log"
)

const (
	DefaultRegistryURL = "https://sup-registry.rbel.co"
	IndexFile          = "index.json.gz"
	IndexChecksumFile  = "index.json.gz.sha256"
	UserAgent          = "sup-cli/1.0"
)

type Client struct {
	registryURL string
	httpClient  *http.Client
	cacheDir    string
}

func NewClient(registryURL string) *Client {
	if registryURL == "" {
		registryURL = DefaultRegistryURL
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic("could not get user home directory")
	}

	cacheDir := filepath.Join(homeDir, ".local", "share", "sup", "cache")

	return &Client{
		registryURL: registryURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir: cacheDir,
	}
}

func (c *Client) FetchIndex() (*Index, error) {
	if err := os.MkdirAll(c.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	indexURL := fmt.Sprintf("%s/%s", c.registryURL, IndexFile)
	checksumURL := fmt.Sprintf("%s/%s", c.registryURL, IndexChecksumFile)

	log.Debug("Fetching index from registry", "url", indexURL)

	expectedChecksum, err := c.fetchChecksum(checksumURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index checksum: %w", err)
	}

	indexData, err := c.fetchFile(indexURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index: %w", err)
	}

	if err := c.verifyChecksum(indexData, expectedChecksum); err != nil {
		return nil, fmt.Errorf("index checksum verification failed: %w", err)
	}

	gzipReader, err := gzip.NewReader(strings.NewReader(string(indexData)))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	jsonData, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress index: %w", err)
	}

	var index Index
	if err := json.Unmarshal(jsonData, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index JSON: %w", err)
	}

	log.Debug("Successfully fetched index", "plugins", len(index.Plugins), "version", index.Version)
	return &index, nil
}

func (c *Client) DownloadPlugin(pluginName, version string, targetDir string) error {
	index, err := c.FetchIndex()
	if err != nil {
		return fmt.Errorf("failed to fetch index: %w", err)
	}

	plugin, exists := index.Plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin '%s' not found in registry", pluginName)
	}

	if version == "" || version == "latest" {
		version = plugin.Latest
	}

	versionInfo, exists := plugin.Versions[version]
	if !exists {
		return fmt.Errorf("version '%s' not found for plugin '%s'", version, pluginName)
	}

	log.Debug("Downloading plugin", "name", pluginName, "version", version)

	downloadURL := c.constructDownloadURL(pluginName, version)
	log.Debug("Fetching plugin from URL", "url", downloadURL)

	pluginData, err := c.fetchFile(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	if err := c.verifyChecksum(pluginData, versionInfo.SHA256); err != nil {
		return fmt.Errorf("plugin checksum verification failed: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	pluginPath := filepath.Join(targetDir, fmt.Sprintf("%s.wasm", pluginName))
	if err := os.WriteFile(pluginPath, pluginData, 0644); err != nil {
		return fmt.Errorf("failed to write plugin file: %w", err)
	}

	log.Debug("Successfully downloaded plugin", "name", pluginName, "version", version, "path", pluginPath)
	return nil
}

func compressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)

	if _, err := gzipWriter.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to gzip writer: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func (c *Client) constructDownloadURL(pluginName, version string) string {
	return fmt.Sprintf("%s/plugins/%s/%s/%s.wasm", c.registryURL, pluginName, version, pluginName)
}

func (c *Client) ListPlugins() ([]PluginInfo, error) {
	index, err := c.FetchIndex()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	pluginDir := filepath.Join(homeDir, ".local", "share", "sup", "plugins")

	var plugins []PluginInfo
	for name, plugin := range index.Plugins {
		pluginPath := filepath.Join(pluginDir, fmt.Sprintf("%s.wasm", name))
		_, err := os.Stat(pluginPath)
		installed := err == nil

		latestVersion := plugin.Versions[plugin.Latest]
		if latestVersion == nil {
			continue
		}

		info := PluginInfo{
			Name:        name,
			Version:     plugin.Latest,
			Author:      plugin.Author,
			Description: plugin.Description,
			HomeURL:     plugin.HomeURL,
			Category:    plugin.Category,
			Tags:        plugin.Tags,
			Installed:   installed,
			Available:   true,
		}

		plugins = append(plugins, info)
	}

	return plugins, nil
}

func (c *Client) fetchFile(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error %d when fetching %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func (c *Client) fetchChecksum(url string) (string, error) {
	data, err := c.fetchFile(url)
	if err != nil {
		return "", err
	}

	checksum := strings.TrimSpace(string(data))
	parts := strings.Fields(checksum)
	if len(parts) > 0 {
		return parts[0], nil
	}

	return checksum, nil
}

func (c *Client) verifyChecksum(data []byte, expectedChecksum string) error {
	hasher := sha256.New()
	hasher.Write(data)
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}
