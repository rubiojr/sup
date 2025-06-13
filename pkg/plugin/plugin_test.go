package plugin

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	extism "github.com/extism/go-sdk"
)

const testPluginPath = "testdata/test-plugin/test-plugin.wasm"

func buildTestPlugin(t *testing.T) {
	t.Helper()

	// Check if tinygo is available
	if _, err := exec.LookPath("tinygo"); err != nil {
		t.Skip("tinygo not available, skipping integration tests")
	}

	pluginDir := filepath.Join("testdata", "test-plugin")

	// Build the test plugin
	cmd := exec.Command("make", "build")
	cmd.Dir = pluginDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build test plugin: %v\nOutput: %s", err, output)
	}

	// Verify the WASM file exists
	if _, err := os.Stat(testPluginPath); os.IsNotExist(err) {
		t.Fatalf("Test plugin WASM file not found at %s", testPluginPath)
	}
}

func createPluginWithHostFunctions(t *testing.T) *extism.Plugin {
	t.Helper()

	buildTestPlugin(t)

	wasmBytes, err := os.ReadFile(testPluginPath)
	if err != nil {
		t.Fatalf("Failed to read test plugin WASM file: %v", err)
	}

	// Define host functions that the plugin expects
	hostFunctions := []extism.HostFunction{
		extism.NewHostFunctionWithStack(
			"extism:host/user/read_file",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				// Simple mock implementation
				stack[0] = 0
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"extism:host/user/send_image",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				stack[0] = 0
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
		extism.NewHostFunctionWithStack(
			"extism:host/user/list_directory",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				stack[0] = 0
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"extism:host/user/get_cache",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				// Mock implementation - return empty cache
				response := cacheResponse{
					Success: false,
					Error:   "cache not found",
				}
				data, _ := json.Marshal(response)
				offset, _ := plugin.WriteBytes(data)
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"extism:host/user/set_cache",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				// Mock implementation - always succeed
				stack[0] = 0
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
		extism.NewHostFunctionWithStack(
			"extism:host/user/get_store",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				// Mock implementation - return empty store
				response := cacheResponse{
					Success: false,
					Error:   "key not found",
				}
				data, _ := json.Marshal(response)
				offset, _ := plugin.WriteBytes(data)
				stack[0] = offset
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI64},
		),
		extism.NewHostFunctionWithStack(
			"extism:host/user/set_store",
			func(ctx context.Context, plugin *extism.CurrentPlugin, stack []uint64) {
				// Mock implementation - always succeed
				stack[0] = 0
			},
			[]extism.ValueType{extism.ValueTypeI64},
			[]extism.ValueType{extism.ValueTypeI32},
		),
	}

	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{Data: wasmBytes},
		},
		AllowedHosts: []string{"*"},
		Config: map[string]string{
			"TEST_ENV_VAR": "test-value",
		},
	}

	plugin, err := extism.NewPlugin(context.Background(), manifest, extism.PluginConfig{
		EnableWasi: true,
	}, hostFunctions)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	return plugin
}

// cacheResponse for testing
type cacheResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func TestPluginBuildsAndLoads(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	// If we get here, the plugin built and loaded successfully
	t.Log("Plugin built and loaded successfully")
}

func TestPluginGetName(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	_, output, err := plugin.Call("get_name", nil)
	if err != nil {
		t.Fatalf("Failed to call get_name: %v", err)
	}

	name := string(output)
	if name != "test-plugin" {
		t.Errorf("Expected plugin name 'test-plugin', got %q", name)
	}
}

func TestPluginGetVersion(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	_, output, err := plugin.Call("get_version", nil)
	if err != nil {
		t.Fatalf("Failed to call get_version: %v", err)
	}

	version := string(output)
	if version != "1.0.0" {
		t.Errorf("Expected plugin version '1.0.0', got %q", version)
	}
}

func TestPluginGetTopics(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	_, output, err := plugin.Call("get_topics", nil)
	if err != nil {
		t.Fatalf("Failed to call get_topics: %v", err)
	}

	var topics []string
	if err := json.Unmarshal(output, &topics); err != nil {
		t.Fatalf("Failed to unmarshal topics: %v", err)
	}

	expectedTopics := []string{"test", "echo", "greet"}
	if len(topics) != len(expectedTopics) {
		t.Errorf("Expected %d topics, got %d", len(expectedTopics), len(topics))
	}

	for i, expected := range expectedTopics {
		if i >= len(topics) || topics[i] != expected {
			t.Errorf("Expected topic %d to be %q, got %q", i, expected, topics[i])
		}
	}
}

func TestPluginGetHelp(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	_, output, err := plugin.Call("get_help", nil)
	if err != nil {
		t.Fatalf("Failed to call get_help: %v", err)
	}

	var help HelpOutput
	if err := json.Unmarshal(output, &help); err != nil {
		t.Fatalf("Failed to unmarshal help: %v", err)
	}

	if help.Name != "test-plugin" {
		t.Errorf("Expected help name 'test-plugin', got %q", help.Name)
	}

	if help.Category != "testing" {
		t.Errorf("Expected help category 'testing', got %q", help.Category)
	}

	if len(help.Examples) == 0 {
		t.Error("Expected help to have examples")
	}
}

func TestPluginGetRequiredEnvVars(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	_, output, err := plugin.Call("get_required_env_vars", nil)
	if err != nil {
		t.Fatalf("Failed to call get_required_env_vars: %v", err)
	}

	var envVars []string
	if err := json.Unmarshal(output, &envVars); err != nil {
		t.Fatalf("Failed to unmarshal env vars: %v", err)
	}

	expectedEnvVars := []string{"TEST_ENV_VAR"}
	if len(envVars) != len(expectedEnvVars) {
		t.Errorf("Expected %d env vars, got %d", len(expectedEnvVars), len(envVars))
	}

	if len(envVars) > 0 && envVars[0] != "TEST_ENV_VAR" {
		t.Errorf("Expected first env var to be 'TEST_ENV_VAR', got %q", envVars[0])
	}
}

func TestPluginHandleMessage(t *testing.T) {
	plugin := createPluginWithHostFunctions(t)
	defer plugin.Close(context.Background())

	tests := []struct {
		name          string
		input         Input
		expectedReply string
		expectSuccess bool
		replyContains string
	}{
		{
			name: "hello message",
			input: Input{
				Message: "hello",
				Sender:  "test@example.com",
				Info: MessageInfo{
					ID:        "msg-123",
					Timestamp: 1640995200,
					PushName:  "Test User",
					IsGroup:   false,
				},
			},
			expectSuccess: true,
			replyContains: "Hello Test User",
		},
		{
			name: "echo message",
			input: Input{
				Message: "echo world",
				Sender:  "test@example.com",
				Info: MessageInfo{
					ID:        "msg-124",
					Timestamp: 1640995201,
					PushName:  "Test User",
					IsGroup:   false,
				},
			},
			expectSuccess: true,
			expectedReply: "Echo: world",
		},
		{
			name: "error message",
			input: Input{
				Message: "error",
				Sender:  "test@example.com",
				Info: MessageInfo{
					ID:        "msg-125",
					Timestamp: 1640995202,
					PushName:  "Test User",
					IsGroup:   false,
				},
			},
			expectSuccess: false,
		},
		{
			name: "info message",
			input: Input{
				Message: "info",
				Sender:  "test@example.com",
				Info: MessageInfo{
					ID:        "msg-126",
					Timestamp: 1640995203,
					PushName:  "Test User",
					IsGroup:   true,
				},
			},
			expectSuccess: true,
			replyContains: "Message ID: msg-126",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal input: %v", err)
			}

			_, output, err := plugin.Call("handle_message", inputJSON)
			if err != nil {
				t.Fatalf("Failed to call handle_message: %v", err)
			}

			var result Output
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("Failed to unmarshal output: %v", err)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v. Error: %s", tt.expectSuccess, result.Success, result.Error)
			}

			if tt.expectedReply != "" && result.Reply != tt.expectedReply {
				t.Errorf("Expected reply %q, got %q", tt.expectedReply, result.Reply)
			}

			if tt.replyContains != "" && !contains(result.Reply, tt.replyContains) {
				t.Errorf("Expected reply to contain %q, got %q", tt.replyContains, result.Reply)
			}
		})
	}
}

// Test helper functions (these don't require WASM compilation)
func TestSuccess(t *testing.T) {
	reply := "Test reply message"
	output := Success(reply)

	if !output.Success {
		t.Errorf("Expected Success to be true, got %v", output.Success)
	}

	if output.Reply != reply {
		t.Errorf("Expected Reply to be %q, got %q", reply, output.Reply)
	}

	if output.Error != "" {
		t.Errorf("Expected Error to be empty, got %q", output.Error)
	}
}

func TestError(t *testing.T) {
	errorMsg := "Test error message"
	output := Error(errorMsg)

	if output.Success {
		t.Errorf("Expected Success to be false, got %v", output.Success)
	}

	if output.Error != errorMsg {
		t.Errorf("Expected Error to be %q, got %q", errorMsg, output.Error)
	}

	if output.Reply != "" {
		t.Errorf("Expected Reply to be empty, got %q", output.Reply)
	}
}

func TestNewHelpOutput(t *testing.T) {
	name := "test-plugin"
	description := "A test plugin"
	usage := "test [options]"
	examples := []string{"test --help", "test run"}
	category := "utility"

	help := NewHelpOutput(name, description, usage, examples, category)

	if help.Name != name {
		t.Errorf("Expected Name to be %q, got %q", name, help.Name)
	}

	if help.Description != description {
		t.Errorf("Expected Description to be %q, got %q", description, help.Description)
	}

	if help.Usage != usage {
		t.Errorf("Expected Usage to be %q, got %q", usage, help.Usage)
	}

	if len(help.Examples) != len(examples) {
		t.Errorf("Expected %d examples, got %d", len(examples), len(help.Examples))
	}

	for i, example := range examples {
		if help.Examples[i] != example {
			t.Errorf("Expected example %d to be %q, got %q", i, example, help.Examples[i])
		}
	}

	if help.Category != category {
		t.Errorf("Expected Category to be %q, got %q", category, help.Category)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
