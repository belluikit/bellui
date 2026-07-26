package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// These exercise the Entry integration by driving the popup directly rather than
// through canvas focus: the headless test driver does not route focus to overlay
// widgets the way a live driver does, so c.Focused() is unreliable here.

func TestEntry_ShowCompletion_AcceptViaKeyboard(t *testing.T) {
	test.NewTempApp(t)
	entry := NewEntry()
	entry.SetText("fmt.")
	entry.CursorColumn = 4
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(320, 200))
	c := w.Canvas()

	var got CompletionItem
	entry.OnCompletionSelected = func(it CompletionItem) { got = it }

	entry.ShowCompletion("Completions", []CompletionItem{
		{Text: "fmt.Println", Detail: "func(a ...any)"},
		{Text: "fmt.Printf"},
	})
	require.NotNil(t, entry.completion)
	require.Len(t, c.Overlays().List(), 1)

	entry.completion.TypedKey(&event.KeyEvent{Name: event.KeyDown})   // second item
	entry.completion.TypedKey(&event.KeyEvent{Name: event.KeyReturn}) // accept

	assert.Equal(t, "fmt.Printf", got.Text)
	assert.Empty(t, c.Overlays().List(), "accepting should dismiss the popup")
}

func TestEntry_Completion_PopupForwardsEditingToEntry(t *testing.T) {
	test.NewTempApp(t)
	entry := NewMultiLineEntry()
	entry.SetText("fmt")
	entry.CursorRow, entry.CursorColumn = 0, 3
	w := test.NewTempWindow(t, entry)
	w.Resize(geo.NewSize(320, 200))

	var got CompletionItem
	entry.OnCompletionSelected = func(it CompletionItem) { got = it }
	entry.ShowCompletion("", []CompletionItem{{Text: "fmt.Println"}, {Text: "fmt.Printf"}})
	require.NotNil(t, entry.completion)
	require.True(t, entry.completion.Visible())
	require.Equal(t, entry, entry.completion.host, "popup is wired to forward to the Entry")

	// On a live driver the popup is the focused overlay; it forwards typing and
	// editing keys back to the host Entry. Drive it directly here — the headless
	// test driver does not route focus to overlay widgets reliably.
	entry.completion.TypedRune('.')
	assert.Equal(t, "fmt.", entry.Text, "typed rune is forwarded to the Entry")
	entry.completion.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, "fmt", entry.Text, "editing keys are forwarded to the Entry")

	// Navigation/accept are handled by the popup itself, not forwarded.
	entry.completion.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 1, entry.completion.selected)
	entry.completion.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, "fmt.Printf", got.Text)
	assert.False(t, entry.completion.Visible(), "accepting closes the popup")
}

func TestEntry_Completion_EscapeCloses(t *testing.T) {
	test.NewTempApp(t)
	entry := NewEntry()
	w := test.NewTempWindow(t, entry)
	c := w.Canvas()

	entry.ShowCompletion("", []CompletionItem{{Text: "a"}, {Text: "b"}})
	require.True(t, entry.completion.Visible())

	entry.completion.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.False(t, entry.completion.Visible())
	assert.Empty(t, c.Overlays().List())
}

func TestEntry_ShowCompletion_EmptyHides(t *testing.T) {
	test.NewTempApp(t)
	entry := NewEntry()
	w := test.NewTempWindow(t, entry)
	c := w.Canvas()

	entry.ShowCompletion("", []CompletionItem{{Text: "a"}})
	require.Len(t, c.Overlays().List(), 1)

	// Re-showing with no items hides the open popup (e.g. filter matched nothing).
	entry.ShowCompletion("", nil)
	assert.Empty(t, c.Overlays().List())
}

func TestEntry_HideCompletion_NilSafe(t *testing.T) {
	test.NewTempApp(t)
	entry := NewEntry()
	test.NewTempWindow(t, entry)
	assert.NotPanics(t, func() { entry.HideCompletion() }) // never shown
}
