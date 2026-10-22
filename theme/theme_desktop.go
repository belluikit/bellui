//go:build !android && !ios && !mobile && !wasm

package theme

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/belluikit/bellui"
	internalApp "github.com/belluikit/bellui/internal/app"
)

// setupSystemTheme reads the user-wide theme file from Bellui's config folder:
// theme.pxf, or the legacy theme.json when there is no theme.pxf (#523).
func setupSystemTheme(fallback bellui.Theme) bellui.Theme {
	return userThemeIn(internalApp.RootConfigDir(), fallback)
}

func userThemeIn(dir string, fallback bellui.Theme) bellui.Theme {
	if th, found := loadUserTheme(filepath.Join(dir, "theme.pxf"), fromPXFWithFallback, fallback); found {
		return th
	}
	th, _ := loadUserTheme(filepath.Join(dir, "theme.json"), fromJSONWithFallback, fallback)
	return th
}

// loadUserTheme reads the theme file at path, reporting whether it exists. A
// file that exists but fails to load is logged and gives a nil theme.
func loadUserTheme(path string, parse func(io.Reader, bellui.Theme) (bellui.Theme, error), fallback bellui.Theme) (bellui.Theme, bool) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			bellui.LogError("Failed to load user theme file: "+path, err)
			return nil, true
		}
		return nil, false
	}
	defer f.Close()

	th, err := parse(bufio.NewReader(f), fallback)
	if err != nil {
		bellui.LogError("Failed to parse user theme file: "+path, err)
		return nil, true
	}
	return th, true
}
