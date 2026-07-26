//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !noos && !tinygo

package app

import (
	"os"
	"path/filepath"
)

func rootConfigDir() string {
	desktopConfig, _ := os.UserConfigDir()
	return filepath.Join(desktopConfig, "bellui")
}
