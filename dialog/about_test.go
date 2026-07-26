package dialog

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// collectAboutObjects flattens the container tree below obj.
func collectAboutObjects(obj core.CanvasObject) []core.CanvasObject {
	out := []core.CanvasObject{obj}
	switch c := obj.(type) {
	case *bellui.Container:
		for _, child := range c.Objects {
			out = append(out, collectAboutObjects(child)...)
		}
	case *container.Scroll:
		out = append(out, collectAboutObjects(c.Content)...)
	}
	return out
}

func aboutLabelTexts(obj core.CanvasObject) []string {
	var texts []string
	for _, o := range collectAboutObjects(obj) {
		if l, ok := o.(*widget.Label); ok {
			texts = append(texts, l.Text)
		}
	}
	return texts
}

func TestAboutContent(t *testing.T) {
	test.NewTempApp(t)

	u1, _ := url.Parse("https://example.com")
	u2, _ := url.Parse("https://example.org")
	links := []*widget.Hyperlink{
		widget.NewHyperlink("Site", u1),
		widget.NewHyperlink("Docs", u2),
	}
	meta := bellui.AppMetadata{Name: "Test App", Version: "1.2.3", Icon: theme.ComputerIcon()}

	obj := aboutContent("**hello** about", links, meta)
	texts := aboutLabelTexts(obj)
	assert.Contains(t, texts, "Test App")
	assert.Contains(t, texts, "Version 1.2.3")
	assert.Contains(t, texts, "·", "links are separated by a dot")

	var rich *widget.RichText
	var linkCount int
	for _, o := range collectAboutObjects(obj) {
		switch v := o.(type) {
		case *widget.Hyperlink:
			linkCount++
		case *widget.RichText:
			if len(v.Segments) > 1 || rich == nil {
				rich = v
			}
		}
	}
	assert.Equal(t, 2, linkCount)
	require.NotNil(t, rich, "markdown content is rendered as RichText")
	assert.False(t, rich.AllowRemoteContent, "remote content stays disabled for app-supplied markdown")
}

func TestAboutContent_EmptyMetadata(t *testing.T) {
	test.NewTempApp(t)

	obj := aboutContent("hello", nil, bellui.AppMetadata{})
	assert.Empty(t, aboutLabelTexts(obj), "no header labels without metadata")
	assert.Equal(t, "About", aboutTitle(bellui.AppMetadata{}))
	assert.Equal(t, "About Test", aboutTitle(bellui.AppMetadata{Name: "Test"}))
}

func TestNewAbout(t *testing.T) {
	a := test.NewTempApp(t)
	w := a.NewWindow("parent")
	defer w.Close()

	d := NewAbout("# Hi\nSome *about* text", nil, a, w)
	require.NotNil(t, d)
	d.Show()
	d.Hide()
}

func TestNewAboutWindow(t *testing.T) {
	a := test.NewTempApp(t)

	w := NewAboutWindow("Some text", nil, a)
	require.NotNil(t, w)
	assert.Equal(t, "About", w.Title(), "test app has no metadata name")
	w.Show()
	w.Close()
}

// --- #453: parallax underlay ---

func TestAboutContent_ParallaxUnderlay(t *testing.T) {
	test.NewTempApp(t)

	// Without an icon there is no underlay layer.
	plain := aboutContent("text", nil, bellui.AppMetadata{Name: "X"})
	_, isBorder := plain.(*bellui.Container)
	assert.True(t, isBorder)
	var clips int
	for _, o := range collectAboutObjects(plain) {
		if s, ok := o.(*container.Scroll); ok && s.Direction == container.ScrollNone {
			clips++
		}
	}
	assert.Zero(t, clips, "no underlay without an icon")

	meta := bellui.AppMetadata{Name: "X", Icon: theme.ComputerIcon()}
	long := strings.Repeat("A line of about text.\n\n", 40) // force scrollable overflow
	obj := aboutContent(long, nil, meta)
	w := test.NewTempWindow(t, obj)
	w.Resize(geo.NewSize(400, 300))

	var underlay *canvas.Image
	var contentScroll *container.Scroll
	for _, o := range collectAboutObjects(obj) {
		switch v := o.(type) {
		case *bellui.Container:
			if _, ok := v.Layout.(*parallaxLayout); ok && len(v.Objects) == 1 {
				underlay = v.Objects[0].(*canvas.Image)
			}
		case *container.Scroll:
			if v.Direction != container.ScrollNone {
				contentScroll = v
			}
		}
	}
	require.NotNil(t, underlay, "icon builds the parallax underlay")
	require.NotNil(t, contentScroll)
	assert.Greater(t, underlay.Translucency, 0.9, "underlay is almost transparent")

	// Scrolling the content drifts the underlay upward at a fraction of the
	// offset.
	before := underlay.Position().Y
	contentScroll.Scrolled(&event.ScrollEvent{Scrolled: geo.Delta{DY: -90}})
	after := underlay.Position().Y
	assert.Less(t, after, before, "underlay drifts up as the content scrolls down")
	assert.InDelta(t, before-contentScroll.Offset.Y/aboutParallaxFactor, after, 0.001)
}
