//go:build !wasm

package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/internal/driver/mobile/event/textedit"
	"github.com/belluikit/bellui/internal/textinput"
	"github.com/belluikit/bellui/widget"
)

// keyRecorder is a focusable widget that takes keys but is not a text editor,
// like the chart while its Text tool waits for a label.
type keyRecorder struct {
	widget.BaseWidget
	keys  []event.KeyName
	runes []rune
}

func newKeyRecorder() *keyRecorder {
	r := &keyRecorder{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *keyRecorder) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(""))
}
func (r *keyRecorder) FocusGained()                {}
func (r *keyRecorder) FocusLost()                  {}
func (r *keyRecorder) TypedRune(ch rune)           { r.runes = append(r.runes, ch) }
func (r *keyRecorder) TypedKey(ev *event.KeyEvent) { r.keys = append(r.keys, ev.Name) }

// The Android soft keyboard's Return commits "\n" as text on the default
// keyboard. With no editor bound, it reaches the focused widget as a Return
// key, so a widget that finishes on Return sees it (#666).
func TestHandleTextEdit_NewlineReachesNonEditorAsReturn(t *testing.T) {
	d := &driver{textInput: textinput.NewSession(keyboardStateBackend{})}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	rec := newKeyRecorder()
	c.SetContent(rec)
	c.Focus(rec)
	d.textInput.Bind(c.Focused())
	require.False(t, d.textInput.Bound(), "a widget that is not an editor binds nothing")

	d.handleTextEdit(c, textedit.Event{Start: 0, End: 0, Text: "\n"})
	assert.Equal(t, []event.KeyName{event.KeyReturn}, rec.keys)

	// Other committed text is not forwarded: such a widget gets its typing as
	// key events, so forwarding it too would type it twice.
	rec.keys, rec.runes = nil, nil
	d.handleTextEdit(c, textedit.Event{Start: 0, End: 0, Text: "hi"})
	d.handleTextEdit(c, textedit.Event{Start: 0, End: 1, Selection: true})
	assert.Empty(t, rec.keys)
	assert.Empty(t, rec.runes)
}

// A bound editor takes the committed newline as text, once: it is not also
// sent as a Return key, which would insert a second one.
func TestHandleTextEdit_NewlineInBoundEditorIsText(t *testing.T) {
	d := &driver{textInput: textinput.NewSession(keyboardStateBackend{})}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	entry := widget.NewMultiLineEntry()
	entry.SetText("ab")
	c.SetContent(entry)
	c.Focus(entry)
	require.True(t, d.textInput.Bind(c.Focused()))

	d.handleTextEdit(c, textedit.Event{Start: 1, End: 1, Text: "\n"})
	assert.Equal(t, "a\nb", entry.Text)
}

// iOS's keyboard deletes by selecting the character before the caret and
// replacing it with nothing. With no editor bound, that reaches the focused
// widget as one Backspace per deleted rune; a replacement that inserts text
// (an autocorrection) still reaches nothing (#666).
func TestHandleTextEdit_DeletionReachesNonEditorAsBackspace(t *testing.T) {
	d := &driver{textInput: textinput.NewSession(keyboardStateBackend{})}
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	rec := newKeyRecorder()
	c.SetContent(rec)
	c.Focus(rec)
	d.textInput.Bind(c.Focused())
	require.False(t, d.textInput.Bound())

	d.handleTextEdit(c, textedit.Event{Start: 3, End: 4})
	assert.Equal(t, []event.KeyName{event.KeyBackspace}, rec.keys)

	rec.keys = nil
	d.handleTextEdit(c, textedit.Event{Start: 2, End: 4})
	assert.Equal(t, []event.KeyName{event.KeyBackspace, event.KeyBackspace}, rec.keys)

	rec.keys = nil
	d.handleTextEdit(c, textedit.Event{Start: 0, End: 4, Text: "popô"})
	assert.Empty(t, rec.keys)
	assert.Empty(t, rec.runes)
}
