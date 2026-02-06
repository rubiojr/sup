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
	root    *os.Root
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

// ExecCommandRequest is the JSON structure for exec_command requests from plugins.
type ExecCommandRequest struct {
	Command string `json:"command"`
	Stdin   string `json:"stdin,omitempty"`
}

// ExecCommandResponse is the JSON structure returned to plugins from exec_command.
type ExecCommandResponse struct {
	Success  bool   `json:"success"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// StoreListResponse is the JSON structure returned to plugins from list_store.
type StoreListResponse struct {
	Success bool     `json:"success"`
	Keys    []string `json:"keys,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func NewWasmHandler(wasmPath string, cache cache.Cache, store store.Store, allowedCommands []string) (*WasmHandler, error) {
	ctx := context.Background()

	pluginDir := filepath.Dir(wasmPath)
	pluginName := stripExt(filepath.Base(wasmPath))
	dataDir := filepath.Join(pluginDir, "../plugin-data", pluginName)

	// Temporary plugin (noop) to query required environment variables
	noopHC := &hostContext{dataDir: dataDir, noop: true}
	tempPlugin, err := newExtismPlugin(ctx, wasmPath, noopHC.hostFunctions(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary WASM plugin from %s: %w", wasmPath, err)
	}

	requiredEnvVars, err := getRequiredEnvVars(tempPlugin)
	if err != nil {
		tempPlugin.Close(ctx)
		return nil, fmt.Errorf("failed to get required env vars from %s: %w", wasmPath, err)
	}
	tempPlugin.Close(ctx)

	// Build module config with only the required environment variables
	moduleConfig := wazero.NewModuleConfig()
	for _, name := range requiredEnvVars {
		if value := os.Getenv(name); value != "" {
			moduleConfig = moduleConfig.WithEnv(name, value)
		}
	}

	// Ensure plugin data directory exists
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create plugin data directory %s: %w", dataDir, err)
	}

	// Open a sandboxed root for filesystem operations
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin data root %s: %w", dataDir, err)
	}

	// Real plugin with actual host function implementations
	hc := &hostContext{
		dataDir:         dataDir,
		root:            root,
		cache:           cache,
		store:           store,
		allowedCommands: allowedCommands,
	}
	plugin, err := newExtismPlugin(ctx, wasmPath, hc.hostFunctions(), moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM plugin from %s: %w", wasmPath, err)
	}

	setupPluginLogger(plugin, wasmPath)

	handler := &WasmHandler{
		plugin:  plugin,
		name:    pluginName,
		dataDir: dataDir,
		root:    root,
	}

	if err := handler.loadHelp(); err != nil {
		return nil, fmt.Errorf("failed to load help for WASM plugin %s: %w", wasmPath, err)
	}

	return handler, nil
}

func newExtismPlugin(ctx context.Context, wasmPath string, hostFunctions []extism.HostFunction, moduleConfig wazero.ModuleConfig) (*extism.Plugin, error) {
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{Path: wasmPath},
		},
		AllowedHosts: []string{"*"},
		AllowedPaths: map[string]string{},
	}

	config := extism.PluginConfig{
		EnableWasi:   true,
		ModuleConfig: moduleConfig,
	}

	return extism.NewPlugin(ctx, manifest, config, hostFunctions)
}

func setupPluginLogger(plugin *extism.Plugin, wasmPath string) {
	extism.SetLogLevel(extism.LogLevelInfo)
	plugin.SetLogger(func(level extism.LogLevel, msg string) {
		switch level {
		case extism.LogLevelError:
			log.Error(msg, "plugin", filepath.Base(wasmPath))
		case extism.LogLevelWarn:
			log.Warn(msg, "plugin", filepath.Base(wasmPath))
		case extism.LogLevelInfo:
			log.Info(msg, "plugin", filepath.Base(wasmPath))
		default:
			log.Debug(msg, "plugin", filepath.Base(wasmPath))
		}
	})
}

func stripExt(name string) string {
	if ext := filepath.Ext(name); ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
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
	if w.root != nil {
		w.root.Close()
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

// SupportsCLI returns true if the plugin exports handle_cli.
func (w *WasmHandler) SupportsCLI() bool {
	return w.plugin.FunctionExists("handle_cli")
}

// CLIInput matches the plugin CLIInput type.
type CLIInput struct {
	Args []string `json:"args"`
}

// CLIOutput matches the plugin CLIOutput type.
type CLIOutput struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleCLI calls the plugin's handle_cli function with the given args.
func (w *WasmHandler) HandleCLI(args []string) (string, error) {
	input := CLIInput{Args: args}
	inputData, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal CLI input: %w", err)
	}

	exit, outputData, err := w.plugin.Call("handle_cli", inputData)
	if err != nil {
		return "", fmt.Errorf("plugin CLI call failed with exit code %d: %w", exit, err)
	}

	var output CLIOutput
	if err := json.Unmarshal(outputData, &output); err != nil {
		return "", fmt.Errorf("failed to parse CLI output: %w", err)
	}

	if !output.Success {
		return "", fmt.Errorf("%s", output.Error)
	}

	return output.Output, nil
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
