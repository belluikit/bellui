package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// A disabled RichEntry is visible but inert (#1216): its text is dimmed, it
// takes no tap, drag, key, shortcut (Copy included), composition or focus, and
// Enable gives all of that back.
func TestRichEntry_Disable(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()

	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyle{Inline: true, ColorName: theme.ColorNamePrimary}, Text: "hello"},
		&TextSegment{Style: RichTextStyleInline, Text: " world\nsecond line"},
	})
	changes := 0
	e.OnChanged = func() { changes++ }

	// Disable before the first draw: no window, no renderer.
	e.Disable()
	require.True(t, e.Disabled())

	before, after := NewEntry(), NewEntry()
	w := test.NewTempWindow(t, &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{before, e, after}})
	w.Resize(geo.NewSize(400, 300))
	c := w.Canvas()
	r := cache.Renderer(e).(*richEntryRenderer)

	assertLook := func(disabled bool) {
		t.Helper()
		th, v := e.Theme(), bellui.CurrentApp().Settings().ThemeVariant()
		want := []bellui.ThemeColorName{theme.ColorNamePrimary, RichTextStyleInline.ColorName}
		border := th.Color(theme.ColorNameInputBorder, v)
		if disabled {
			want = []bellui.ThemeColorName{theme.ColorNameDisabled, theme.ColorNameDisabled}
			border = th.Color(theme.ColorNameDisabled, v)
		}
		// Read the colours off the text the provider's cached renderer draws, not
		// off the segments it is handed: its rows point at the segments it laid
		// out, which a stale row model would keep undimmed.
		texts := renderedTexts(&e.provider)
		require.NotEmpty(t, texts, "the provider drew no text, so the check would prove nothing")
		for _, txt := range texts {
			name := want[1]
			if txt.Text == "hello" {
				name = want[0]
			}
			assert.Equal(t, th.Color(name, v), txt.Color, "colour of %q", txt.Text)
		}
		assert.Equal(t, border, r.border.StrokeColor, "border colour")
		// The document keeps its own colours: only the drawn copies are dimmed.
		assert.Equal(t, theme.ColorNamePrimary, e.doc.segments()[0].(*TextSegment).Style.ColorName)
	}
	assertLook(true)

	// A position on the text, in canvas coordinates.
	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(e)
	onText := abs.AddXY(30, e.Size().Height/4)

	// A selection made while enabled, to test that Copy refuses it.
	e.Enable()
	e.selectAll()
	require.NotEmpty(t, e.SelectedText())
	e.Disable()
	assert.Empty(t, e.SelectedText(), "a disabled editor reports no selection")
	assert.False(t, e.sel.selecting, "the selection is drawn while disabled")
	e.anchor, e.caret, e.selecting = 0, 0, false

	t.Run("pointer", func(t *testing.T) {
		// The real hit object is the embedded selectable, which forwards to the
		// editor; a mouse press on it used to grab the focus, which the focus
		// manager grants a disabled widget that has SelectedText.
		e.sel.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: geo.NewPos(30, 10)}, Button: desktop.MouseButtonPrimary})
		assert.Nil(t, c.Focused(), "a click focused the disabled editor")

		test.TapCanvas(c, onText)
		test.DoubleTap(e.sel)
		test.LongPress(e.sel)
		test.TapSecondary(e.sel)
		test.Drag(c, onText, 60, 0)
		assert.Nil(t, c.Focused(), "a gesture focused the disabled editor")
		assert.Equal(t, 0, e.caret, "a gesture moved the caret")
		assert.False(t, e.selecting, "a gesture selected text")
		assert.Nil(t, c.Overlays().Top(), "a gesture opened the context menu")

		// RichEntry does not scroll its own content: a scroll over it reaches
		// whatever scrollable is behind it, so there is nothing of the editor's
		// to refuse. Send one anyway to show it changes nothing here.
		test.Scroll(c, onText, 0, 20)
		assert.Equal(t, 0, e.caret)
	})

	t.Run("keys", func(t *testing.T) {
		cb.SetContent("pasted")
		e.TypedRune('x')
		e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
		e.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
		e.KeyDown(&event.KeyEvent{Name: desktop.KeyShiftLeft})
		e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
		e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})
		e.TypedShortcut(&bellui.ShortcutCut{Clipboard: cb})
		e.TypedShortcut(&bellui.ShortcutSelectAll{})
		e.TypedShortcut(&bellui.ShortcutCopy{Clipboard: cb})
		e.Compose(&event.PreeditEvent{Text: "ka", CursorPos: 2})
		e.ComposeCommit("か")
		e.ReplaceAccessibleSelection("ax")
		e.SetAccessibleSelection(0, 3)
		e.PerformAccessibleAction("undo")

		assert.Equal(t, "hello world\nsecond line", e.String(), "input edited the disabled editor")
		assert.Equal(t, 0, changes, "OnChanged fired while disabled")
		assert.Equal(t, 0, e.caret, "a key moved the caret")
		assert.False(t, e.selecting, "a key selected text")
		assert.False(t, e.composing, "the disabled editor took a composition")
		assert.Equal(t, "pasted", cb.Content(), "Copy or Cut reached the clipboard")
		assert.Empty(t, e.AccessibleActions(), "a disabled editor offers accessible edits")
	})

	t.Run("focus", func(t *testing.T) {
		c.Focus(before)
		c.FocusNext()
		assert.Equal(t, after, c.Focused(), "Tab did not skip the disabled editor")

		e.Enable()
		c.Focus(e)
		require.Equal(t, e, c.Focused())
		e.Compose(&event.PreeditEvent{Text: "ka", CursorPos: 2})
		require.True(t, e.composing)
		e.Disable()
		assert.Nil(t, c.Focused(), "Disable left the focus on the editor")
		assert.False(t, e.composing, "Disable left a composition in progress")
		assert.Equal(t, "hello world\nsecond line", e.String(), "the cancelled composition was committed")
	})

	t.Run("enable", func(t *testing.T) {
		e.Enable()
		assertLook(false)

		c.Focus(e)
		require.Equal(t, e, c.Focused(), "an enabled editor takes the focus")
		e.caret, e.anchor = 0, 0
		e.TypedRune('x')
		assert.Equal(t, "xhello world\nsecond line", e.String())
		assert.Equal(t, 1, changes, "typing in the enabled editor fires OnChanged")

		// Disable after the first draw dims it again.
		e.Disable()
		assertLook(true)
	})
}

// renderedTexts walks a widget's cached renderers and returns the text objects
// they draw.
func renderedTexts(o core.CanvasObject) []*canvas.Text {
	var out []*canvas.Text
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		switch x := o.(type) {
		case *canvas.Text:
			if x.Visible() && strings.TrimSpace(x.Text) != "" {
				out = append(out, x)
			}
		case *bellui.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		case core.Widget:
			for _, c := range cache.Renderer(x).Objects() {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

// A disabled editor lays out dimmed copies of its segments. With wrapping on,
// its row model is built from those copies, keyed by identity, and must match
// the enabled one row for row (#1216).
func TestRichEntry_DisableKeepsWrapRows(t *testing.T) {
	test.NewTempApp(t)

	e := NewRichEntry()
	e.Wrapping = bellui.TextWrapWord
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "a long first run that wraps "},
		&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}, Text: "and a bold second run that wraps too"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(140, 300))

	enabled := append([]richRow(nil), e.doc.activeRowModel()...)
	require.Greater(t, len(enabled), 2, "the text did not wrap, so the check would prove nothing")

	e.Disable()
	assert.Equal(t, enabled, e.doc.activeRowModel(), "the disabled row model differs")
	e.Enable()
	assert.Equal(t, enabled, e.doc.activeRowModel())
}
