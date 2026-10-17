package templates

import (
	"bytes"
	"image/gif"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultIcon(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(DefaultIcon))
	require.NoError(t, err)
	assert.Equal(t, 1024, img.Bounds().Dx())
	assert.Equal(t, 1024, img.Bounds().Dy())
}

func TestSpinnersAreAnimated(t *testing.T) {
	for name, data := range map[string][]byte{"dark": SpinnerDark, "light": SpinnerLight} {
		g, err := gif.DecodeAll(bytes.NewReader(data))
		require.NoError(t, err, name)
		assert.Greater(t, len(g.Image), 1, name)
		assert.Equal(t, 0, g.LoopCount, name) // loop forever
	}
}

// The web page must declare a viewport, or phone browsers lay it out 980 CSS px
// wide: the app shows as a shrunken desktop page and its canvas gets about six
// times the pixels it needs.
func TestIndexHTMLHasViewport(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, IndexHTML.Execute(&out, struct {
		AppName, AppVersion, WasmFile string
		IsReleased, WebGPU            bool
	}{"App", "1.0.0", "App.wasm", true, true}))
	assert.Contains(t, out.String(), `<meta name="viewport" content="width=device-width, initial-scale=1`)
}
