package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

func TestEntry_composedDisplay(t *testing.T) {
	e := NewEntry()
	e.Text = "ab"

	assert.Equal(t, "ab", e.composedDisplay(), "no composition → Text unchanged")

	e.composing = true
	e.preeditText = "XY"
	e.preeditAnchor = 1
	assert.Equal(t, "aXYb", e.composedDisplay(), "preedit spliced at anchor")

	e.preeditAnchor = 0
	assert.Equal(t, "XYab", e.composedDisplay())

	e.preeditAnchor = 2
	assert.Equal(t, "abXY", e.composedDisplay())

	// Out-of-range anchor is clamped.
	e.preeditAnchor = 99
	assert.Equal(t, "abXY", e.composedDisplay())
}

// AccessiblePreedit is the seam the text-input mirror uses to subtract the
// spliced preedit from its pushes so the native IME buffer only ever sees
// committed text (#1057). It must report exactly the span composedDisplay /
// the provider splice occupy within AccessibleText, and clear on commit and
// cancel.
func TestEntry_AccessiblePreedit(t *testing.T) {
	e := NewEntry()
	test.NewTempApp(t)
	w := test.NewTempWindow(t, e)
	w.Canvas().Focus(e)

	e.SetText("ab")
	e.SetAccessibleSelection(1, 1)

	_, _, ok := e.AccessiblePreedit()
	assert.False(t, ok, "no composition → no preedit report")

	e.Compose(&event.PreeditEvent{Text: "XY", CursorPos: 2})
	text, start, ok := e.AccessiblePreedit()
	assert.True(t, ok)
	assert.Equal(t, "XY", text)
	assert.Equal(t, 1, start)
	// The premise the seam relies on: the accessible text contains the preedit
	// at exactly the reported span.
	assert.Equal(t, "aXYb", e.AccessibleText())

	e.ComposeCommit("Z")
	_, _, ok = e.AccessiblePreedit()
	assert.False(t, ok, "commit clears the preedit report")
	assert.Equal(t, "aZb", e.AccessibleText())

	e.Compose(&event.PreeditEvent{Text: "Q", CursorPos: 1})
	_, _, ok = e.AccessiblePreedit()
	assert.True(t, ok)
	e.ComposeCancel()
	_, _, ok = e.AccessiblePreedit()
	assert.False(t, ok, "cancel clears the preedit report")
	assert.Equal(t, "aZb", e.AccessibleText())
}

// AccessibleComposing tracks the composition lifecycle — including the
// desktop-commit TypedRune path, which is what makes an Android hardware-key
// word-finalize stale (#1057).
func TestEntry_AccessibleComposing(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.FocusGained()

	assert.False(t, e.AccessibleComposing())

	e.Compose(&event.PreeditEvent{Text: "worl", CursorPos: 4})
	assert.True(t, e.AccessibleComposing())

	e.TypedRune('d') // native/hardware rune commits the preedit as typed text
	assert.False(t, e.AccessibleComposing(), "desktop-commit path ends the composition")

	e.Compose(&event.PreeditEvent{Text: "x", CursorPos: 1})
	assert.True(t, e.AccessibleComposing())
	e.ComposeCancel()
	assert.False(t, e.AccessibleComposing())

	// Composing with an empty preedit still reports true — a following commit
	// legitimately inserts (finalizing an emptied composition is not stale).
	e.Compose(&event.PreeditEvent{Text: "", CursorPos: 0})
	assert.True(t, e.AccessibleComposing())
}

func TestUnderlineSubrange(t *testing.T) {
	// Single base run "abcde", underline [1,3) → "a" | "bc"(underlined) | "de".
	runs := underlineSubrange([]StyleRun{{Text: "abcde"}}, 1, 3)
	assert.Len(t, runs, 3)
	assert.Equal(t, "a", runs[0].Text)
	assert.False(t, runs[0].Underline)
	assert.Equal(t, "bc", runs[1].Text)
	assert.True(t, runs[1].Underline)
	assert.Equal(t, UnderlineSolid, runs[1].UnderlineStyle)
	assert.Equal(t, "de", runs[2].Text)
	assert.False(t, runs[2].Underline)

	// Whole run underlined → unchanged single run, now underlined.
	runs = underlineSubrange([]StyleRun{{Text: "abc"}}, 0, 3)
	assert.Len(t, runs, 1)
	assert.True(t, runs[0].Underline)

	// Underline spanning two base runs preserves their colors.
	runs = underlineSubrange([]StyleRun{{Text: "ab", ColorName: "red"}, {Text: "cd", ColorName: "blue"}}, 1, 3)
	// "a" | "b"(ul, red) | "c"(ul, blue) | "d"
	assert.Len(t, runs, 4)
	assert.Equal(t, "a", runs[0].Text)
	assert.True(t, runs[1].Underline)
	assert.Equal(t, "red", string(runs[1].ColorName))
	assert.True(t, runs[2].Underline)
	assert.Equal(t, "blue", string(runs[2].ColorName))
	assert.False(t, runs[3].Underline)
}

func TestEntry_styleRunsForRow_Underline(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.Text = "hello"
	e.FocusGained()
	e.sel.selecting = false // drop single-line select-all
	e.CursorRow, e.CursorColumn = 0, 0
	e.Compose(&event.PreeditEvent{Text: "ni", CursorPos: 2})
	// Display row is now "nihello" with preedit "ni" at [0,2).

	runs := e.styleRunsForRow(0)
	assert.NotEmpty(t, runs)

	// Runs must tile the display row exactly and underline only the preedit.
	var total string
	var underlined string
	for _, r := range runs {
		total += r.Text
		if r.Underline {
			underlined += r.Text
		}
	}
	assert.Equal(t, "nihello", total, "runs tile the whole display row")
	assert.Equal(t, "ni", underlined, "only the preedit is underlined")
}

func TestEntry_styleRunsForRow_NotComposing(t *testing.T) {
	e := NewEntry()
	e.Text = "hello"
	// Not composing and no LineHighlighter → no synthesized runs.
	assert.Nil(t, e.styleRunsForRow(0))
}

// TestEntry_DesktopCommit_ClearsComposition covers #354: on the desktop driver
// the committed text arrives via TypedRune while composing. It must finalize the
// composition (drop the preedit, clear `composing`) so the visible text is the
// committed string — not the preedit spliced alongside it, which is the あ→ああ
// doubling users saw. composedDisplay() is the visible text, so it has teeth.
func TestEntry_DesktopCommit_ClearsComposition(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.FocusGained()

	e.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})
	assert.True(t, e.composing)
	assert.Equal(t, "あ", e.composedDisplay())

	e.TypedRune('あ') // IME commit delivered as a char event while composing

	assert.False(t, e.composing, "composition must end on commit")
	assert.Equal(t, "", e.preeditText)
	assert.Equal(t, "あ", e.Text)
	assert.Equal(t, "あ", e.composedDisplay(), "no lingering preedit → no ああ doubling (#354)")
}

// TestEntry_EscapeClearsComposition: Esc discards an active composition (#354).
func TestEntry_EscapeClearsComposition(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.FocusGained()
	e.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})

	e.TypedKey(&event.KeyEvent{Name: event.KeyEscape})

	assert.False(t, e.composing, "Esc cancels composition")
	assert.Equal(t, "", e.Text)
	assert.Equal(t, "", e.composedDisplay())
}

// TestEntry_EscapeCompositionPrecedesOnEscape: while composing, Esc discards
// the composition only — OnEscape fires on the next, composition-free Esc, so
// rejecting an IME suggestion cannot dismiss the surrounding inline editor
// (#1052).
func TestEntry_EscapeCompositionPrecedesOnEscape(t *testing.T) {
	test.NewTempApp(t)
	escaped := 0
	e := NewEntry()
	e.OnEscape = func() { escaped++ }
	e.FocusGained()
	e.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})

	e.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, e.composing, "first Esc cancels the composition")
	assert.Equal(t, 0, escaped, "the composition-cancelling Esc must not reach OnEscape")

	e.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.Equal(t, 1, escaped, "the next Esc reaches OnEscape")
}

// TestEntry_FocusLostClearsComposition: focus-out discards an active composition (#354).
func TestEntry_FocusLostClearsComposition(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	e.FocusGained()
	e.Compose(&event.PreeditEvent{Text: "あ", CursorPos: 1})

	e.FocusLost()

	assert.False(t, e.composing, "focus-out cancels composition")
	assert.Equal(t, "", e.Text)
}
