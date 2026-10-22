//go:build !android && !ios && !mobile && !wasm

package theme

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The user theme file is theme.pxf, or the legacy theme.json while there is no
// theme.pxf (#523).
func TestUserThemeIn(t *testing.T) {
	red := &color.NRGBA{R: 0xff, A: 0xff}
	blue := &color.NRGBA{B: 0xff, A: 0xff}
	write := func(dir, name, data string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600))
	}

	t.Run("none", func(t *testing.T) {
		assert.Nil(t, userThemeIn(t.TempDir(), DefaultTheme()))
	})
	t.Run("legacy JSON", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "theme.json", `{"Colors": {"foreground": "#ff0000"}}`)
		th := userThemeIn(dir, DefaultTheme())
		require.NotNil(t, th)
		assert.Equal(t, red, th.Color(ColorNameForeground, VariantLight))
	})
	t.Run("PXF wins over JSON", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "theme.json", `{"Colors": {"foreground": "#ff0000"}}`)
		write(dir, "theme.pxf", `colors = { foreground: "#0000ff" }`)
		th := userThemeIn(dir, DefaultTheme())
		require.NotNil(t, th)
		assert.Equal(t, blue, th.Color(ColorNameForeground, VariantLight))
	})
	t.Run("broken PXF does not fall back to JSON", func(t *testing.T) {
		dir := t.TempDir()
		write(dir, "theme.json", `{"Colors": {"foreground": "#ff0000"}}`)
		write(dir, "theme.pxf", `colors = {`)
		assert.Nil(t, userThemeIn(dir, DefaultTheme()))
	})
}
