package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	extism "github.com/extism/go-sdk"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/store"
	"github.com/tetratelabs/wazero"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// CacheResponse represents the response from cache operations
type CacheResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type WasmHandler struct {
	plugin  *extism.Plugin
	name    string
	help    HandlerHelp
	dataDir string
}

type WasmInput struct {
	Message string          `json:"message"`
	Sender  string          `json:"sender"`
	Info    WasmMessageInfo `json:"info"`
}

type WasmMessageInfo struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	PushName  string `json:"push_name"`
	IsGroup   bool   `json:"is_group"`
}

type WasmOutput struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Reply   string `json:"reply,omitempty"`
}

type WasmHelpOutput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Usage       string   `json:"usage"`
	Examples    []string `json:"examples"`
	Category    string   `json:"category"`
}

type SendImageRequest struct {
	Recipient string `json:"recipient"`
	ImagePath string `json:"image_path"`
}

type ListDirResponse struct {
	Success bool     `json:"success"`
	Files   []string `json:"files,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func NewWasmHandler(wasmPath string, cache cache.Cache, store store.Store) (*WasmHandler, error) {
	ctx := context.Background()

	// Calculate the plugin data directory
	pluginDir := filepath.Dir(wasmPath)
	pluginName := filepath.Base(wasmPath)
	if ext := filepath.Ext(pluginName); ext != "" {
		pluginName = pluginName[:len(pluginName)-len(ext)]
	}
	dataDir := filepath.Join(pluginDir, "../plugin-data", pluginName)

	// First create a temporary plugin to query required environment variables
	tempManifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmPath,
			},
		},
		AllowedHosts: []string{"*"},
		AllowedPaths: map[string]string{},
	}

	tempConfig := extism.PluginConfig{
		EnableWasi: true,
	}

	// Create minimal host functions for temporary plugin
	tempHostFunctions := []extism.HostFunction{
		extism.NewHostFunctionWithStack(
			"read_file",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the path string from memory
				pathOffset := extism.DecodeU32(stack[0])
				requestedPath, err := p.ReadString(uint64(pathOffset))
				if err != nil {
					offset, _ := p.WriteString("")
					stack[0] = offset
					return
				}

				// Sanitize and resolve the path relative to plugin data directory
				safePath := sanitizePluginPath(requestedPath, dataDir)
				if safePath == "" {
					log.Warn("Plugin file read blocked - path outside allowed directory", "requested_path", requestedPath, "data_dir", dataDir)
					// Path is outside allowed directory - return empty string
					offset, _ := p.WriteString("")
					stack[0] = offset
					return
				}

				log.Debug("Plugin reading file", "path", safePath, "requested_path", requestedPath)

				// For temp plugin, just return empty string (don't actually read files)
				offset, _ := p.WriteString("")
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"send_image",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the request data from memory
				dataOffset := extism.DecodeU32(stack[0])
				requestData, err := p.ReadBytes(uint64(dataOffset))
				if err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				var req SendImageRequest
				if err := json.Unmarshal(requestData, &req); err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				// Sanitize and resolve the image path relative to plugin data directory
				safePath := sanitizePluginPath(req.ImagePath, dataDir)
				if safePath == "" {
					log.Warn("Plugin image send blocked - path outside allowed directory", "requested_path", req.ImagePath, "data_dir", dataDir, "recipient", req.Recipient)
					// Path is outside allowed directory
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				log.Info("Plugin sending image", "path", safePath, "requested_path", req.ImagePath, "recipient", req.Recipient)

				// For temp plugin, just return success without actually sending
				stack[0] = extism.EncodeU32(0) // success
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
		extism.NewHostFunctionWithStack(
			"list_directory",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the path string from memory
				pathOffset := extism.DecodeU32(stack[0])
				requestedPath, err := p.ReadString(uint64(pathOffset))
				if err != nil {
					resp := ListDirResponse{Success: false, Error: "Failed to read path"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				// Sanitize and resolve the path relative to plugin data directory
				safePath := sanitizePluginPath(requestedPath, dataDir)
				if safePath == "" {
					log.Warn("Plugin directory listing blocked - path outside allowed directory", "requested_path", requestedPath, "data_dir", dataDir)
					resp := ListDirResponse{Success: false, Error: "Path outside allowed directory"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				log.Debug("Plugin listing directory", "path", safePath, "requested_path", requestedPath)

				// For temp plugin, return empty list
				resp := ListDirResponse{Success: true, Files: []string{}}
				respData, _ := json.Marshal(resp)
				offset, _ := p.WriteString(string(respData))
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"get_cache",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// For temp plugin, return empty cache response
				resp := CacheResponse{Success: false, Error: "Cache not available in temp plugin"}
				respData, _ := json.Marshal(resp)
				offset, _ := p.WriteString(string(respData))
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"set_cache",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// For temp plugin, just return success without actually setting
				stack[0] = extism.EncodeU32(0) // success
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
		extism.NewHostFunctionWithStack(
			"get_store",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// For temp plugin, return empty store response
				resp := CacheResponse{Success: false, Error: "Store not available in temp plugin"}
				respData, _ := json.Marshal(resp)
				offset, _ := p.WriteString(string(respData))
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"set_store",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// For temp plugin, just return success without actually setting
				stack[0] = extism.EncodeU32(0) // success
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
	}

	tempPlugin, err := extism.NewPlugin(ctx, tempManifest, tempConfig, tempHostFunctions)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary WASM plugin from %s: %w", wasmPath, err)
	}

	// Query the plugin for required environment variables
	requiredEnvVars, err := getRequiredEnvVars(tempPlugin)
	if err != nil {
		tempPlugin.Close(ctx)
		return nil, fmt.Errorf("failed to get required env vars from %s: %w", wasmPath, err)
	}

	// Close the temporary plugin
	tempPlugin.Close(ctx)

	// Create the final module config with only the required environment variables
	moduleConfig := wazero.NewModuleConfig()
	for _, name := range requiredEnvVars {
		if value := os.Getenv(name); value != "" {
			moduleConfig = moduleConfig.WithEnv(name, value)
		}
	}

	// Create the final plugin with the proper environment variables
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmPath,
			},
		},
		AllowedHosts: []string{"*"},
		AllowedPaths: map[string]string{},
	}

	config := extism.PluginConfig{
		EnableWasi:   true,
		ModuleConfig: moduleConfig,
	}

	hostFunctions := []extism.HostFunction{
		extism.NewHostFunctionWithStack(
			"read_file",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the path string from memory
				pathOffset := extism.DecodeU32(stack[0])
				requestedPath, err := p.ReadString(uint64(pathOffset))
				if err != nil {
					offset, _ := p.WriteString("")
					stack[0] = offset
					return
				}

				// Sanitize and resolve the path relative to plugin data directory
				safePath := sanitizePluginPath(requestedPath, dataDir)
				if safePath == "" {
					log.Warn("Plugin file read blocked - path outside allowed directory", "requested_path", requestedPath, "data_dir", dataDir)
					// Path is outside allowed directory
					offset, _ := p.WriteString("")
					stack[0] = offset
					return
				}

				log.Debug("Plugin reading file", "path", safePath, "requested_path", requestedPath)

				// Read the file
				data, err := os.ReadFile(safePath)
				if err != nil {
					// Return empty string on error
					offset, _ := p.WriteString("")
					stack[0] = offset
					return
				}

				// Write the file contents to memory and return offset
				offset, err := p.WriteString(string(data))
				if err != nil {
					offset, _ := p.WriteString("")
					stack[0] = offset
					return
				}
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"send_image",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the request data from memory
				dataOffset := extism.DecodeU32(stack[0])
				requestData, err := p.ReadBytes(uint64(dataOffset))
				if err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				var req SendImageRequest
				if err := json.Unmarshal(requestData, &req); err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				// Sanitize and resolve the image path relative to plugin data directory
				safePath := sanitizePluginPath(req.ImagePath, dataDir)
				if safePath == "" {
					log.Warn("Plugin image send blocked - path outside allowed directory", "requested_path", req.ImagePath, "data_dir", dataDir, "recipient", req.Recipient)
					// Path is outside allowed directory
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				log.Info("Plugin sending image", "path", safePath, "requested_path", req.ImagePath, "recipient", req.Recipient)

				// Get the client and send the image
				c, err := client.GetClient()
				if err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				// Parse the recipient JID
				recipientJID, err := types.ParseJID(req.Recipient)
				if err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				// Send the image using the sanitized path
				err = c.SendImage(recipientJID, safePath)
				if err != nil {
					log.Error("Plugin image send failed", "path", safePath, "recipient", req.Recipient, "error", err)
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				stack[0] = extism.EncodeU32(0) // success
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
		extism.NewHostFunctionWithStack(
			"list_directory",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the path string from memory
				pathOffset := extism.DecodeU32(stack[0])
				requestedPath, err := p.ReadString(uint64(pathOffset))
				if err != nil {
					resp := ListDirResponse{Success: false, Error: "Failed to read path"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				// Sanitize and resolve the path relative to plugin data directory
				safePath := sanitizePluginPath(requestedPath, dataDir)
				if safePath == "" {
					log.Warn("Plugin directory listing blocked - path outside allowed directory", "requested_path", requestedPath, "data_dir", dataDir)
					resp := ListDirResponse{Success: false, Error: "Path outside allowed directory"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				log.Debug("Plugin listing directory", "path", safePath, "requested_path", requestedPath)

				// Check if the path exists and is a directory
				fileInfo, err := os.Stat(safePath)
				if err != nil {
					log.Debug("Plugin directory listing failed - path not found", "path", safePath, "error", err)
					resp := ListDirResponse{Success: false, Error: fmt.Sprintf("Directory not found: %s", err.Error())}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				if !fileInfo.IsDir() {
					resp := ListDirResponse{Success: false, Error: "Path is not a directory"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				// Read directory contents
				entries, err := os.ReadDir(safePath)
				if err != nil {
					log.Debug("Plugin directory read failed", "path", safePath, "error", err)
					resp := ListDirResponse{Success: false, Error: fmt.Sprintf("Failed to read directory: %s", err.Error())}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				// Build list of file names
				var files []string
				for _, entry := range entries {
					files = append(files, entry.Name())
				}

				// Return successful response with file list
				resp := ListDirResponse{Success: true, Files: files}
				respData, _ := json.Marshal(resp)
				offset, _ := p.WriteString(string(respData))
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"get_cache",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the key string from memory
				keyOffset := extism.DecodeU32(stack[0])
				key, err := p.ReadString(uint64(keyOffset))
				if err != nil {
					resp := CacheResponse{Success: false, Error: "Failed to read key"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				log.Debug("Plugin getting cache value", "key", key)

				// Get value from cache
				var value []byte
				if cache != nil {
					value, err = cache.Get([]byte(key))
					if err != nil {
						log.Debug("Plugin cache get failed", "key", key, "error", err)
						resp := CacheResponse{Success: false, Error: err.Error()}
						respData, _ := json.Marshal(resp)
						offset, _ := p.WriteString(string(respData))
						stack[0] = offset
						return
					}
					log.Debug("Plugin cache get success", "key", key, "value", string(value), "raw_bytes", value)
				} else {
					resp := CacheResponse{Success: false, Error: "Cache not available"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				// Return successful response with data
				resp := CacheResponse{Success: true, Data: string(value)}
				respData, _ := json.Marshal(resp)
				offset, _ := p.WriteString(string(respData))
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"set_cache",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the request data from memory
				dataOffset := extism.DecodeU32(stack[0])
				requestData, err := p.ReadBytes(uint64(dataOffset))
				if err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				var req map[string]interface{}
				if err := json.Unmarshal(requestData, &req); err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				key, ok := req["key"].(string)
				if !ok {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				// Handle value as string
				var value []byte
				if v, ok := req["value"].(string); ok {
					value = []byte(v)
				} else {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				log.Debug("Plugin setting cache value", "key", key, "value", string(value), "raw_bytes", value)

				// Set value in cache
				if cache != nil {
					err = cache.Put([]byte(key), value)
					if err != nil {
						log.Debug("Plugin cache put failed", "key", key, "error", err)
						stack[0] = extism.EncodeU32(1) // error
						return
					}
					log.Debug("Plugin cache put success", "key", key)
				} else {
					stack[0] = extism.EncodeU32(1) // error - cache not available
					return
				}

				stack[0] = extism.EncodeU32(0) // success
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
		extism.NewHostFunctionWithStack(
			"get_store",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the key string from memory
				keyOffset := extism.DecodeU32(stack[0])
				key, err := p.ReadString(uint64(keyOffset))
				if err != nil {
					resp := CacheResponse{Success: false, Error: "Failed to read key"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				log.Debug("Plugin getting store value", "key", key)

				// Get value from store
				var value []byte
				if store != nil {
					value, err = store.Get([]byte(key))
					if err != nil {
						log.Debug("Plugin store get failed", "key", key, "error", err)
						resp := CacheResponse{Success: false, Error: err.Error()}
						respData, _ := json.Marshal(resp)
						offset, _ := p.WriteString(string(respData))
						stack[0] = offset
						return
					}
					log.Debug("Plugin store get success", "key", key, "value", string(value), "raw_bytes", value)
				} else {
					resp := CacheResponse{Success: false, Error: "Store not available"}
					respData, _ := json.Marshal(resp)
					offset, _ := p.WriteString(string(respData))
					stack[0] = offset
					return
				}

				// Return successful response with data
				resp := CacheResponse{Success: true, Data: string(value)}
				respData, _ := json.Marshal(resp)
				offset, _ := p.WriteString(string(respData))
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"set_store",
			func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
				// Get the request data from memory
				dataOffset := extism.DecodeU32(stack[0])
				requestData, err := p.ReadBytes(uint64(dataOffset))
				if err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				var req map[string]interface{}
				if err := json.Unmarshal(requestData, &req); err != nil {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				key, ok := req["key"].(string)
				if !ok {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				// Handle value as string
				var value []byte
				if v, ok := req["value"].(string); ok {
					value = []byte(v)
				} else {
					stack[0] = extism.EncodeU32(1) // error
					return
				}

				log.Debug("Plugin setting store value", "key", key, "value", string(value), "raw_bytes", value)

				// Set value in store
				if store != nil {
					err = store.Put([]byte(key), value)
					if err != nil {
						log.Debug("Plugin store put failed", "key", key, "error", err)
						stack[0] = extism.EncodeU32(1) // error
						return
					}
					log.Debug("Plugin store put success", "key", key)
				} else {
					stack[0] = extism.EncodeU32(1) // error - store not available
					return
				}

				stack[0] = extism.EncodeU32(0) // success
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
	}

	plugin, err := extism.NewPlugin(ctx, manifest, config, hostFunctions)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM plugin from %s: %w", wasmPath, err)
	}

	name := filepath.Base(wasmPath)
	if ext := filepath.Ext(name); ext != "" {
		name = name[:len(name)-len(ext)]
	}

	handler := &WasmHandler{
		plugin:  plugin,
		name:    name,
		dataDir: dataDir,
	}

	if err := handler.loadHelp(); err != nil {
		return nil, fmt.Errorf("failed to load help for WASM plugin %s: %w", wasmPath, err)
	}

	return handler, nil
}

func (w *WasmHandler) HandleMessage(msg *events.Message) error {
	// Extract command text from the message
	var messageText string
	if msg.Message.GetConversation() != "" {
		messageText = msg.Message.GetConversation()
	} else if msg.Message.GetExtendedTextMessage() != nil {
		messageText = msg.Message.GetExtendedTextMessage().GetText()
	}

	var text string
	// For wildcard handlers (name "*"), pass the full message text
	if w.name == "*" {
		text = messageText
	} else {
		// Extract command arguments (skip the command prefix and handler name)
		parts := strings.Fields(messageText)
		if len(parts) > 2 {
			text = strings.Join(parts[2:], " ")
		}
	}

	input := WasmInput{
		Message: text,
		Sender:  msg.Info.Chat.String(),
		Info: WasmMessageInfo{
			ID:        msg.Info.ID,
			Timestamp: msg.Info.Timestamp.Unix(),
			PushName:  msg.Info.PushName,
			IsGroup:   msg.Info.Chat.Server == types.GroupServer,
		},
	}

	inputData, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input for WASM plugin: %w", err)
	}

	exit, outputData, err := w.plugin.Call("handle_message", inputData)
	if err != nil {
		return fmt.Errorf("WASM plugin call failed with exit code %d: %w", exit, err)
	}

	var output WasmOutput
	if err := json.Unmarshal(outputData, &output); err != nil {
		return fmt.Errorf("failed to unmarshal WASM plugin output: %w", err)
	}

	if !output.Success {
		return fmt.Errorf("WASM plugin returned error: %s", output.Error)
	}

	if output.Reply != "" {
		return w.sendReply(msg.Info.Chat, output.Reply)
	}

	return nil
}

func (w *WasmHandler) Name() string {
	exit, outputData, err := w.plugin.Call("get_name", []byte{})
	if err != nil {
		return w.name
	}
	if exit != 0 {
		return w.name
	}
	return string(outputData)
}

func (w *WasmHandler) Topics() []string {
	exit, outputData, err := w.plugin.Call("get_topics", []byte{})
	if err != nil {
		// Fallback to plugin name for backward compatibility
		return []string{w.name}
	}
	if exit != 0 {
		return []string{w.name}
	}

	var topics []string
	if err := json.Unmarshal(outputData, &topics); err != nil {
		return []string{w.name}
	}
	return topics
}

func (w *WasmHandler) GetHelp() HandlerHelp {
	return w.help
}

func (w *WasmHandler) loadHelp() error {
	exit, outputData, err := w.plugin.Call("get_help", []byte{})
	if err != nil {
		w.help = HandlerHelp{
			Name:        w.name,
			Description: "WASM plugin (help not available)",
			Usage:       fmt.Sprintf(".sup %s", w.name),
			Examples:    []string{fmt.Sprintf(".sup %s", w.name)},
			Category:    "plugin",
		}
		return nil
	}

	if exit != 0 {
		return fmt.Errorf("get_help returned non-zero exit code: %d", exit)
	}

	var helpOutput WasmHelpOutput
	if err := json.Unmarshal(outputData, &helpOutput); err != nil {
		return fmt.Errorf("failed to unmarshal help output: %w", err)
	}

	w.help = HandlerHelp{
		Name:        helpOutput.Name,
		Description: helpOutput.Description,
		Usage:       helpOutput.Usage,
		Examples:    helpOutput.Examples,
		Category:    helpOutput.Category,
	}

	if w.help.Name == "" {
		w.help.Name = w.name
	}
	if w.help.Category == "" {
		w.help.Category = "plugin"
	}

	return nil
}

func (w *WasmHandler) sendReply(recipient types.JID, message string) error {
	c, err := client.GetClient()
	if err != nil {
		return fmt.Errorf("error getting client: %w", err)
	}

	c.SendText(recipient, message)
	return nil
}

func (w *WasmHandler) Close() error {
	if w.plugin != nil {
		ctx := context.Background()
		w.plugin.Close(ctx)
	}
	return nil
}

// Version returns the version of the WASM plugin
func (w *WasmHandler) Version() string {
	exit, outputData, err := w.plugin.Call("get_version", []byte{})
	if err != nil {
		return "unknown"
	}
	if exit != 0 {
		return "unknown"
	}
	return string(outputData)
}

// getRequiredEnvVars queries a WASM plugin for its required environment variables
func getRequiredEnvVars(plugin *extism.Plugin) ([]string, error) {
	exit, outputData, err := plugin.Call("get_required_env_vars", []byte{})
	if err != nil {
		// If the function doesn't exist, assume no environment variables are required
		return []string{}, nil
	}

	if exit != 0 {
		return nil, fmt.Errorf("get_required_env_vars returned non-zero exit code: %d", exit)
	}

	var envVars []string
	if err := json.Unmarshal(outputData, &envVars); err != nil {
		return nil, fmt.Errorf("failed to unmarshal required env vars: %w", err)
	}

	return envVars, nil
}

// sanitizePluginPath ensures the requested path is within the plugin's data directory
// and converts absolute paths to be relative to the data directory
func sanitizePluginPath(requestedPath, dataDir string) string {
	// Clean the requested path
	cleanPath := filepath.Clean(requestedPath)

	// If it's an absolute path, treat it as relative to dataDir
	if filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Clean(strings.TrimPrefix(cleanPath, "/"))
	}

	// Join with the data directory
	fullPath := filepath.Join(dataDir, cleanPath)

	// Ensure the final path is still within the data directory
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return ""
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return ""
	}

	// Check if the resolved path is within the data directory
	if !strings.HasPrefix(absFullPath, absDataDir+string(filepath.Separator)) && absFullPath != absDataDir {
		return ""
	}

	return fullPath
}
