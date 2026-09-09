package playground

import (
	"encoding/base64"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/test"
	belluitest "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
	assert.Equal(t, "iVBORw0KGgoAAAANSUhEUgAAAAoAAAAKCAIAAAACUFjqAAAAFElEQVR4nGJiwAtGpbECQAAAAP//DogAFaNSFa8AAAAASUVORK5CYII=", enc)

	bytes, err := base64.StdEncoding.DecodeString(enc)
	assert.NoError(t, err)
	assert.Equal(t, "PNG", string(bytes)[1:4])
}
