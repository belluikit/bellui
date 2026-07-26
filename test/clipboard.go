package test

import (
	"image"

	"github.com/belluikit/bellui"
)

// Declare conformity with the clipboard interfaces
var (
	_ bellui.Clipboard            = (*clipboard)(nil)
	_ bellui.ImageClipboard       = (*clipboard)(nil)
	_ bellui.FileClipboard        = (*clipboard)(nil)
	_ bellui.HTMLClipboard        = (*clipboard)(nil)
	_ bellui.MultiFlavorClipboard = (*clipboard)(nil)
)

type clipboard struct {
	content string
	image   image.Image
	files   []bellui.URI
	html    string
	hasHTML bool
}

func (c *clipboard) Content() string {
	return c.content
}

func (c *clipboard) SetContent(content string) {
	c.clear()
	c.content = content
}

// Image returns the image on the clipboard, or (nil, false) when the
// clipboard holds no image.
func (c *clipboard) Image() (image.Image, bool) {
	if c.image == nil {
		return nil, false
	}
	return c.image, true
}

// SetImage replaces the clipboard content with img.
func (c *clipboard) SetImage(img image.Image) {
	c.clear()
	c.image = img
}

// Files returns the list of files on the clipboard, or nil when the clipboard
// holds no files.
func (c *clipboard) Files() []bellui.URI {
	return c.files
}

// SetFiles replaces the clipboard content with the given files.
func (c *clipboard) SetFiles(files []bellui.URI) {
	c.clear()
	c.files = files
}

// HTML returns the HTML markup on the clipboard, or ("", false) when the
// clipboard holds no HTML flavor.
func (c *clipboard) HTML() (string, bool) {
	if !c.hasHTML {
		return "", false
	}
	return c.html, true
}

// SetHTML replaces the clipboard content with html, storing plainFallback as
// the plain-text flavor of the same transaction.
func (c *clipboard) SetHTML(html, plainFallback string) {
	c.clear()
	c.html = html
	c.hasHTML = true
	c.content = plainFallback
}

// SetFlavors replaces the clipboard content with every set flavor of content
// in one transaction.
func (c *clipboard) SetFlavors(content bellui.ClipboardContent) {
	c.clear()
	c.content = content.Text
	c.image = content.Image
	c.files = content.Files
	if content.HTML != "" {
		c.html = content.HTML
		c.hasHTML = true
	}
}

func (c *clipboard) clear() {
	*c = clipboard{}
}

// NewClipboard returns a single use in-memory clipboard used for testing
func NewClipboard() bellui.Clipboard {
	return &clipboard{}
}
