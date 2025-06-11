package botfs

import (
	"os"
	"path/filepath"
)

func HandlersDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local/share/sup/handlers"), nil
}

func HandlerDataDir(handler string) (string, error) {
	d, err := HandlersDataDir()
	return filepath.Join(d, handler), err
}
