package cell

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/widget"
)

// Entry is the text field the text and date editors create: a widget.Entry
// that also reports losing keyboard focus, which is how the grid ends an edit
// when the user clicks outside it (belluikit/bellui#1522). An app's own
// EditorFactory can return one to get the same behaviour.
type Entry struct {
	widget.Entry

	// OnFocusLost, if set, runs after the entry loses keyboard focus. It also
	// runs when the entry's window loses focus, with the entry still the
	// canvas's focused object.
	OnFocusLost func()
}

// NewEntry returns an empty single-line Entry, set up as widget.NewEntry sets
// one up.
func NewEntry() *Entry {
	e := &Entry{}
	e.Wrapping = bellui.TextWrap(bellui.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

// FocusLost ends the entry's focus, then runs OnFocusLost.
func (e *Entry) FocusLost() {
	e.Entry.FocusLost()
	if e.OnFocusLost != nil {
		e.OnFocusLost()
	}
}
