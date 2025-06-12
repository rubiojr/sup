package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/extism/go-pdk"
	"github.com/rubiojr/sup/internal/log"
)

// MessageInfo contains information about the incoming message
type MessageInfo struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	PushName  string `json:"push_name"`
	IsGroup   bool   `json:"is_group"`
}

// Input represents the input data passed to a plugin
type Input struct {
	Message string      `json:"message"`
	Sender  string      `json:"sender"`
	Info    MessageInfo `json:"info"`
}

// Output represents the response from a plugin
type Output struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Reply   string `json:"reply,omitempty"`
}

// HelpOutput contains help information for a plugin
type HelpOutput struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Usage       string   `json:"usage"`
	Examples    []string `json:"examples"`
	Category    string   `json:"category"`
}

// Plugin interface that plugin authors must implement
type Plugin interface {
	Name() string
	Topics() []string
	HandleMessage(input Input) Output
	GetHelp() HelpOutput
	GetRequiredEnvVars() []string
	Version() string
}

// Global plugin instance
var pluginInstance Plugin

// RegisterPlugin registers a plugin instance with the framework
// This should be called from the plugin's main function
func RegisterPlugin(p Plugin) {
	pluginInstance = p
}

// handle_message is the exported function that Extism calls
// Plugin authors don't need to implement this directly
//
//export handle_message
func handle_message() int32 {
	if pluginInstance == nil {
		pdk.OutputString(`{"success":false,"error":"Plugin not registered. Call plugin.RegisterPlugin() in your main function."}`)
		return 1
	}

	input := pdk.Input()

	var req Input
	if err := json.Unmarshal(input, &req); err != nil {
		pdk.OutputString(fmt.Sprintf(`{"success":false,"error":"Failed to parse input: %v"}`, err))
		return 1
	}

	output := pluginInstance.HandleMessage(req)

	outputData, err := json.Marshal(output)
	if err != nil {
		pdk.OutputString(fmt.Sprintf(`{"success":false,"error":"Failed to marshal output: %v"}`, err))
		return 1
	}

	pdk.OutputString(string(outputData))
	return 0
}

// get_help is the exported function that Extism calls for help
// Plugin authors don't need to implement this directly
//
//export get_help
func get_help() int32 {
	if pluginInstance == nil {
		pdk.OutputString(`{"success":false,"error":"Plugin not registered. Call plugin.RegisterPlugin() in your main function."}`)
		return 1
	}

	help := pluginInstance.GetHelp()

	helpData, err := json.Marshal(help)
	if err != nil {
		pdk.OutputString(fmt.Sprintf(`{"success":false,"error":"Failed to marshal help: %v"}`, err))
		return 1
	}

	pdk.OutputString(string(helpData))
	return 0
}

// get_required_env_vars is the exported function that Extism calls to get required environment variables
// Plugin authors don't need to implement this directly
//
//export get_required_env_vars
func get_required_env_vars() int32 {
	if pluginInstance == nil {
		pdk.OutputString(`{"error":"Plugin not registered. Call plugin.RegisterPlugin() in your main function."}`)
		return 1
	}

	envVars := pluginInstance.GetRequiredEnvVars()

	envData, err := json.Marshal(envVars)
	if err != nil {
		pdk.OutputString(fmt.Sprintf(`{"error":"Failed to marshal env vars: %v"}`, err))
		return 1
	}

	pdk.OutputString(string(envData))
	return 0
}

// get_name is the exported function that Extism calls to get the plugin name
// Plugin authors don't need to implement this directly
//
//export get_name
func get_name() int32 {
	if pluginInstance == nil {
		pdk.OutputString(`{"error":"Plugin not registered. Call plugin.RegisterPlugin() in your main function."}`)
		return 1
	}

	name := pluginInstance.Name()
	pdk.OutputString(name)
	return 0
}

// get_topics is the exported function that Extism calls to get the plugin topics
// Plugin authors don't need to implement this directly
//
//export get_topics
func get_topics() int32 {
	if pluginInstance == nil {
		pdk.OutputString(`{"error":"Plugin not registered. Call plugin.RegisterPlugin() in your main function."}`)
		return 1
	}

	topics := pluginInstance.Topics()

	topicsData, err := json.Marshal(topics)
	if err != nil {
		pdk.OutputString(fmt.Sprintf(`{"error":"Failed to marshal topics: %v"}`, err))
		return 1
	}

	pdk.OutputString(string(topicsData))
	return 0
}

//export get_version
func get_version() int32 {
	if pluginInstance == nil {
		pdk.OutputString(`{"error":"Plugin not registered. Call plugin.RegisterPlugin() in your main function."}`)
		return 1
	}

	version := pluginInstance.Version()
	pdk.OutputString(version)
	return 0
}

// outputError is a helper function to output error responses
func outputError(message string) {
	output := Output{
		Success: false,
		Error:   message,
	}
	outputData, _ := json.Marshal(output)
	pdk.OutputString(string(outputData))
}

// Success creates a successful output with a reply message
func Success(reply string) Output {
	return Output{
		Success: true,
		Reply:   reply,
	}
}

// Error creates an error output with an error message
func Error(message string) Output {
	return Output{
		Success: false,
		Error:   message,
	}
}

// NewHelpOutput creates a new HelpOutput with the given parameters
func NewHelpOutput(name, description, usage string, examples []string, category string) HelpOutput {
	return HelpOutput{
		Name:        name,
		Description: description,
		Usage:       usage,
		Examples:    examples,
		Category:    category,
	}
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

// ReadFile reads a file from the host and returns its contents as bytes
func ReadFile(path string) ([]byte, error) {
	// Allocate memory for the path string
	pathMem := pdk.AllocateString(path)

	// Call the host function
	resultMem := hostReadFile(pathMem.Offset())

	// Read the result
	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to read file %s", path)
	}

	return data.ReadBytes(), nil
}

// SendImage sends an image to a recipient via WhatsApp
func SendImage(recipient, imagePath string) error {
	// Create the request data
	request := map[string]string{
		"recipient":  recipient,
		"image_path": imagePath,
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal send image request: %w", err)
	}

	// Allocate memory for the request data
	dataMem := pdk.AllocateBytes(requestData)

	// Call the host function
	result := hostSendImage(dataMem.Offset())
	if result != 0 {
		return fmt.Errorf("failed to send image, error code: %d", result)
	}

	return nil
}

// ListDirectoryResponse represents the response from listing a directory
type ListDirectoryResponse struct {
	Success bool     `json:"success"`
	Files   []string `json:"files,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// ListDirectory lists the contents of a directory within the plugin's data directory
func ListDirectory(path string) ([]string, error) {
	// Allocate memory for the path string
	pathMem := pdk.AllocateString(path)

	// Call the host function
	resultMem := hostListDirectory(pathMem.Offset())

	// Read the result
	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to list directory %s", path)
	}

	// Parse the JSON response
	var response ListDirectoryResponse
	err := json.Unmarshal(data.ReadBytes(), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory listing response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("directory listing failed: %s", response.Error)
	}

	return response.Files, nil
}

// CacheResponse represents the response from cache operations
type CacheResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// GetCache retrieves a value from the cache by key
func GetCache(key string) ([]byte, error) {
	// Allocate memory for the key string
	keyMem := pdk.AllocateString(key)

	// Call the host function
	resultMem := hostGetCache(keyMem.Offset())

	// Read the result
	data := pdk.FindMemory(resultMem)
	if data.Length() == 0 {
		return nil, fmt.Errorf("failed to get cache value for key %s", key)
	}

	// Parse the JSON response
	var response CacheResponse
	err := json.Unmarshal(data.ReadBytes(), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("cache get failed: %s", response.Error)
	}

	return []byte(response.Data), nil
}

// SetCache stores a value in the cache with the given key
func SetCache(key string, value []byte) error {
	log.Debug("foo")
	// Create the request data
	request := map[string]interface{}{
		"key":   key,
		"value": string(value),
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal cache request: %w", err)
	}

	// Allocate memory for the request data
	dataMem := pdk.AllocateBytes(requestData)

	// Call the host function
	result := hostSetCache(dataMem.Offset())
	if result != 0 {
		return fmt.Errorf("failed to set cache value, error code: %d", result)
	}

	return nil
}
