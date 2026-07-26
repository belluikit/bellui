package bellui

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

// textOnlyClipboard supports no optional flavors.
type textOnlyClipboard struct {
	content string
}

func (c *textOnlyClipboard) Content() string           { return c.content }
func (c *textOnlyClipboard) SetContent(content string) { c.content = content }

// flavorClipboard records flavored content in memory.
type flavorClipboard struct {
	textOnlyClipboard
	image   image.Image
	files   []URI
	html    string
	hasHTML bool
}

var (
	_ ImageClipboard = (*flavorClipboard)(nil)
	_ FileClipboard  = (*flavorClipboard)(nil)
	_ HTMLClipboard  = (*flavorClipboard)(nil)
)

func (c *flavorClipboard) Image() (image.Image, bool) { return c.image, c.image != nil }
func (c *flavorClipboard) SetImage(img image.Image)   { c.image = img }
func (c *flavorClipboard) Files() []URI               { return c.files }
func (c *flavorClipboard) SetFiles(files []URI)       { c.files = files }
func (c *flavorClipboard) HTML() (string, bool)       { return c.html, c.hasHTML }
func (c *flavorClipboard) SetHTML(html, plain string) {
	c.html = html
	c.hasHTML = true
	c.content = plain
}

// clipboardApp is a dummyApp whose Clipboard() returns a real value.
type clipboardApp struct {
	dummyApp
	clipboard Clipboard
}

func (a *clipboardApp) Clipboard() Clipboard { return a.clipboard }

func TestClipboardSugar_Flavored(t *testing.T) {
	cb := &flavorClipboard{}
	SetCurrentApp(&clipboardApp{clipboard: cb})
	defer SetCurrentApp(nil)

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	assert.True(t, SetClipboardImage(img))
	got, ok := ClipboardImage()
	assert.True(t, ok)
	assert.Equal(t, image.Image(img), got)

	files := []URI{nil} // value identity is all the sugar forwards
	assert.True(t, SetClipboardFiles(files))
	assert.Equal(t, files, ClipboardFiles())

	assert.True(t, SetClipboardHTML("<b>hi</b>", "hi"))
	html, ok := ClipboardHTML()
	assert.True(t, ok)
	assert.Equal(t, "<b>hi</b>", html)
	assert.Equal(t, "hi", cb.Content(), "SetHTML must write the plain fallback")
}

func TestClipboardSugar_TextOnlyClipboard(t *testing.T) {
	SetCurrentApp(&clipboardApp{clipboard: &textOnlyClipboard{}})
	defer SetCurrentApp(nil)

	assertClipboardSugarUnsupported(t)
}

func TestClipboardSugar_NoApp(t *testing.T) {
	SetCurrentApp(nil)

	assertClipboardSugarUnsupported(t)
}

func assertClipboardSugarUnsupported(t *testing.T) {
	t.Helper()

	img, ok := ClipboardImage()
	assert.False(t, ok)
	assert.Nil(t, img)
	assert.False(t, SetClipboardImage(image.NewNRGBA(image.Rect(0, 0, 1, 1))))

	assert.Nil(t, ClipboardFiles())
	assert.False(t, SetClipboardFiles(nil))

	html, ok := ClipboardHTML()
	assert.False(t, ok)
	assert.Empty(t, html)
	assert.False(t, SetClipboardHTML("<b>x</b>", "x"))
}
