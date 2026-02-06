//go:build !test

package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/extism/go-pdk"
)

var pluginInstance Plugin

// RegisterPlugin registers a plugin instance with the framework.
// This should be called from the plugin's main function.
func RegisterPlugin(p Plugin) {
	pluginInstance = p
}

//export handle_message
func handle_message() int32 {
	if pluginInstance == nil {
		pdk.OutputJSON(Output{Success: false, Error: "Plugin not registered. Call plugin.RegisterPlugin() in your main function."})
		return 1
	}

	var req Input
	if err := pdk.InputJSON(&req); err != nil {
		pdk.OutputJSON(Output{Success: false, Error: fmt.Sprintf("Failed to parse input: %v", err)})
		return 1
	}

	output := pluginInstance.HandleMessage(req)
	pdk.OutputJSON(output)
	return 0
}

//export get_help
func get_help() int32 {
	if pluginInstance == nil {
		pdk.OutputJSON(map[string]interface{}{"success": false, "error": "Plugin not registered. Call plugin.RegisterPlugin() in your main function."})
		return 1
	}

	help := pluginInstance.GetHelp()
	pdk.OutputJSON(help)
	return 0
}

//export get_required_env_vars
func get_required_env_vars() int32 {
	if pluginInstance == nil {
		pdk.OutputJSON(map[string]string{"error": "Plugin not registered. Call plugin.RegisterPlugin() in your main function."})
		return 1
	}

	envVars := pluginInstance.GetRequiredEnvVars()
	pdk.OutputJSON(envVars)
	return 0
}

//export get_name
func get_name() int32 {
	if pluginInstance == nil {
		pdk.OutputJSON(map[string]string{"error": "Plugin not registered. Call plugin.RegisterPlugin() in your main function."})
		return 1
	}

	name := pluginInstance.Name()
	pdk.OutputString(name)
	return 0
}

//export get_topics
func get_topics() int32 {
	if pluginInstance == nil {
		pdk.OutputJSON(map[string]string{"error": "Plugin not registered. Call plugin.RegisterPlugin() in your main function."})
		return 1
	}

	topics := pluginInstance.Topics()
	pdk.OutputJSON(topics)
	return 0
}

//export get_version
func get_version() int32 {
	if pluginInstance == nil {
		pdk.OutputJSON(map[string]string{"error": "Plugin not registered. Call plugin.RegisterPlugin() in your main function."})
		return 1
	}

	version := pluginInstance.Version()
	pdk.OutputString(version)
	return 0
}

//export handle_cli
func handle_cli() int32 {
	cliPlugin, ok := pluginInstance.(CLIPlugin)
	if !ok {
		pdk.OutputJSON(CLIOutput{Success: false, Error: "Plugin does not support CLI commands."})
		return 1
	}

	var req CLIInput
	if err := pdk.InputJSON(&req); err != nil {
		pdk.OutputJSON(CLIOutput{Success: false, Error: fmt.Sprintf("Failed to parse CLI input: %v", err)})
		return 1
	}

	output := cliPlugin.HandleCLI(req)
	pdk.OutputJSON(output)
	return 0
}

func outputError(message string) {
	pdk.OutputJSON(Output{Success: false, Error: message})
}

//go:wasmimport extism:host/user read_file
func hostReadFile(pathPtr uint64) uint64

//go:wasmimport extism:host/user send_image
func hostSendImage(dataPtr uint64) uint32

//go:wasmimport extism:host/user list_directory
func hostListDirectory(pathPtr uint64) uint64

//go:wasmimport extism:host/user get_cache
func hostGetCache(keyPtr uint64) uint64

//go:wasmimport extism:host/user set_cache
func hostSetCache(dataPtr uint64) uint32

//go:wasmimport extism:host/user get_store
func hostGetStore(keyPtr uint64) uint64

//go:wasmimport extism:host/user set_store
func hostSetStore(dataPtr uint64) uint32

//go:wasmimport extism:host/user list_store
func hostListStore(prefixPtr uint64) uint64

//go:wasmimport extism:host/user exec_command
func hostExecCommand(dataPtr uint64) uint64

// ExecCommandRequest is the request payload for ExecCommand.
type ExecCommandRequest struct {
	Command string `json:"command"`
	Stdin   string `json:"stdin,omitempty"`
}

// ExecCommandResponse is the response from ExecCommand.
type ExecCommandResponse struct {
	Success  bool   `json:"success"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// ExecCommand executes a whitelisted command on the host with optional stdin.
func ExecCommand(command, stdin string) (*ExecCommandResponse, error) {
	request := ExecCommandRequest{
		Command: command,
		Stdin:   stdin,
	}

	dataMem, err := pdk.AllocateJSON(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal exec request: %w", err)
	}

	resultMem := hostExecCommand(dataMem.Offset())
	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("exec_command returned empty response")
	}

	var resp ExecCommandResponse
	if err := json.Unmarshal(data.ReadBytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse exec response: %w", err)
	}

	return &resp, nil
}

// ReadFile reads a file from the host and returns its contents as bytes
func ReadFile(path string) ([]byte, error) {
	pathMem := pdk.AllocateString(path)
	resultMem := hostReadFile(pathMem.Offset())

	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to read file %s", path)
	}

	return data.ReadBytes(), nil
}

type sendImageRequest struct {
	Recipient string `json:"recipient"`
	ImagePath string `json:"image_path"`
}

// SendImage sends an image to a recipient via WhatsApp
func SendImage(recipient, imagePath string) error {
	request := sendImageRequest{
		Recipient: recipient,
		ImagePath: imagePath,
	}

	dataMem, err := pdk.AllocateJSON(request)
	if err != nil {
		return fmt.Errorf("failed to marshal send image request: %w", err)
	}

	result := hostSendImage(dataMem.Offset())
	if result != 0 {
		return fmt.Errorf("failed to send image, error code: %d", result)
	}

	return nil
}

type ListDirectoryResponse struct {
	Success bool     `json:"success"`
	Files   []string `json:"files,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// ListDirectory lists the contents of a directory within the plugin's data directory
func ListDirectory(path string) ([]string, error) {
	pathMem := pdk.AllocateString(path)
	resultMem := hostListDirectory(pathMem.Offset())

	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to list directory %s", path)
	}

	var response ListDirectoryResponse
	if err := json.Unmarshal(data.ReadBytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse directory listing response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("directory listing failed: %s", response.Error)
	}

	return response.Files, nil
}

type cacheResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// GetCache retrieves a value from the cache by key
func GetCache(key string) ([]byte, error) {
	keyMem := pdk.AllocateString(key)
	resultMem := hostGetCache(keyMem.Offset())

	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to get cache value for key %s", key)
	}

	var response cacheResponse
	if err := json.Unmarshal(data.ReadBytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse cache response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("cache get failed: %s", response.Error)
	}

	return []byte(response.Data), nil
}

type cacheRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SetCache stores a value in the cache with the given key
func SetCache(key string, value []byte) error {
	request := cacheRequest{
		Key:   key,
		Value: string(value),
	}

	dataMem, err := pdk.AllocateJSON(request)
	if err != nil {
		return fmt.Errorf("failed to marshal cache request: %w", err)
	}

	result := hostSetCache(dataMem.Offset())
	if result != 0 {
		return fmt.Errorf("failed to set cache value, error code: %d", result)
	}

	return nil
}

type storageImpl struct{}

func (s *storageImpl) Get(key string) ([]byte, error) {
	keyMem := pdk.AllocateString(key)
	resultMem := hostGetStore(keyMem.Offset())

	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to get store value for key %s", key)
	}

	var response cacheResponse
	if err := json.Unmarshal(data.ReadBytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse store response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("store get failed: %s", response.Error)
	}

	return []byte(response.Data), nil
}

func (s *storageImpl) Set(key string, value []byte) error {
	request := cacheRequest{
		Key:   key,
		Value: string(value),
	}

	dataMem, err := pdk.AllocateJSON(request)
	if err != nil {
		return fmt.Errorf("failed to marshal store request: %w", err)
	}

	result := hostSetStore(dataMem.Offset())
	if result != 0 {
		return fmt.Errorf("failed to set store value, error code: %d", result)
	}

	return nil
}

type storeListResponse struct {
	Success bool     `json:"success"`
	Keys    []string `json:"keys,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func (s *storageImpl) List(prefix string) ([]string, error) {
	prefixMem := pdk.AllocateString(prefix)
	resultMem := hostListStore(prefixMem.Offset())

	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to list store keys")
	}

	var response storeListResponse
	if err := json.Unmarshal(data.ReadBytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse store list response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("store list failed: %s", response.Error)
	}

	return response.Keys, nil
}

// Storage returns a Store interface for plugin storage operations
func Storage() Store {
	return &storageImpl{}
}

// GetStore retrieves a value from the store by key (deprecated, use Storage().Get() instead)
func GetStore(key string) ([]byte, error) {
	return Storage().Get(key)
}

// SetStore stores a value in the store with the given key (deprecated, use Storage().Set() instead)
func SetStore(key string, value []byte) error {
	return Storage().Set(key, value)
}
