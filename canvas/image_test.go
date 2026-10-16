package canvas_test

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/driver/software"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestImage_AlphaDefault(t *testing.T) {
	img := &canvas.Image{}

	assert.Equal(t, 1.0, img.Alpha())
}

func TestImage_TranslucencyDefault(t *testing.T) {
	img := &canvas.Image{}

	assert.Equal(t, 0.0, img.Translucency)
}

func TestImage_RefreshBlank(t *testing.T) {
	img := &canvas.Image{}
	img.Resize(geo.NewSize(64, 64))
	img.Refresh()
	assert.Nil(t, img.Image)

	img.Resource = theme.HomeIcon()
	img.Refresh()
	assert.NotNil(t, img.Image)

	img.Image = nil
	img.Resource = nil
	img.Refresh()
	assert.Nil(t, img.Image)
}

func TestNewImageFromFile(t *testing.T) {
	pwd, _ := os.Getwd()
	path := filepath.Join(filepath.Dir(pwd), "theme", "testdata", "bellui.png")

	img := canvas.NewImageFromFile(path)
	assert.NotNil(t, img)
	assert.Equal(t, path, img.File)
}

func TestNewImageFromReader(t *testing.T) {
	pwd, _ := os.Getwd()
	path := filepath.Join(filepath.Dir(pwd), "theme", "testdata", "bellui.png")
	read, err := os.Open(path)
	assert.NoError(t, err)
	defer read.Close()

	img := canvas.NewImageFromReader(read, "bellui.png")
	assert.NotNil(t, img)
	assert.Equal(t, "", img.File)
	assert.NotNil(t, img.Resource)
	assert.Equal(t, "bellui.png", img.Resource.Name())

	img.FillMode = canvas.ImageFillOriginal

	size := img.MinSize()
	assert.Equal(t, float32(512), size.Width)
	assert.Equal(t, float32(512), size.Height)
}

func TestNewImageFromURI_File(t *testing.T) {
	pwd, _ := os.Getwd()
	path := filepath.Join(filepath.Dir(pwd), "theme", "testdata", "bellui.png")

	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, "\\", "/")
	}

	img := canvas.NewImageFromURI(storage.NewFileURI(path))
	assert.NotNil(t, img)
	assert.Equal(t, path, img.File)

	img.FillMode = canvas.ImageFillOriginal

	size := img.MinSize()
	assert.Equal(t, float32(512), size.Width)
	assert.Equal(t, float32(512), size.Height)
}

func TestImage_CornerRadius(t *testing.T) {
	pwd, _ := os.Getwd()
	path := filepath.Join(filepath.Dir(pwd), "theme", "testdata", "bellui.png")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	logo, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	// The logo has a transparent background, so its corners look the same
	// with or without rounding. Flatten it onto an opaque background so the
	// goldens show where the corners are clipped.
	opaque := image.NewRGBA(logo.Bounds())
	draw.Draw(opaque, opaque.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), logo, logo.Bounds().Min, draw.Over)

	i := &canvas.Image{
		Image:        opaque,
		CornerRadius: 25,
	}
	c := software.NewCanvas()
	c.SetContent(i)
	c.Resize(geo.NewSize(120, 120))

	test.AssertRendersToImage(t, "image_rounded_corners.png", c)

	i.CornerRadius = canvas.RadiusMaximum
	c.Resize(geo.NewSize(60, 60))
	test.AssertRendersToImage(t, "image_fully_rounded_corners.png", c)

	i.FillMode = canvas.ImageFillCover
	i.CornerRadius = 20
	c.Resize(geo.NewSize(60, 100))
	test.AssertRendersToImage(t, "image_cover_rounded_corners.png", c)
}
