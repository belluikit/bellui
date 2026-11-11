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

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
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

// countingResource counts how often the image reads its content.
type countingResource struct {
	bellui.Resource
	reads int
}

func (r *countingResource) Content() []byte {
	r.reads++
	return r.Resource.Content()
}

// A hidden image keeps a zero size, so it is never decoded: MinSize must not
// re-read its source every call once the aspect is known (#1062).
func TestImage_MinSizeZeroSizeReadsOnce(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "arc_80_100.png"))
	assert.NoError(t, err)
	res := &countingResource{Resource: bellui.NewStaticResource("arc.png", data)}
	img := canvas.NewImageFromResource(res)

	img.MinSize() // learns the aspect
	first := res.reads
	assert.NotZero(t, first)
	for range 100 {
		img.MinSize()
	}
	assert.Equal(t, first, res.reads)
	assert.Nil(t, img.Image)
	assert.NotZero(t, img.Aspect())
}

// MinSize used to decode an ImageFillOriginal file image on the call after
// layout gave it a size; now Resize must, or it draws nothing (#1062).
func TestImage_FillOriginalFileDrawsAfterLayout(t *testing.T) {
	test.NewTempApp(t)
	img := canvas.NewImageFromFile(filepath.Join("testdata", "arc_80_100.png"))
	img.FillMode = canvas.ImageFillOriginal
	w := test.NewTempWindow(t, container.NewStack(img))
	w.Resize(img.MinSize().AddWidthHeight(40, 40))

	out := w.Canvas().Capture()
	assert.NotNil(t, img.Image)
	coloured := 0
	b := out.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := out.At(x, y).RGBA()
			if r != g || g != bl { // the arc is coloured, the window grey
				coloured++
			}
		}
	}
	assert.NotZero(t, coloured)
}
