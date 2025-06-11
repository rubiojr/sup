package handlers

import (
	"fmt"
)

type Registry interface {
	Register(name string, handler Handler) error
	Unregister(name string) error
	Get(name string) (Handler, error)
	GetAllHandlers() map[string]Handler
	GetHandlersForMessage(commandName string, isCommand bool) []Handler
	SetPluginManager(pm PluginManager) error
}

type registry struct {
	handlers      map[string]Handler
	pluginManager PluginManager
}

// Option is a function that configures the Bot
type RegistryOption func(*registry)

func WithPluginManager(m PluginManager) RegistryOption {
	return func(b *registry) {
		b.pluginManager = m
	}
}

func NewRegistry(opts ...RegistryOption) Registry {
	r := &registry{
		handlers: make(map[string]Handler),
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *registry) SetPluginManager(pm PluginManager) error {
	r.pluginManager = pm
	return nil
}

func (r *registry) Register(name string, handler Handler) error {
	if _, ok := r.handlers[name]; ok {
		return fmt.Errorf("handler with name %s already registered", name)
	}
	r.handlers[name] = handler
	return nil
}

func (r *registry) Unregister(name string) error {
	if _, ok := r.handlers[name]; !ok {
		return fmt.Errorf("handler with name %s not registered", name)
	}
	delete(r.handlers, name)
	return nil
}

func (r *registry) Get(name string) (Handler, error) {
	// Check built-in handlers first
	if handler, ok := r.handlers[name]; ok {
		return handler, nil
	}

	if r.pluginManager != nil {
		// Check WASM plugin handlers
		if plugin, ok := r.pluginManager.GetPlugin(name); ok {
			return plugin, nil
		}
	}

	return nil, fmt.Errorf("handler with name %s not registered", name)
}

func (r *registry) GetAllHandlers() map[string]Handler {
	result := make(map[string]Handler)
	for name, handler := range r.handlers {
		result[name] = handler
	}

	// Add WASM plugin handlers
	if r.pluginManager != nil {
		for name, plugin := range r.pluginManager.GetAllPlugins() {
			result[name] = plugin
		}
	}

	return result
}

// GetHandlersForMessage returns all handlers that should receive a given message
func (r *registry) GetHandlersForMessage(commandName string, isCommand bool) []Handler {
	var handlers []Handler

	// Check built-in handlers
	for _, handler := range r.handlers {
		if r.shouldReceiveMessage(handler.Topics(), commandName, isCommand) {
			handlers = append(handlers, handler)
		}
	}

	if r.pluginManager != nil {
		// Check WASM plugin handlers
		for _, plugin := range r.pluginManager.GetAllPlugins() {
			if r.shouldReceiveMessage(plugin.Topics(), commandName, isCommand) {
				handlers = append(handlers, plugin)
			}
		}
	}

	return handlers
}

// shouldReceiveMessage determines if a handler should receive a message based on its topics
func (r *registry) shouldReceiveMessage(topics []string, commandName string, isCommand bool) bool {
	if len(topics) == 0 {
		// Default behavior: only receive messages for the handler's own command
		return false
	}

	for _, topic := range topics {
		if topic == "*" {
			// Wildcard: receive all messages
			return true
		}
		if isCommand && topic == commandName {
			// Specific command topic
			return true
		}
	}
	return false
}
