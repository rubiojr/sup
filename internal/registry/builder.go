package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rubiojr/sup/internal/log"
)

type PluginMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	HomeURL     string   `json:"home_url"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
}

type Builder struct {
	baseDir    string
	baseURL    string
	pluginsDir string
}

func NewBuilder(baseDir, baseURL string) *Builder {
	return &Builder{
		baseDir:    baseDir,
		baseURL:    baseURL,
		pluginsDir: filepath.Join(baseDir, "plugins"),
	}
}

func (b *Builder) BuildIndex() (*Index, error) {
	index := &Index{
		Version:   "1.0.0",
		UpdatedAt: time.Now(),
		Plugins:   make(map[string]*Plugin),
	}

	if _, err := os.Stat(b.pluginsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugins directory does not exist: %s", b.pluginsDir)
	}

	err := filepath.WalkDir(b.pluginsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".wasm") {
			return nil
		}

		return b.processWasmFile(path, index)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk plugins directory: %w", err)
	}

	b.setLatestVersions(index)

	log.Debug("Built index", "plugins", len(index.Plugins))
	return index, nil
}

func (b *Builder) processWasmFile(wasmPath string, index *Index) error {
	relPath, err := filepath.Rel(b.pluginsDir, wasmPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	parts := strings.Split(filepath.ToSlash(relPath), "/")
	if len(parts) < 3 {
		log.Warn("Skipping WASM file with invalid path structure", "path", relPath)
		return nil
	}

	pluginName := parts[0]
	version := parts[1]
	filename := parts[2]

	expectedFilename := pluginName + ".wasm"
	if filename != expectedFilename {
		log.Warn("WASM filename mismatch", "path", relPath, "expected", expectedFilename, "actual", filename)
	}

	stat, err := os.Stat(wasmPath)
	if err != nil {
		return fmt.Errorf("failed to stat WASM file %s: %w", wasmPath, err)
	}

	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("failed to read WASM file %s: %w", wasmPath, err)
	}

	hasher := sha256.New()
	hasher.Write(wasmData)
	checksum := hex.EncodeToString(hasher.Sum(nil))

	metadataPath := filepath.Join(b.pluginsDir, pluginName, "metadata.json")
	metadata, err := b.loadMetadata(metadataPath, pluginName)
	if err != nil {
		log.Warn("Failed to load metadata, using defaults", "path", metadataPath, "error", err)
	}

	if index.Plugins[pluginName] == nil {
		index.Plugins[pluginName] = &Plugin{
			Name:        metadata.Name,
			Description: metadata.Description,
			Author:      metadata.Author,
			HomeURL:     metadata.HomeURL,
			Category:    metadata.Category,
			Tags:        metadata.Tags,
			Versions:    make(map[string]*Version),
		}
	}

	versionInfo := &Version{
		Version:     version,
		ReleaseDate: stat.ModTime(),
		SHA256:      checksum,
		Size:        stat.Size(),
	}

	index.Plugins[pluginName].Versions[version] = versionInfo

	log.Debug("Processed plugin version", "name", pluginName, "version", version, "size", stat.Size())
	return nil
}

func (b *Builder) loadMetadata(metadataPath, pluginName string) (*PluginMetadata, error) {
	metadata := &PluginMetadata{
		Name:        pluginName,
		Description: fmt.Sprintf("WASM plugin: %s", pluginName),
		Author:      "Unknown",
		HomeURL:     "",
		Category:    "utility",
		Tags:        []string{},
	}

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return metadata, nil
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return metadata, fmt.Errorf("failed to read metadata file: %w", err)
	}

	if err := json.Unmarshal(data, metadata); err != nil {
		return metadata, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	if metadata.Name == "" {
		metadata.Name = pluginName
	}

	return metadata, nil
}

func (b *Builder) setLatestVersions(index *Index) {
	for pluginName, plugin := range index.Plugins {
		if len(plugin.Versions) == 0 {
			continue
		}

		var latestVersion string
		var latestTime time.Time

		for version, versionInfo := range plugin.Versions {
			if latestVersion == "" || versionInfo.ReleaseDate.After(latestTime) {
				latestVersion = version
				latestTime = versionInfo.ReleaseDate
			}
		}

		plugin.Latest = latestVersion
		log.Debug("Set latest version", "plugin", pluginName, "version", latestVersion)
	}
}

func (b *Builder) WriteIndex(index *Index, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	jsonData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index to JSON: %w", err)
	}

	indexPath := filepath.Join(outputDir, "index.json")
	if err := os.WriteFile(indexPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write index.json: %w", err)
	}

	compressedData, err := b.compressJSON(jsonData)
	if err != nil {
		return fmt.Errorf("failed to compress index: %w", err)
	}

	compressedPath := filepath.Join(outputDir, "index.json.gz")
	if err := os.WriteFile(compressedPath, compressedData, 0644); err != nil {
		return fmt.Errorf("failed to write index.json.gz: %w", err)
	}

	hasher := sha256.New()
	hasher.Write(compressedData)
	checksum := hex.EncodeToString(hasher.Sum(nil))

	checksumPath := filepath.Join(outputDir, "index.json.gz.sha256")
	checksumContent := fmt.Sprintf("%s  index.json.gz\n", checksum)
	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0644); err != nil {
		return fmt.Errorf("failed to write checksum file: %w", err)
	}

	log.Debug("Wrote index files", "dir", outputDir, "plugins", len(index.Plugins))
	fmt.Printf("Successfully generated registry index:\n")
	fmt.Printf("  - %s (%d bytes)\n", indexPath, len(jsonData))
	fmt.Printf("  - %s (%d bytes)\n", compressedPath, len(compressedData))
	fmt.Printf("  - %s\n", checksumPath)
	fmt.Printf("  - %d plugins indexed\n", len(index.Plugins))

	return nil
}

func (b *Builder) compressJSON(data []byte) ([]byte, error) {
	return compressData(data)
}
