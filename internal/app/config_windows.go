//go:build !ci && !android && !ios && !wasm && !noos && !tinygo

package app

import (
	"os"
	"path/filepath"
)

func rootConfigDir() string {
	homeDir, _ := os.UserHomeDir()

	desktopConfig := filepath.Join(filepath.Join(homeDir, "AppData"), "Roaming")
	return filepath.Join(desktopConfig, "bellui")
}
