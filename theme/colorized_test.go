package theme_test

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/svg"
	"github.com/belluikit/bellui/theme"
)

func TestNewColorizedResource_SameColourInEveryTheme(t *testing.T) {
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	for _, th := range []bellui.Theme{theme.LightTheme(), theme.DarkTheme()} {
		bellui.CurrentApp().Settings().SetTheme(th)
		res := theme.NewColorizedResource(theme.CancelIcon(), white)

		dec, err := svg.NewDecoder(bytes.NewReader(res.Content()))
		require.NoError(t, err)
		img, err := dec.Draw(24, 24)
		require.NoError(t, err)

		painted := 0
		for y := 0; y < 24; y++ {
			for x := 0; x < 24; x++ {
				c := img.NRGBAAt(x, y)
				if c.A == 0xff {
					painted++
					assert.Equal(t, white, c, "pixel %d,%d", x, y)
				}
			}
		}
		assert.Positive(t, painted, "the icon drew nothing")
	}
}

func TestNewColorizedResource_NameCarriesTheColour(t *testing.T) {
	src := helperNewStaticResource()
	white := theme.NewColorizedResource(src, color.White)
	red := theme.NewColorizedResource(src, color.NRGBA{R: 0xff, A: 0xff})

	assert.Equal(t, "colorized_ffffffff_content-remove.svg", white.Name())
	assert.NotEqual(t, white.Name(), red.Name())
	assert.NotEqual(t, white.Content(), red.Content())
}

func TestNewColorizedResource_ThemedSourceTintsTheOriginal(t *testing.T) {
	src := helperNewStaticResource()
	direct := theme.NewColorizedResource(src, color.White)
	viaTheme := theme.NewColorizedResource(theme.NewErrorThemedResource(theme.NewThemedResource(src)), color.White)

	assert.Equal(t, direct.Name(), viaTheme.Name())
	assert.Equal(t, direct.Content(), viaTheme.Content())
}

func TestNewColorizedResource_NotSVGReturnedUnchanged(t *testing.T) {
	png := helperLoadRes(t, "bellui.png")
	assert.Same(t, png, theme.NewColorizedResource(png, color.White))
}
