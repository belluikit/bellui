package mobile

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// TestWriteIOSAppIcon checks the AppIcon set handed to Xcode is complete:
// every file its Contents.json names exists at the size the slot needs. The
// set is compiled inside xcodebuild so the icons are signed with the app
// (#677); a missing or wrongly sized file would fail that build or ship a
// blank icon.
func TestWriteIOSAppIcon(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("needs sips (macOS)")
	}
	if _, err := exec.LookPath("sips"); err != nil {
		t.Skip("needs sips")
	}
	setDir := filepath.Join(t.TempDir(), "AppIcon.appiconset")
	icon := filepath.Join(t.TempDir(), "icon.png")
	require.NoError(t, os.WriteFile(icon, templates.DefaultIcon, 0o600))

	require.NoError(t, writeIOSAppIcon(icon, setDir))

	var contents struct {
		Images []struct {
			Filename, Size, Scale string
		}
	}
	data, err := os.ReadFile(filepath.Join(setDir, "Contents.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &contents))

	named := 0
	for _, img := range contents.Images {
		if img.Filename == "" {
			continue
		}
		named++
		f, err := os.Open(filepath.Join(setDir, img.Filename))
		require.NoError(t, err, "Contents.json names %s", img.Filename)
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		require.NoError(t, err)

		var pt float64
		var scale int
		_, err = fmt.Sscanf(img.Size, "%gx", &pt)
		require.NoError(t, err, img.Size)
		_, err = fmt.Sscanf(img.Scale, "%dx", &scale)
		require.NoError(t, err, img.Scale)
		want := int(pt * float64(scale))
		assert.Equal(t, want, cfg.Width, img.Filename)
		assert.Equal(t, want, cfg.Height, img.Filename)
	}
	assert.Equal(t, len(iosAppIconSizes), named, "every resized PNG is named by Contents.json")
}
