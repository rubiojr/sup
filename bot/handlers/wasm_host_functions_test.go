package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- cleanPluginPath tests --------------------------------------------------

func TestCleanPluginPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"relative path", "foo/bar.txt", "foo/bar.txt"},
		{"absolute path stripped", "/foo/bar.txt", "foo/bar.txt"},
		{"dot-dot collapsed", "foo/../bar.txt", "bar.txt"},
		{"leading dot-dot", "../escape.txt", filepath.Clean("../escape.txt")},
		{"double dot-dot", "../../etc/passwd", filepath.Clean("../../etc/passwd")},
		{"current dir", ".", "."},
		{"root slash", "/", "."},
		{"deeply nested", "a/b/c/d.txt", "a/b/c/d.txt"},
		{"trailing slash", "foo/bar/", "foo/bar"},
		{"double slashes", "foo//bar.txt", "foo/bar.txt"},
		{"absolute with dot-dot", "/foo/../bar.txt", "bar.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanPluginPath(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// --- os.Root confinement tests ----------------------------------------------

func TestRootConfinement(t *testing.T) {
	dataDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "allowed.txt"), []byte("hello"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "subdir", "nested.txt"), []byte("nested"), 0o644))

	// Create a file outside the root to test escapes
	outsideDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o644))

	root, err := os.OpenRoot(dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { root.Close() })

	t.Run("read allowed file", func(t *testing.T) {
		data, err := root.ReadFile(cleanPluginPath("allowed.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
	})

	t.Run("read nested file", func(t *testing.T) {
		data, err := root.ReadFile(cleanPluginPath("subdir/nested.txt"))
		require.NoError(t, err)
		assert.Equal(t, "nested", string(data))
	})

	t.Run("read with absolute path", func(t *testing.T) {
		data, err := root.ReadFile(cleanPluginPath("/allowed.txt"))
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
	})

	t.Run("read nonexistent file", func(t *testing.T) {
		_, err := root.ReadFile(cleanPluginPath("missing.txt"))
		assert.Error(t, err)
	})

	t.Run("dot-dot escape blocked", func(t *testing.T) {
		_, err := root.ReadFile(cleanPluginPath("../../../etc/passwd"))
		assert.Error(t, err)
	})

	t.Run("dot-dot through subdir blocked", func(t *testing.T) {
		_, err := root.ReadFile(cleanPluginPath("subdir/../../etc/passwd"))
		assert.Error(t, err)
	})

	t.Run("absolute escape blocked", func(t *testing.T) {
		_, err := root.ReadFile(cleanPluginPath("/etc/passwd"))
		// cleanPluginPath turns this into "etc/passwd" which doesn't exist in root
		assert.Error(t, err)
	})

	t.Run("symlink escape blocked", func(t *testing.T) {
		symlinkPath := filepath.Join(dataDir, "escape_link")
		require.NoError(t, os.Symlink(outsideDir, symlinkPath))

		_, err := root.ReadFile(cleanPluginPath("escape_link/secret.txt"))
		assert.Error(t, err, "symlink escaping the root should be blocked")
	})

	t.Run("stat via root", func(t *testing.T) {
		info, err := root.Stat(cleanPluginPath("subdir"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("list directory via root", func(t *testing.T) {
		f, err := root.Open(cleanPluginPath("subdir"))
		require.NoError(t, err)
		defer f.Close()

		entries, err := f.ReadDir(-1)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, "nested.txt", entries[0].Name())
	})

	t.Run("list root directory", func(t *testing.T) {
		f, err := root.Open(cleanPluginPath("."))
		require.NoError(t, err)
		defer f.Close()

		entries, err := f.ReadDir(-1)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(entries), 2, "should contain at least allowed.txt and subdir")
	})
}

// --- executeWhitelistedCommand tests ----------------------------------------

func TestExecuteWhitelistedCommand(t *testing.T) {
	t.Run("allowed command succeeds", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "echo hello"},
			[]string{"echo"},
		)
		assert.True(t, resp.Success)
		assert.Equal(t, "hello\n", resp.Stdout)
		assert.Equal(t, 0, resp.ExitCode)
	})

	t.Run("blocked command fails", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "rm -rf /"},
			[]string{"echo"},
		)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Error, "not in allowed list")
	})

	t.Run("empty command", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: ""},
			[]string{"echo"},
		)
		assert.False(t, resp.Success)
		assert.Equal(t, "empty command", resp.Error)
	})

	t.Run("empty allowed list", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "echo hi"},
			[]string{},
		)
		assert.False(t, resp.Success)
		assert.Contains(t, resp.Error, "not in allowed list")
	})

	t.Run("command with args", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "echo -n foo"},
			[]string{"echo"},
		)
		assert.True(t, resp.Success)
		assert.Equal(t, "foo", resp.Stdout)
	})

	t.Run("stdin forwarded", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "cat", Stdin: "from stdin"},
			[]string{"cat"},
		)
		assert.True(t, resp.Success)
		assert.Equal(t, "from stdin", resp.Stdout)
	})

	t.Run("nonzero exit code", func(t *testing.T) {
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "false"},
			[]string{"false"},
		)
		assert.False(t, resp.Success)
		assert.NotEqual(t, 0, resp.ExitCode)
	})

	t.Run("stderr captured", func(t *testing.T) {
		// Use a command that writes to stderr reliably
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "cat /dev/null/invalid"},
			[]string{"cat"},
		)
		assert.False(t, resp.Success)
		assert.NotEmpty(t, resp.Stderr)
	})

	t.Run("only base command checked against allowlist", func(t *testing.T) {
		// "sh" is allowed, args don't matter for allowlist check
		resp := executeWhitelistedCommand(
			ExecCommandRequest{Command: "sh -c echo works"},
			[]string{"sh"},
		)
		assert.True(t, resp.Success)
	})
}
