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
