package playground

import (
	"encoding/base64"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	obj := canvas.NewRectangle(color.Black)
	obj.SetMinSize(geo.NewSquareSize(10))

	img := software.Render(obj, test.DarkTheme(theme.DefaultTheme()))
	assert.NotNil(t, img)

	enc, err := encodeImage(img)
	assert.NoError(t, err)
	assert.Equal(t, "iVBORw0KGgoAAAANSUhEUgAAAAoAAAAKCAIAAAACUFjqAAAAFElEQVR4nGJiwAtGpbECQAAAAP//DogAFaNSFa8AAAAASUVORK5CYII=", enc)

	bytes, err := base64.StdEncoding.DecodeString(enc)
	assert.NoError(t, err)
	assert.Equal(t, "PNG", string(bytes)[1:4])
}
