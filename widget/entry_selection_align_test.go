package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// TestEntry_SelectionAdoptsAlignment guards pre-rebrand tracker #781: a non-leading Entry alignment
// must propagate to the selectable so the selection highlight (and drag
// hit-testing) tracks the text edge instead of being pinned to the left.
//
// The Entry is laid out in a window, as in an app: the selection takes its
// width from that layout. Sized by hand here, it hid that the Entry never
// sized it (#1523).
func TestEntry_SelectionAdoptsAlignment(t *testing.T) {
	test.NewTempApp(t)

	e := NewEntry()
	e.Alignment = bellui.TextAlignTrailing
	e.SetText("hi")
	w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{e}})
	w.Resize(geo.NewSize(400, 100))
	e.Resize(geo.NewSize(300, 40))

	assert.Equal(t, bellui.TextAlignTrailing, e.sel.alignment,
		"selection must adopt the Entry's alignment (#781)")
	assert.Greater(t, e.sel.alignOffsetX(0), float32(0),
		"trailing alignment must offset the selection highlight")

	// Leading alignment stays at 0 (no regression).
	e.Alignment = bellui.TextAlignLeading
	e.syncSelectable()
	assert.Equal(t, bellui.TextAlignLeading, e.sel.alignment)
	assert.Equal(t, float32(0), e.sel.alignOffsetX(0))
}

// entryMark is a canvas object of the entry with its position in the entry.
type entryMark struct {
	x, y, w float32
	text    string // for text, its content
}

// entryDrawing lists the entry's visible text rows and selection highlights,
// positioned within the entry.
func entryDrawing(e *Entry) (texts, highlights []entryMark) {
	sel := e.Theme().Color("selection", bellui.CurrentApp().Settings().ThemeVariant())
	var walk func(o core.CanvasObject, at geo.Position)
	walk = func(o core.CanvasObject, at geo.Position) {
		if !o.Visible() {
			return
		}
		p := at.Add(o.Position())
		switch v := o.(type) {
		case *canvas.Text:
			if v.Text != "" {
				texts = append(texts, entryMark{p.X, p.Y, o.Size().Width, v.Text})
			}
		case *canvas.Rectangle:
			if v.FillColor == sel {
				highlights = append(highlights, entryMark{x: p.X, y: p.Y, w: o.Size().Width})
			}
		}
		var kids []core.CanvasObject
		switch v := o.(type) {
		case *bellui.Container:
			kids = v.Objects
		case core.Widget:
			kids = cache.Renderer(v).Objects()
		}
		for _, k := range kids {
			walk(k, p)
		}
	}
	walk(e, geo.Position{X: -e.Position().X, Y: -e.Position().Y})
	return texts, highlights
}

// A whole selection's highlight covers its text, wherever the text is entryMark:
// at the leading edge, centred, at the trailing edge, or right to left
// (#1523). It follows the text when the entry is resized while selected.
func TestEntry_SelectionHighlightCoversText(t *testing.T) {
	for _, c := range []struct {
		dir   bellui.Direction
		align bellui.TextAlign
	}{
		{bellui.DirectionLTR, bellui.TextAlignLeading},
		{bellui.DirectionLTR, bellui.TextAlignCenter},
		{bellui.DirectionLTR, bellui.TextAlignTrailing},
		{bellui.DirectionRTL, bellui.TextAlignLeading},
		{bellui.DirectionRTL, bellui.TextAlignTrailing},
	} {
		t.Run(fmt.Sprintf("dir%d/align%d", c.dir, c.align), func(t *testing.T) {
			test.NewTempApp(t)
			e := NewEntry()
			e.Alignment = c.align
			e.SetText("Person 3")
			w := inDirection(t, e, c.dir)
			w.Resize(geo.NewSize(400, 100))
			e.Resize(geo.NewSize(200, 40))
			w.Canvas().Focus(e)
			e.TypedShortcut(&bellui.ShortcutSelectAll{})

			for _, width := range []float32{200, 260} {
				e.Resize(geo.NewSize(width, 40))
				texts, highlights := entryDrawing(e)
				require.Len(t, texts, 1)
				require.Len(t, highlights, 1, "width %v", width)
				txt, hl := texts[0], highlights[0]
				assert.InDelta(t, txt.x, hl.x, 1.5, "width %v: the highlight starts where the text does", width)
				assert.InDelta(t, txt.x+txt.w, hl.x+hl.w, 1.5, "width %v: and ends where it does", width)
			}
		})
	}
}

// Part of a right-to-left row is highlighted under that part: "son" in
// "Person 3" starts after "Per".
func TestEntry_PartialSelectionHighlightRTL(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.SetText("Person 3")
	w := inDirection(t, e, bellui.DirectionRTL)
	w.Resize(geo.NewSize(400, 100))
	e.Resize(geo.NewSize(200, 40))
	w.Canvas().Focus(e)
	e.Select(0, 3, 0, 6)
	require.Equal(t, "son", e.SelectedText())

	texts, highlights := entryDrawing(e)
	require.Len(t, texts, 1)
	require.Len(t, highlights, 1)
	per := bellui.MeasureText("Per", e.Theme().Size("text"), e.TextStyle).Width
	son := bellui.MeasureText("son", e.Theme().Size("text"), e.TextStyle).Width
	assert.InDelta(t, texts[0].x+per, highlights[0].x, 1.5, "the highlight starts after \"Per\"")
	assert.InDelta(t, son, highlights[0].w, 1.5, "and is as wide as \"son\"")
}

// Each row of a right-to-left multi-line entry is highlighted under its own
// text, rows of different widths included.
func TestEntry_MultiLineSelectionHighlightRTL(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	e.SetText("Person 3\nA much longer row")
	w := inDirection(t, e, bellui.DirectionRTL)
	w.Resize(geo.NewSize(400, 200))
	e.Resize(geo.NewSize(300, 120))
	w.Canvas().Focus(e)
	e.TypedShortcut(&bellui.ShortcutSelectAll{})

	texts, highlights := entryDrawing(e)
	require.Len(t, texts, 2)
	require.Len(t, highlights, 2)
	for i := range texts {
		assert.InDelta(t, texts[i].x, highlights[i].x, 1.5, "row %d: the highlight starts where its text does", i)
		assert.InDelta(t, texts[i].x+texts[i].w, highlights[i].x+highlights[i].w, 1.5, "row %d: and ends where it does", i)
	}
}

// inDirection puts e in a window laid out in dir.
func inDirection(t *testing.T, e *Entry, dir bellui.Direction) bellui.Window {
	t.Helper()
	w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{e}})
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(dir)
	return w
}
