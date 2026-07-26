package widget

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
)

// RichEntry's context menu (#888): a secondary tap — dispatched by the driver
// to the embedded selectable, the deepest SecondaryTappable — forwards to the
// editor's own menu, which offers the editing actions the selectable's
// read-only Copy menu cannot (and whose Select all drives the editor's
// document model, not the selectable's shadow state).

func popupMenuLabels(t *testing.T, c core.Canvas) []string {
	t.Helper()
	oc, ok := c.Overlays().Top().(*internalwidget.OverlayContainer)
	require.True(t, ok, "a popup menu overlay is shown")
	pm, ok := oc.Content.(*PopUpMenu)
	require.True(t, ok)
	var labels []string
	for _, it := range pm.Items {
		if mi, ok := it.(*menuItem); ok {
			labels = append(labels, mi.Item.Label)
		}
	}
	return labels
}

func TestRichEntry_TappedSecondary_EditorMenu(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "hello world"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(250, 100))
	c := w.Canvas()

	// The driver dispatches the secondary tap to the embedded selectable,
	// which forwards it to the editor.
	e.sel.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, 10)})

	assert.Equal(t, []string{"Cut", "Copy", "Paste", "Select all"}, popupMenuLabels(t, c))
	c.Overlays().Remove(c.Overlays().Top())

	// With edit history the menu grows Undo/Redo, like Entry's (the separator
	// is not a menu item widget, so it does not appear among the labels).
	e.TypedRune('x')
	e.InsertImage(image.NewRGBA(image.Rect(0, 0, 8, 8)))
	e.Undo()
	e.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, 10)})
	assert.Equal(t, []string{"Undo", "Redo", "Cut", "Copy", "Paste", "Select all"},
		popupMenuLabels(t, c), "undo/redo offered when available")
}

func TestRichEntry_MenuSelectAll_DrivesEditorModel(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "hello world"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(250, 100))
	c := w.Canvas()

	e.sel.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, 10)})
	oc, ok := c.Overlays().Top().(*internalwidget.OverlayContainer)
	require.True(t, ok)
	pm := oc.Content.(*PopUpMenu)
	for _, it := range pm.Items {
		if mi, ok := it.(*menuItem); ok && mi.Item.Label == "Select all" {
			mi.Item.Action()
		}
	}

	assert.Equal(t, "hello world", e.SelectedText(),
		"Select all from the menu selects through the editor's document model")
	assert.True(t, e.selecting)
}

// TestRichEntry_MenuSelectAll_ReShowsOnTouch verifies that a touch-initiated
// "Select all" re-presents the context menu over the full selection so Cut/Copy
// stay reachable, matching Entry and the native toolbar (#1128).
func TestRichEntry_MenuSelectAll_ReShowsOnTouch(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "hello world"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(250, 100))
	c := w.Canvas()

	e.touchPointer = true // touch-initiated menu
	e.TappedSecondary(&event.PointEvent{Position: geo.NewPos(10, 10)})

	oc, ok := c.Overlays().Top().(*internalwidget.OverlayContainer)
	require.True(t, ok)
	pm := oc.Content.(*PopUpMenu)
	c.Overlays().Remove(c.Overlays().Top()) // the menu dismisses on item tap
	for _, it := range pm.Items {
		if mi, ok := it.(*menuItem); ok && mi.Item.Label == "Select all" {
			mi.Item.Action()
		}
	}

	assert.Equal(t, "hello world", e.SelectedText())
	assert.Equal(t, []string{"Cut", "Copy", "Paste", "Select all"},
		popupMenuLabels(t, c), "menu re-presented over the full selection after touch Select all")
}
