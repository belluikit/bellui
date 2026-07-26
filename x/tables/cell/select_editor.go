package cell

import (
	"fmt"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// SelectEditor is a cell editor that uses a dropdown Select widget.
type SelectEditor struct {
	Options []string
}

// NewSelectEditor creates a select editor factory with the given options.
func NewSelectEditor(options []string) *SelectEditor {
	return &SelectEditor{Options: options}
}

func (e *SelectEditor) CreateEditor() core.CanvasObject {
	return widget.NewSelect(e.Options, nil)
}

func (e *SelectEditor) BeginEdit(editor core.CanvasObject, value any, params Params) {
	sel := editor.(*widget.Select)
	sel.SetSelected(fmt.Sprint(value))
}

func (e *SelectEditor) EndEdit(editor core.CanvasObject) (any, bool) {
	sel := editor.(*widget.Select)
	return sel.Selected, false
}
