//go:build ci || (mobile && !android && !ios) || (!linux && !darwin && !windows && !freebsd && !openbsd && !netbsd && !wasm && !noos && !tinygo)

package app

import (
	"os"
	"path/filepath"
)

func rootConfigDir() string {
	return filepath.Join(os.TempDir(), "bellui-test")
}
