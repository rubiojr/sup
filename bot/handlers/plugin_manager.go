package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/store"
)

type PluginManager interface {
	LoadPlugins() error
	ReloadPlugins() error
	GetAllPlugins() map[string]*WasmHandler
	GetPlugin(name string) (*WasmHandler, bool)
	UnloadAll() error
	UnloadPlugin(name string) error
}

type pluginManager struct {
	pluginDir string
	plugins   map[string]*WasmHandler
	cache     cache.Cache
	store     store.Store
}

func NewPluginManager(pluginDir string, c cache.Cache, s store.Store) PluginManager {
	return &pluginManager{
		pluginDir: pluginDir,
		plugins:   make(map[string]*WasmHandler),
		cache:     c,
		store:     s,
	}
}

func DefaultPluginManager(cache cache.Cache, store store.Store) PluginManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic("could not get default user home")
	}

	pluginDir := filepath.Join(homeDir, ".local", "share", "sup", "plugins")
	return NewPluginManager(pluginDir, cache, store)
}

func (pm *pluginManager) LoadPlugins() error {
	if err := pm.ensurePluginDir(); err != nil {
		return fmt.Errorf("failed to ensure plugin directory: %w", err)
	}

	entries, err := os.ReadDir(pm.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory %s: %w", pm.pluginDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		pluginPath := filepath.Join(pm.pluginDir, entry.Name())
		if err := pm.loadPlugin(pluginPath); err != nil {
			log.Warn("Warning: failed to load plugin", "name", entry.Name(), "error", err)
			continue
		}
	}

	return nil
}

func (pm *pluginManager) loadPlugin(pluginPath string) error {
	pluginName := filepath.Base(pluginPath)
	if ext := filepath.Ext(pluginName); ext != "" {
		pluginName = pluginName[:len(pluginName)-len(ext)]
	}
	handler, err := NewWasmHandler(pluginPath, pm.cache.Namespace(pluginName), pm.store.Namespace(pluginName))
	if err != nil {
		return fmt.Errorf("failed to create WASM handler: %w", err)
	}

	if n := handler.GetHelp().Name; n != "" {
		pluginName = n
	}

	if existing, exists := pm.plugins[pluginName]; exists {
		existing.Close()
	}

	pm.plugins[pluginName] = handler
	log.Debug("Loaded WASM plugin", "name", pluginName)
	return nil
}

func (pm *pluginManager) GetPlugin(name string) (*WasmHandler, bool) {
	plugin, exists := pm.plugins[name]
	return plugin, exists
}

func (pm *pluginManager) GetAllPlugins() map[string]*WasmHandler {
	result := make(map[string]*WasmHandler)
	for name, plugin := range pm.plugins {
		result[name] = plugin
	}
	return result
}

func (pm *pluginManager) UnloadPlugin(name string) error {
	if plugin, exists := pm.plugins[name]; exists {
		if err := plugin.Close(); err != nil {
			return fmt.Errorf("failed to close plugin %s: %w", name, err)
		}
		delete(pm.plugins, name)
		log.Debug("Unloaded WASM plugin", "name", name)
	}
	return nil
}

func (pm *pluginManager) UnloadAll() error {
	var errors []string
	for name := range pm.plugins {
		if err := pm.UnloadPlugin(name); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to unload some plugins: %s", strings.Join(errors, "; "))
	}

	return nil
}

func (pm *pluginManager) ensurePluginDir() error {
	if pm.pluginDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		pm.pluginDir = filepath.Join(homeDir, ".local", "share", "sup", "plugins")
	}

	if err := os.MkdirAll(pm.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory %s: %w", pm.pluginDir, err)
	}

	return nil
}

func (pm *pluginManager) ReloadPlugins() error {
	if err := pm.UnloadAll(); err != nil {
		return fmt.Errorf("failed to unload existing plugins: %w", err)
	}

	return pm.LoadPlugins()
}
