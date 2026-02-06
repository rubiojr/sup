package botfs

import (
	"os"
	"path/filepath"
)

func HandlersDataDir() string {
	return filepath.Join(DataDir(), "handlers")
}

func DataDir() string {
	return filepath.Join(UserHome(), ".local/share/sup")
}

func ConfigDir() string {
	return filepath.Join(UserHome(), ".config/sup")
}

func HandlerDataDir(handler string) string {
	return filepath.Join(DataDir(), handler)
}

func UserHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// We need a home dir, this should only panic in rare circumstances
		// where we actually want to panic.
		panic(err)
	}
	return home
}
