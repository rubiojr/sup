package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/rubiojr/sup/cache"
	"github.com/rubiojr/sup/internal/client"
	"github.com/rubiojr/sup/internal/log"
	"github.com/rubiojr/sup/store"
	"go.mau.fi/whatsmeow/types"
)

// hostContext holds shared dependencies for WASM host functions.
// When noop is true, functions return stubs without performing real I/O
// (used for the temporary plugin that queries env vars).
type hostContext struct {
	dataDir         string
	root            *os.Root
	cache           cache.Cache
	store           store.Store
	allowedCommands []string
	noop            bool
}

// hostFunctions returns all host functions wired to this context.
func (hc *hostContext) hostFunctions() []extism.HostFunction {
	return []extism.HostFunction{
		hc.readFileFunc(),
		hc.sendImageFunc(),
		hc.listDirectoryFunc(),
		hc.getCacheFunc(),
		hc.setCacheFunc(),
		hc.getStoreFunc(),
		hc.setStoreFunc(),
		hc.execCommandFunc(),
		hc.listStoreFunc(),
	}
}

// --- helpers ----------------------------------------------------------------

func (hc *hostContext) readString(p *extism.CurrentPlugin, stack []uint64) (string, error) {
	offset := extism.DecodeU32(stack[0])
	return p.ReadString(uint64(offset))
}

func (hc *hostContext) readBytes(p *extism.CurrentPlugin, stack []uint64) ([]byte, error) {
	offset := extism.DecodeU32(stack[0])
	return p.ReadBytes(uint64(offset))
}

func writeJSON(p *extism.CurrentPlugin, stack []uint64, v any) {
	data, _ := json.Marshal(v)
	offset, _ := p.WriteString(string(data))
	stack[0] = offset
}

func writeString(p *extism.CurrentPlugin, stack []uint64, s string) {
	offset, _ := p.WriteString(s)
	stack[0] = offset
}

// Signatures shared by most host functions.
var (
	i64In  = []extism.ValueType{extism.ValueTypeI64}
	i64Out = []extism.ValueType{extism.ValueTypeI64}
	i32Out = []extism.ValueType{extism.ValueTypeI32}
)

// --- read_file --------------------------------------------------------------

func (hc *hostContext) readFileFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"read_file",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			requestedPath, err := hc.readString(p, stack)
			if err != nil {
				writeString(p, stack, "")
				return
			}

			log.Debug("Plugin reading file", "requested_path", requestedPath, "data_dir", hc.dataDir)

			if hc.noop {
				writeString(p, stack, "")
				return
			}

			data, err := hc.root.ReadFile(cleanPluginPath(requestedPath))
			if err != nil {
				log.Warn("Plugin file read failed", "requested_path", requestedPath, "data_dir", hc.dataDir, "error", err)
				writeString(p, stack, "")
				return
			}

			offset, err := p.WriteString(string(data))
			if err != nil {
				writeString(p, stack, "")
				return
			}
			stack[0] = offset
		},
		i64In, i64Out,
	)
}

// --- send_image -------------------------------------------------------------

func (hc *hostContext) sendImageFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"send_image",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			requestData, err := hc.readBytes(p, stack)
			if err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			var req SendImageRequest
			if err := json.Unmarshal(requestData, &req); err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			cleanPath := cleanPluginPath(req.ImagePath)
			log.Info("Plugin sending image", "requested_path", req.ImagePath, "data_dir", hc.dataDir, "recipient", req.Recipient)

			if hc.noop {
				stack[0] = extism.EncodeU32(0)
				return
			}

			// Validate the path is inside the root before passing to SendImage
			if _, err := hc.root.Stat(cleanPath); err != nil {
				log.Warn("Plugin image send blocked", "requested_path", req.ImagePath, "data_dir", hc.dataDir, "error", err)
				stack[0] = extism.EncodeU32(1)
				return
			}

			c, err := client.GetClient()
			if err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			recipientJID, err := types.ParseJID(req.Recipient)
			if err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			// Use the root-resolved absolute path
			absPath := filepath.Join(hc.root.Name(), cleanPath)
			if err = c.SendImage(recipientJID, absPath); err != nil {
				log.Error("Plugin image send failed", "path", absPath, "recipient", req.Recipient, "error", err)
				stack[0] = extism.EncodeU32(1)
				return
			}

			stack[0] = extism.EncodeU32(0)
		},
		i64In, i32Out,
	)
}

// --- list_directory ---------------------------------------------------------

func (hc *hostContext) listDirectoryFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"list_directory",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			requestedPath, err := hc.readString(p, stack)
			if err != nil {
				writeJSON(p, stack, ListDirResponse{Success: false, Error: "Failed to read path"})
				return
			}

			log.Debug("Plugin listing directory", "requested_path", requestedPath, "data_dir", hc.dataDir)

			if hc.noop {
				writeJSON(p, stack, ListDirResponse{Success: true, Files: []string{}})
				return
			}

			cleanPath := cleanPluginPath(requestedPath)

			fileInfo, err := hc.root.Stat(cleanPath)
			if err != nil {
				log.Debug("Plugin directory listing failed", "requested_path", requestedPath, "data_dir", hc.dataDir, "error", err)
				writeJSON(p, stack, ListDirResponse{Success: false, Error: fmt.Sprintf("Directory not found: %s", err.Error())})
				return
			}

			if !fileInfo.IsDir() {
				writeJSON(p, stack, ListDirResponse{Success: false, Error: "Path is not a directory"})
				return
			}

			f, err := hc.root.Open(cleanPath)
			if err != nil {
				log.Debug("Plugin directory open failed", "requested_path", requestedPath, "data_dir", hc.dataDir, "error", err)
				writeJSON(p, stack, ListDirResponse{Success: false, Error: fmt.Sprintf("Failed to open directory: %s", err.Error())})
				return
			}
			defer f.Close()

			entries, err := f.ReadDir(-1)
			if err != nil {
				log.Debug("Plugin directory read failed", "requested_path", requestedPath, "data_dir", hc.dataDir, "error", err)
				writeJSON(p, stack, ListDirResponse{Success: false, Error: fmt.Sprintf("Failed to read directory: %s", err.Error())})
				return
			}

			var files []string
			for _, entry := range entries {
				files = append(files, entry.Name())
			}

			writeJSON(p, stack, ListDirResponse{Success: true, Files: files})
		},
		i64In, i64Out,
	)
}

// --- get_cache --------------------------------------------------------------

func (hc *hostContext) getCacheFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"get_cache",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if hc.noop {
				writeJSON(p, stack, CacheResponse{Success: false, Error: "Cache not available in temp plugin"})
				return
			}

			key, err := hc.readString(p, stack)
			if err != nil {
				writeJSON(p, stack, CacheResponse{Success: false, Error: "Failed to read key"})
				return
			}

			log.Debug("Plugin getting cache value", "key", key)

			if hc.cache == nil {
				writeJSON(p, stack, CacheResponse{Success: false, Error: "Cache not available"})
				return
			}

			value, err := hc.cache.Get([]byte(key))
			if err != nil {
				log.Debug("Plugin cache get failed", "key", key, "error", err)
				writeJSON(p, stack, CacheResponse{Success: false, Error: err.Error()})
				return
			}
			log.Debug("Plugin cache get success", "key", key, "value", string(value), "raw_bytes", value)

			writeJSON(p, stack, CacheResponse{Success: true, Data: string(value)})
		},
		i64In, i64Out,
	)
}

// --- set_cache --------------------------------------------------------------

func (hc *hostContext) setCacheFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"set_cache",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if hc.noop {
				stack[0] = extism.EncodeU32(0)
				return
			}

			requestData, err := hc.readBytes(p, stack)
			if err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			var req map[string]interface{}
			if err := json.Unmarshal(requestData, &req); err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			key, ok := req["key"].(string)
			if !ok {
				stack[0] = extism.EncodeU32(1)
				return
			}

			var value []byte
			if v, ok := req["value"].(string); ok {
				value = []byte(v)
			} else {
				stack[0] = extism.EncodeU32(1)
				return
			}

			log.Debug("Plugin setting cache value", "key", key, "value", string(value), "raw_bytes", value)

			if hc.cache == nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			if err = hc.cache.Put([]byte(key), value); err != nil {
				log.Debug("Plugin cache put failed", "key", key, "error", err)
				stack[0] = extism.EncodeU32(1)
				return
			}
			log.Debug("Plugin cache put success", "key", key)

			stack[0] = extism.EncodeU32(0)
		},
		i64In, i32Out,
	)
}

// --- get_store --------------------------------------------------------------

func (hc *hostContext) getStoreFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"get_store",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if hc.noop {
				writeJSON(p, stack, CacheResponse{Success: false, Error: "Store not available in temp plugin"})
				return
			}

			key, err := hc.readString(p, stack)
			if err != nil {
				writeJSON(p, stack, CacheResponse{Success: false, Error: "Failed to read key"})
				return
			}

			log.Debug("Plugin getting store value", "key", key)

			if hc.store == nil {
				writeJSON(p, stack, CacheResponse{Success: false, Error: "Store not available"})
				return
			}

			value, err := hc.store.Get([]byte(key))
			if err != nil {
				log.Debug("Plugin store get failed", "key", key, "error", err)
				writeJSON(p, stack, CacheResponse{Success: false, Error: err.Error()})
				return
			}
			log.Debug("Plugin store get success", "key", key, "value", string(value), "raw_bytes", value)

			writeJSON(p, stack, CacheResponse{Success: true, Data: string(value)})
		},
		i64In, i64Out,
	)
}

// --- set_store --------------------------------------------------------------

func (hc *hostContext) setStoreFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"set_store",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if hc.noop {
				stack[0] = extism.EncodeU32(0)
				return
			}

			requestData, err := hc.readBytes(p, stack)
			if err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			var req map[string]interface{}
			if err := json.Unmarshal(requestData, &req); err != nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			key, ok := req["key"].(string)
			if !ok {
				stack[0] = extism.EncodeU32(1)
				return
			}

			var value []byte
			if v, ok := req["value"].(string); ok {
				value = []byte(v)
			} else {
				stack[0] = extism.EncodeU32(1)
				return
			}

			log.Debug("Plugin setting store value", "key", key, "value", string(value), "raw_bytes", value)

			if hc.store == nil {
				stack[0] = extism.EncodeU32(1)
				return
			}

			if err = hc.store.Put([]byte(key), value); err != nil {
				log.Debug("Plugin store put failed", "key", key, "error", err)
				stack[0] = extism.EncodeU32(1)
				return
			}
			log.Debug("Plugin store put success", "key", key)

			stack[0] = extism.EncodeU32(0)
		},
		i64In, i32Out,
	)
}

// --- exec_command -----------------------------------------------------------

func (hc *hostContext) execCommandFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"exec_command",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if hc.noop {
				writeJSON(p, stack, ExecCommandResponse{Success: false, Error: "exec_command not available in temp plugin"})
				return
			}

			requestData, err := hc.readBytes(p, stack)
			if err != nil {
				writeJSON(p, stack, ExecCommandResponse{Success: false, Error: "failed to read request"})
				return
			}

			var req ExecCommandRequest
			if err := json.Unmarshal(requestData, &req); err != nil {
				writeJSON(p, stack, ExecCommandResponse{Success: false, Error: "invalid request JSON"})
				return
			}

			resp := executeWhitelistedCommand(req, hc.allowedCommands)
			writeJSON(p, stack, resp)
		},
		i64In, i64Out,
	)
}

// --- list_store -------------------------------------------------------------

func (hc *hostContext) listStoreFunc() extism.HostFunction {
	return extism.NewHostFunctionWithStack(
		"list_store",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			if hc.noop {
				writeJSON(p, stack, StoreListResponse{Success: false, Error: "list_store not available in temp plugin"})
				return
			}

			prefix, err := hc.readString(p, stack)
			if err != nil {
				writeJSON(p, stack, StoreListResponse{Success: false, Error: "failed to read prefix"})
				return
			}

			if hc.store == nil {
				writeJSON(p, stack, StoreListResponse{Success: false, Error: "Store not available"})
				return
			}

			keys, err := hc.store.List(prefix)
			if err != nil {
				writeJSON(p, stack, StoreListResponse{Success: false, Error: err.Error()})
				return
			}

			writeJSON(p, stack, StoreListResponse{Success: true, Keys: keys})
		},
		i64In, i64Out,
	)
}

// executeWhitelistedCommand runs a command only if it's in the allowed list.
func executeWhitelistedCommand(req ExecCommandRequest, allowedCommands []string) ExecCommandResponse {
	cmdParts := strings.Fields(req.Command)
	if len(cmdParts) == 0 {
		return ExecCommandResponse{Success: false, Error: "empty command"}
	}

	cmdName := cmdParts[0]
	allowed := false
	for _, ac := range allowedCommands {
		if ac == cmdName {
			allowed = true
			break
		}
	}
	if !allowed {
		log.Warn("Plugin exec_command blocked - command not whitelisted", "command", cmdName)
		return ExecCommandResponse{Success: false, Error: fmt.Sprintf("command %q not in allowed list", cmdName)}
	}

	var stdout, stderr bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	cmd.Stdin = strings.NewReader(req.Stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return ExecCommandResponse{
			Success:  false,
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
			Error:    err.Error(),
		}
	}

	return ExecCommandResponse{
		Success:  true,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}
}

// cleanPluginPath strips leading slashes so the path is relative,
// suitable for use with os.Root methods which enforce confinement.
func cleanPluginPath(requestedPath string) string {
	cleanPath := filepath.Clean(requestedPath)
	if filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Clean(strings.TrimPrefix(cleanPath, "/"))
	}
	return cleanPath
}
