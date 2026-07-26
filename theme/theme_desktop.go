//go:build !android && !ios && !mobile && !wasm

package theme

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	internalApp "github.com/belluikit/bellui/internal/app"
)

func setupSystemTheme(fallback bellui.Theme) bellui.Theme {
	path := filepath.Join(internalApp.RootConfigDir(), "theme.json")
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			bellui.LogError("Failed to load user theme file: "+path, err)
		}
		return nil
	}
	defer f.Close()

	th, err := fromJSONWithFallback(bufio.NewReader(f), fallback)
	if err != nil {
		bellui.LogError("Failed to parse user theme file: "+path, err)
		return nil
	}

	return th
}
