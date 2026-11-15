package editor

import (
	"context"
	"image/color"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// redHighlighter colours every line red, so the test can tell syntax colours
// from the disabled colour.
type redHighlighter struct{}

var red = color.NRGBA{R: 0xff, A: 0xff}

func (redHighlighter) Highlight(line string, state int) ([]StyleRun, int) {
	return []StyleRun{{Text: line, ForegroundColor: red}}, state
}

// countingHover counts hover requests, which a disabled editor must not make.
type countingHover struct{ calls int }

func (h *countingHover) Hover(context.Context, int, int) (*HoverContent, error) {
	h.calls++
	return &HoverContent{PlainText: "doc"}, nil
}

// editorTexts walks the editor's cached renderers and returns the text objects
// they draw.
func editorTexts(t *testing.T, o core.CanvasObject) []*canvas.Text {
	var out []*canvas.Text
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		if !o.Visible() {
			return
		}
		switch x := o.(type) {
		case *canvas.Text:
			if strings.TrimSpace(x.Text) != "" {
				out = append(out, x)
			}
		case *bellui.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		case core.Widget:
			for _, c := range test.TempWidgetRenderer(t, x).Objects() {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

// A disabled Editor is visible but inert (#1216): its text is dimmed, and no
// tap, drag, scroll, hover, mouse press, key, shortcut (Copy included) or Tab
// reaches it or its find bar. Enable gives it all back, and read-only stays a
// separate state.
func TestEditor_Disable(t *testing.T) {
	test.NewTempApp(t)
	cb := bellui.CurrentApp().Clipboard()

	ed := NewEditor()
	ed.MinRows = 5
	ed.Highlighter = redHighlighter{}
	hover := &countingHover{}
	ed.SetHoverProvider(hover)
	ed.SetText(strings.Repeat("hello world\n", 60) + "last")
	changes, saves, runs := 0, 0, 0
	ed.OnChanged = func(string) { changes++ }
	ed.OnSave = func() { saves++ }
	ed.OnRun = func() { runs++ }

	// Disable before the first draw: no window, no renderer.
	ed.Disable()
	require.True(t, ed.Disabled())

	before, after := widget.NewEntry(), widget.NewEntry()
	w := test.NewTempWindow(t, container.NewBorder(before, after, nil, nil, ed))
	w.Resize(geo.NewSize(500, 400))
	c := w.Canvas()

	th, v := ed.Theme(), bellui.CurrentApp().Settings().ThemeVariant()
	assertLook := func(t *testing.T, disabled bool) {
		t.Helper()
		want := color.Color(red)
		if disabled {
			want = th.Color(theme.ColorNameDisabled, v)
		}
		drawn := 0
		for _, txt := range editorTexts(t, ed.entry) {
			if !strings.Contains(txt.Text, "hello") {
				continue // line numbers and the like are not the document's text
			}
			if _, _, _, a := txt.Color.RGBA(); a == 0 {
				// With a highlighter the Entry keeps its plain text as an
				// invisible layer and draws each run over it.
				continue
			}
			drawn++
			assert.Equal(t, want, txt.Color, "colour of %q", txt.Text)
		}
		require.NotZero(t, drawn, "the editor drew no document text, so the check would prove nothing")
		assert.Equal(t, disabled, ed.shield.Visible(), "the pointer shield")
	}
	assertLook(t, true)

	abs := bellui.CurrentApp().Driver().AbsolutePositionForObject(ed.entry)
	onText := abs.AddXY(40, 12)

	// A selection made while enabled, to test that Copy refuses it.
	ed.Enable()
	ed.entry.TypedShortcut(&bellui.ShortcutSelectAll{})
	require.NotEmpty(t, ed.SelectedText())
	ed.Disable()
	assert.Empty(t, ed.SelectedText(), "a disabled editor reports a selection")
	assert.False(t, ed.entry.KeepSelectionVisible, "the selection is drawn while disabled")
	ed.SetCursorPos(0, 0)

	t.Run("pointer", func(t *testing.T) {
		// What a real mouse press reaches: the topmost Mouseable under it. It
		// used to be the Entry's selection layer, which grabs the focus, and
		// the focus manager grants it to a disabled Entry.
		o, _, ok := driver.ObjectAtPositionMatching(c, onText, func(o core.CanvasObject) bool {
			_, is := o.(desktop.Mouseable)
			return is
		})
		require.True(t, ok)
		assert.Same(t, ed.shield, o, "a mouse press reaches the disabled editor's parts")
		o.(desktop.Mouseable).MouseDown(&desktop.MouseEvent{Button: desktop.MouseButtonPrimary})

		test.TapCanvas(c, onText)
		test.Drag(c, onText, 80, 30)
		test.MoveMouse(c, onText)
		before := ed.entry.Entry.SelectedText() // what the Entry itself holds

		scrolled := ed.scrollY
		test.Scroll(c, onText, 0, -200)

		assert.Nil(t, c.Focused(), "a gesture focused the disabled editor")
		row, col := ed.CursorPos()
		assert.Equal(t, [2]int{0, 0}, [2]int{row, col}, "a gesture moved the cursor")
		assert.Empty(t, before, "a drag selected text")
		assert.Equal(t, scrolled, ed.scrollY, "a scroll scrolled the disabled editor")
		assert.False(t, ed.popoverVisible(), "a hover opened a card")
		assert.Equal(t, 0, hover.calls, "a hover asked the provider")
		assert.Nil(t, ed.hover.timer, "a hover armed the dwell timer")
	})

	t.Run("keys", func(t *testing.T) {
		cb.SetContent("clip")
		ed.entry.TypedRune('x')
		ed.entry.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
		ed.entry.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
		ed.entry.TypedKey(&event.KeyEvent{Name: event.KeyTab})
		ed.entry.TypedKey(&event.KeyEvent{Name: event.KeyF12})
		ed.entry.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})
		ed.entry.TypedShortcut(&bellui.ShortcutCut{Clipboard: cb})
		ed.entry.TypedShortcut(&bellui.ShortcutCopy{Clipboard: cb})
		ed.entry.TypedShortcut(&desktop.CustomShortcut{KeyName: event.KeyS, Modifier: event.KeyModifierShortcutDefault})
		ed.entry.TypedShortcut(&desktop.CustomShortcut{KeyName: event.KeyR, Modifier: event.KeyModifierShortcutDefault})
		ed.entry.Compose(&event.PreeditEvent{Text: "ka", CursorPos: 2})
		ed.entry.ComposeCommit("か")
		ed.entry.SetAccessibleSelection(0, 5)

		assert.Equal(t, strings.Repeat("hello world\n", 60)+"last", ed.Text(), "input edited the disabled editor")
		assert.Equal(t, [3]int{0, 0, 0}, [3]int{changes, saves, runs}, "OnChanged, OnSave or OnRun fired")
		assert.Equal(t, "clip", cb.Content(), "Copy or Cut reached the clipboard")
		assert.Empty(t, ed.entry.Entry.SelectedText(), "assistive technology selected text")
	})

	t.Run("focus", func(t *testing.T) {
		ed.ShowFind() // shown, and disabled with the editor
		c.Focus(before)
		c.FocusNext()
		assert.Equal(t, after, c.Focused(), "Tab did not skip the disabled editor and its find bar")
		ed.Focus()
		assert.Equal(t, after, c.Focused(), "Focus focused the disabled editor")
		// Directly, as code or assistive tech would: the Entry inside the
		// editor answers SelectableWhenDisabled false (belluikit/bellui#1251).
		c.Focus(ed.entry)
		assert.Equal(t, after, c.Focused(), "the disabled editor's entry took the focus from code")

		ed.Enable()
		c.Focus(ed.find.entry)
		require.Equal(t, ed.find.entry, c.Focused())
		ed.Disable()
		assert.Nil(t, c.Focused(), "Disable left the focus in the find bar")

		ed.Enable()
		ed.Focus()
		require.Equal(t, ed.entry, c.Focused())
		ed.Disable()
		assert.Nil(t, c.Focused(), "Disable left the focus on the editor")
		ed.HideFind()
		assert.Nil(t, c.Focused(), "HideFind focused the disabled editor")
	})

	t.Run("read-only", func(t *testing.T) {
		// Read-only set while disabled holds once enabled: select and copy, no
		// edits.
		ed.SetReadOnly(true)
		ed.Enable()
		ed.Focus()
		ed.entry.TypedShortcut(&bellui.ShortcutSelectAll{})
		ed.entry.TypedShortcut(&bellui.ShortcutCopy{Clipboard: cb})
		assert.Equal(t, ed.Text(), cb.Content(), "a read-only editor copies")
		ed.entry.TypedRune('x')
		assert.Equal(t, 0, changes, "a read-only editor was edited")
		ed.SetReadOnly(false)
	})

	t.Run("enable", func(t *testing.T) {
		assertLook(t, false)
		assert.True(t, ed.entry.KeepSelectionVisible)

		ed.Focus()
		require.Equal(t, ed.entry, c.Focused())
		ed.SetCursorPos(0, 0)
		ed.entry.TypedRune('x')
		assert.Equal(t, 1, changes, "typing in the enabled editor fires OnChanged")
		assert.True(t, strings.HasPrefix(ed.Text(), "x"), "typing in the enabled editor")

		// Disable after the first draw dims it again.
		ed.Disable()
		assertLook(t, true)
	})
}
