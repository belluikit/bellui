package metadata

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestScaleIcon(t *testing.T) {
	data, err := bellui.LoadResourceFromPath("./testdata/bellui.png")
	assert.Nil(t, err)

	assert.Equal(t, "bellui.png", data.Name())
	img, err := png.Decode(bytes.NewReader(data.Content()))
	assert.Nil(t, err)
	assert.Equal(t, 512, img.Bounds().Dx())

	smallData := ScaleIcon(data, 256)

	assert.Equal(t, "bellui-256.png", smallData.Name())
	img, err = png.Decode(bytes.NewReader(smallData.Content()))
	assert.Nil(t, err)
	assert.Equal(t, 256, img.Bounds().Dx())
}
