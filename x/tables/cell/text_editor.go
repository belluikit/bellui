package cell

import (
	"fmt"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// TextEditor is a cell editor that uses a text Entry widget.
type TextEditor struct {
	cancelled bool
}

// NewTextEditor creates a text editor factory.
func NewTextEditor() *TextEditor {
	return &TextEditor{}
}

func (e *TextEditor) CreateEditor() core.CanvasObject {
	entry := widget.NewEntry()
	return entry
}

func (e *TextEditor) BeginEdit(editor core.CanvasObject, value any, params Params) {
	entry := editor.(*widget.Entry)
	entry.SetText(fmt.Sprint(value))
	e.cancelled = false
}

func (e *TextEditor) EndEdit(editor core.CanvasObject) (any, bool) {
	entry := editor.(*widget.Entry)
	return entry.Text, e.cancelled
}

// Cancel marks the editor as cancelled.
func (e *TextEditor) Cancel() {
	e.cancelled = true
}
