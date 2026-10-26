package canvas_test

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// An image that declares a colour profile is drawn in its own colours: a
// Display P3 PNG is converted to sRGB when decoded, and matches what macOS
// ColorSync converts it to within one 8-bit step (#802). The fixtures are
// internal/colorprofile's.
func TestImage_ConvertsDeclaredProfileToSRGB(t *testing.T) {
	test.NewTempApp(t)
	dir := filepath.Join("..", "internal", "colorprofile", "testdata")
	data, err := os.ReadFile(filepath.Join(dir, "grid-p3.png"))
	require.NoError(t, err)
	img := canvas.NewImageFromResource(bellui.NewStaticResource("grid-p3.png", data))
	img.Resize(geo.NewSize(27, 27))
	img.Refresh()
	require.NotNil(t, img.Image)

	f, err := os.Open(filepath.Join(dir, "grid-p3.colorsync-srgb.png"))
	require.NoError(t, err)
	defer f.Close()
	want, err := png.Decode(f)
	require.NoError(t, err)
	raw, err := png.Decode(mustOpen(t, filepath.Join(dir, "grid-p3.png")))
	require.NoError(t, err)

	differ := 0
	for y := range 27 {
		for x := range 27 {
			got := color.NRGBAModel.Convert(img.Image.At(x, y)).(color.NRGBA)
			ref := color.NRGBAModel.Convert(want.At(x, y)).(color.NRGBA)
			if got != color.NRGBAModel.Convert(raw.At(x, y)).(color.NRGBA) {
				differ++
			}
			assert.LessOrEqual(t, absDiff(got.R, ref.R)+absDiff(got.G, ref.G)+absDiff(got.B, ref.B), 3,
				"pixel (%d,%d): drawn as %v, ColorSync gives %v", x, y, got, ref)
		}
	}
	assert.Greater(t, differ, 600, "the decoded image is the file's raw pixels: no conversion happened")
}

// An image with no profile is drawn exactly as decoded.
func TestImage_NoProfileUnchanged(t *testing.T) {
	test.NewTempApp(t)
	raw, err := os.ReadFile(filepath.Join("..", "theme", "testdata", "bellui.png"))
	require.NoError(t, err)
	img := canvas.NewImageFromResource(bellui.NewStaticResource("bellui.png", raw))
	img.Resize(geo.NewSize(64, 64))
	img.Refresh()
	require.NotNil(t, img.Image)
	plain, err := png.Decode(mustOpen(t, filepath.Join("..", "theme", "testdata", "bellui.png")))
	require.NoError(t, err)
	b := plain.Bounds()
	require.Equal(t, b, img.Image.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if p, g := color.NRGBAModel.Convert(plain.At(x, y)), color.NRGBAModel.Convert(img.Image.At(x, y)); p != g {
				t.Fatalf("pixel (%d,%d) is %v, decoded as %v: an image without a profile was changed", x, y, g, p)
			}
		}
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	return f
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
