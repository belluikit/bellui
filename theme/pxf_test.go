package theme

import (
	"image/color"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage/repository"
)

func TestFromPXF(t *testing.T) {
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	repository.Register("file", intRepo.NewFileRepository())
	th, err := FromPXF(`
colors = { background: "#c0c0c0ff" }
colors_light = { foreground: "#ffffffff" }
sizes = { iconInline: 5.0 }
fonts = { monospace: "file://./testdata/NotoMono-Regular.ttf" }
icons = { cancel: "file://./testdata/cancel_Paths.svg" }
`)

	require.NoError(t, err)
	assert.Equal(t, &color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}, th.Color(ColorNameBackground, VariantDark))
	assert.Equal(t, &color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 0xff}, th.Color(ColorNameBackground, VariantLight))
	assert.Equal(t, &color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, th.Color(ColorNameForeground, VariantLight))
	assert.Equal(t, float32(5), th.Size(SizeNameInlineIcon))
	assert.Equal(t, "NotoMono-Regular.ttf", th.Font(bellui.TextStyle{Monospace: true}).Name())
	assert.Equal(t, "cancel_Paths.svg", th.Icon(IconNameCancel).Name())
	// Anything the data does not set comes from the fallback.
	assert.Equal(t, DefaultTheme().Color(ColorNamePrimary, VariantDark), th.Color(ColorNamePrimary, VariantDark))
}

func TestFromPXF_Errors(t *testing.T) {
	fallback := DarkTheme()
	th, err := FromPXFWithFallback(`colors = { foreground: "#12" }`, fallback)
	assert.Error(t, err, "an invalid color fails the load, as in JSON")
	assert.Equal(t, fallback, th)

	th, err = FromPXFWithFallback(`colors = {`, fallback)
	assert.Error(t, err)
	assert.Equal(t, fallback, th)
}

// TestFromPXF_MatchesJSON: the PXF and JSON fixtures hold the same theme, and
// read back the same values for every color, size, font and icon name.
func TestFromPXF_MatchesJSON(t *testing.T) {
	jsonData, err := os.ReadFile("./testdata/theme.json")
	require.NoError(t, err)
	pxfData, err := os.ReadFile("./testdata/theme.pxf")
	require.NoError(t, err)
	fromJSON, err := FromJSON(string(jsonData))
	require.NoError(t, err)
	fromPXF, err := FromPXF(string(pxfData))
	require.NoError(t, err)

	for _, name := range []bellui.ThemeColorName{ColorNameBackground, ColorNameForeground, ColorNamePrimary} {
		for _, v := range []bellui.ThemeVariant{VariantLight, VariantDark} {
			assert.Equal(t, fromJSON.Color(name, v), fromPXF.Color(name, v), "%s, variant %d", name, v)
		}
	}
	assert.Equal(t, float32(10), fromPXF.Size(SizeNameInlineIcon))
	assert.Equal(t, fromJSON.Size(SizeNameInlineIcon), fromPXF.Size(SizeNameInlineIcon))
	assert.Equal(t, fromJSON.Size(SizeNamePadding), fromPXF.Size(SizeNamePadding))
}
