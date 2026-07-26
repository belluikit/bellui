package cell

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// CheckboxEditor is a cell editor for boolean values.
type CheckboxEditor struct{}

// NewCheckboxEditor creates a checkbox editor factory.
func NewCheckboxEditor() *CheckboxEditor {
	return &CheckboxEditor{}
}

func (e *CheckboxEditor) CreateEditor() core.CanvasObject {
	return widget.NewCheck("", nil)
}

func (e *CheckboxEditor) BeginEdit(editor core.CanvasObject, value any, params Params) {
	check := editor.(*widget.Check)
	b, _ := value.(bool)
	check.SetChecked(b)
}

func (e *CheckboxEditor) EndEdit(editor core.CanvasObject) (any, bool) {
	check := editor.(*widget.Check)
	return check.Checked, false
}
