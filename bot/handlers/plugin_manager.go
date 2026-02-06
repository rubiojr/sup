package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/store"
)

type PluginManager interface {
	LoadPlugins() error
	ReloadPlugins() error
	WatchPlugins(ctx context.Context) error
	GetAllPlugins() map[string]*WasmHandler
	GetPlugin(name string) (*WasmHandler, bool)
	UnloadAll() error
	UnloadPlugin(name string) error
}

type pluginManager struct {
	pluginDir       string
	plugins         map[string]*WasmHandler
	cache           cache.Cache
	store           store.Store
	allowedCommands []string
}

func NewPluginManager(pluginDir string, c cache.Cache, s store.Store, allowedCommands []string) PluginManager {
	return &pluginManager{
		pluginDir:       pluginDir,
		plugins:         make(map[string]*WasmHandler),
		cache:           c,
		store:           s,
		allowedCommands: allowedCommands,
	}
}

func DefaultPluginManager(cache cache.Cache, store store.Store, allowedCommands []string) PluginManager {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic("could not get default user home")
	}

	pluginDir := filepath.Join(homeDir, ".local", "share", "sup", "plugins")
	return NewPluginManager(pluginDir, cache, store, allowedCommands)
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
	handler, err := NewWasmHandler(pluginPath, pm.cache.Namespace(pluginName), pm.store.Namespace(pluginName), pm.allowedCommands)
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

// WatchPlugins watches the plugin directory for .wasm file changes and
// reloads affected plugins automatically. It blocks until ctx is cancelled.
func (pm *pluginManager) WatchPlugins(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	if err := watcher.Add(pm.pluginDir); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to watch plugin directory %s: %w", pm.pluginDir, err)
	}

	log.Info("Watching plugin directory for changes", "dir", pm.pluginDir)

	// Debounce: editors often write files in multiple steps (write tmp + rename).
	// Wait a short period after the last event before reloading.
	const debounce = 500 * time.Millisecond
	var debounceTimer *time.Timer
	pending := make(map[string]bool)

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !strings.HasSuffix(event.Name, ".wasm") {
					continue
				}
				if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) == 0 {
					continue
				}

				pending[event.Name] = true
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounce, func() {
					for path := range pending {
						log.Info("Plugin file changed, reloading", "path", filepath.Base(path))
						if err := pm.loadPlugin(path); err != nil {
							log.Error("Failed to reload plugin", "path", filepath.Base(path), "error", err)
						} else {
							log.Info("Plugin reloaded successfully", "name", filepath.Base(path))
						}
					}
					pending = make(map[string]bool)
				})
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("Plugin watcher error", "error", err)
			}
		}
	}()

	return nil
}
