//go:build !test

package painter

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/belluikit/typesetting/fontscan"
)

func loadSystemFonts(fm *fontscan.FontMap) error {
	cacheDir := ""
	if runtime.GOOS == "android" {
		parent := os.Getenv("FILESDIR")
		cacheDir = filepath.Join(parent, "fontcache")
	}

	return fm.UseSystemFonts(cacheDir)
}
