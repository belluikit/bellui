package playground

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/test"
	belluitest "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	// software.Render resolves theme colours through bellui.CurrentApp. The app
	// used to arrive via driver/software's import of bellui/test; that import is
	// gone (#304), so install one explicitly.
	belluitest.NewTempApp(t)

	obj := canvas.NewRectangle(color.Black)
	obj.SetMinSize(geo.NewSquareSize(10))

	img := software.Render(obj, test.DarkTheme(theme.DefaultTheme()))
	assert.NotNil(t, img)

	enc, err := encodeImage(img)
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(enc)
	require.NoError(t, err)
	assert.Equal(t, "PNG", string(raw[1:4]), "the playground emits an inline PNG")

	// Decode and check the picture, not the bytes. Pinning the exact base64
	// pinned compress/flate's output, which go1.27 changed — the image was
	// identical and the assertion failed anyway, taking the macOS leg red on
	// every PR (#318).
	decoded, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)

	b := decoded.Bounds()
	assert.Equal(t, 10, b.Dx(), "rendered width")
	assert.Equal(t, 10, b.Dy(), "rendered height")

	// The object is an opaque black rectangle filling the whole image, so every
	// corner and the centre must be black. Sampling several points is what
	// makes this catch a wrong colour, a wrong size, or a blank render — the
	// regressions the old assertion caught only incidentally.
	for _, pt := range []image.Point{
		{X: b.Min.X, Y: b.Min.Y},
		{X: b.Max.X - 1, Y: b.Min.Y},
		{X: b.Min.X, Y: b.Max.Y - 1},
		{X: b.Max.X - 1, Y: b.Max.Y - 1},
		{X: b.Dx() / 2, Y: b.Dy() / 2},
	} {
		r, g, bl, a := decoded.At(pt.X, pt.Y).RGBA()
		assert.Equalf(t, [4]uint32{0, 0, 0, 0xffff}, [4]uint32{r, g, bl, a},
			"pixel at (%d,%d) should be opaque black", pt.X, pt.Y)
	}
}
