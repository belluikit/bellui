package test

import (
	"image"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClipboard_FlavorsRoundTrip(t *testing.T) {
	cb := NewClipboard()

	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	ic, ok := cb.(bellui.ImageClipboard)
	require.True(t, ok)
	ic.SetImage(img)
	got, ok := ic.Image()
	assert.True(t, ok)
	assert.Equal(t, image.Image(img), got)

	files := []bellui.URI{storage.NewFileURI("/tmp/a.txt"), storage.NewFileURI("/tmp/b.txt")}
	fc, ok := cb.(bellui.FileClipboard)
	require.True(t, ok)
	fc.SetFiles(files)
	assert.Equal(t, files, fc.Files())

	hc, ok := cb.(bellui.HTMLClipboard)
	require.True(t, ok)
	hc.SetHTML("<b>rich</b>", "rich")
	html, ok := hc.HTML()
	assert.True(t, ok)
	assert.Equal(t, "<b>rich</b>", html)
	assert.Equal(t, "rich", cb.Content(), "SetHTML stores the plain fallback as Content")
}

func TestClipboard_FlavorsAreExclusive(t *testing.T) {
	cb := NewClipboard().(*clipboard)

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	files := []bellui.URI{storage.NewFileURI("/tmp/a.txt")}

	assertEmptyBut := func(t *testing.T, keep string) {
		t.Helper()
		if keep != "image" {
			_, ok := cb.Image()
			assert.False(t, ok)
		}
		if keep != "files" {
			assert.Nil(t, cb.Files())
		}
		if keep != "html" {
			_, ok := cb.HTML()
			assert.False(t, ok)
		}
		if keep != "content" && keep != "html" {
			assert.Empty(t, cb.Content())
		}
	}

	cb.SetImage(img)
	cb.SetContent("plain")
	assertEmptyBut(t, "content")
	assert.Equal(t, "plain", cb.Content())

	cb.SetImage(img)
	assertEmptyBut(t, "image")

	cb.SetFiles(files)
	assertEmptyBut(t, "files")

	cb.SetHTML("<i>x</i>", "x")
	assertEmptyBut(t, "html")
	assert.Equal(t, "x", cb.Content())

	cb.SetContent("")
	assertEmptyBut(t, "")
}

func TestClipboard_SetFlavors(t *testing.T) {
	cb := NewClipboard()
	mc, ok := cb.(bellui.MultiFlavorClipboard)
	require.True(t, ok)

	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	files := []bellui.URI{storage.NewFileURI("/tmp/a.txt")}
	mc.SetFlavors(bellui.ClipboardContent{
		Text:  "plain",
		HTML:  "<b>rich</b>",
		Image: img,
		Files: files,
	})

	// Unlike the single-flavor setters, every flavor coexists after the write.
	assert.Equal(t, "plain", cb.Content())
	html, ok := cb.(bellui.HTMLClipboard).HTML()
	assert.True(t, ok)
	assert.Equal(t, "<b>rich</b>", html)
	got, ok := cb.(bellui.ImageClipboard).Image()
	assert.True(t, ok)
	assert.Equal(t, image.Image(img), got)
	assert.Equal(t, files, cb.(bellui.FileClipboard).Files())

	// A partial write replaces the whole previous transaction.
	mc.SetFlavors(bellui.ClipboardContent{Text: "only text"})
	assert.Equal(t, "only text", cb.Content())
	_, ok = cb.(bellui.HTMLClipboard).HTML()
	assert.False(t, ok)
	_, ok = cb.(bellui.ImageClipboard).Image()
	assert.False(t, ok)
	assert.Nil(t, cb.(bellui.FileClipboard).Files())

	// A wholly zero value clears the clipboard.
	mc.SetFlavors(bellui.ClipboardContent{})
	assert.Empty(t, cb.Content())
}

func TestSetClipboardFlavors_FreeFunction(t *testing.T) {
	a := NewTempApp(t)

	ok := bellui.SetClipboardFlavors(bellui.ClipboardContent{Text: "combo", HTML: "<i>combo</i>"})
	assert.True(t, ok)
	assert.Equal(t, "combo", a.Clipboard().Content())
	html, ok := a.Clipboard().(bellui.HTMLClipboard).HTML()
	assert.True(t, ok)
	assert.Equal(t, "<i>combo</i>", html)
}

func TestClipboard_EmptyFlavors(t *testing.T) {
	cb := NewClipboard()

	img, ok := cb.(bellui.ImageClipboard).Image()
	assert.False(t, ok)
	assert.Nil(t, img)

	assert.Nil(t, cb.(bellui.FileClipboard).Files())

	html, ok := cb.(bellui.HTMLClipboard).HTML()
	assert.False(t, ok)
	assert.Empty(t, html)
}
