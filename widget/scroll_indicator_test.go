package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	iwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
)

func TestNewScrollIndicator(t *testing.T) {
	content := NewLabel("Hello\nWorld\nThis is a long text\nwith many lines\nto scroll through")
	scroll := iwidget.NewScroll(content)
	si := NewScrollIndicator(scroll)

	assert.NotNil(t, si)
	assert.Equal(t, scroll, si.Scroll)
}

func TestScrollIndicator_Render(t *testing.T) {
	content := NewLabel("Test")
	scroll := iwidget.NewScroll(content)
	si := NewScrollIndicator(scroll)
	r := test.WidgetRenderer(si)

	assert.NotNil(t, r)
	// MinSize is (0,0) since the indicator is an overlay that stretches to fill its parent
	min := si.MinSize()
	assert.Equal(t, float32(0), min.Width)
	assert.Equal(t, float32(0), min.Height)
	// But it should have renderable objects
	assert.Equal(t, 2, len(r.Objects()))
}

func TestScrollIndicator_UpdatesOnScroll(t *testing.T) {
	content := NewLabel("Hello\nWorld\nLine3\nLine4\nLine5\nLine6\nLine7\nLine8\nLine9\nLine10")
	scroll := iwidget.NewScroll(content)
	scroll.Resize(geo.NewSize(100, 50))
	si := NewScrollIndicator(scroll)
	_ = test.WidgetRenderer(si)

	// Initial state
	si.Refresh()
	assert.Equal(t, float32(0), si.offset.Y)

	// Scroll down
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -20)})
	assert.Greater(t, si.offset.Y, float32(0))
}

func TestScrollIndicator_ChainsOnScrolled(t *testing.T) {
	var chained bool
	content := NewLabel("Test")
	scroll := iwidget.NewScroll(content)
	scroll.OnScrolled = func(_ geo.Position) { chained = true }

	si := NewScrollIndicator(scroll)
	_ = si

	// The chained callback should still work
	scroll.Resize(geo.NewSize(100, 10))
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -5)})
	assert.True(t, chained)
}

func TestScrollIndicator_DestroyDetaches(t *testing.T) {
	var chainedCalls int
	original := func(_ geo.Position) { chainedCalls++ }
	content := NewLabel("Hello\nWorld\nLine3\nLine4\nLine5\nLine6\nLine7\nLine8\nLine9\nLine10")
	scroll := iwidget.NewScroll(content)
	scroll.OnScrolled = original
	scroll.Resize(geo.NewSize(100, 50))

	si := NewScrollIndicator(scroll)
	r := test.WidgetRenderer(si)
	assert.NotNil(t, si.removeListener)

	// While attached, scrolling updates the indicator and the unrelated
	// OnScrolled callback still fires (the indicator no longer wraps it).
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -20)})
	assert.Greater(t, si.offset.Y, float32(0))
	assert.Equal(t, 1, chainedCalls)

	// Destroying the renderer must remove the listener and stop the indicator
	// from being driven by future scrolls (no leaked reference).
	r.Destroy()
	assert.Nil(t, si.removeListener)
	prevOffset := si.offset.Y
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -20)})
	assert.Equal(t, prevOffset, si.offset.Y, "indicator should not update after detach")
	assert.Equal(t, 2, chainedCalls, "OnScrolled should still fire after detach")

	// Re-creating the renderer (hide/show cycle) re-attaches.
	_ = si.CreateRenderer()
	assert.NotNil(t, si.removeListener)
	scroll.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -20)})
	assert.Greater(t, si.offset.Y, prevOffset, "indicator should update again after re-attach")
	assert.Equal(t, 3, chainedCalls)
}
