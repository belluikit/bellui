package cell

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// BooleanRenderer displays boolean values as a disabled checkbox.
type BooleanRenderer struct{}

// NewBooleanRenderer creates a BooleanRenderer.
func NewBooleanRenderer() *BooleanRenderer {
	return &BooleanRenderer{}
}

// CreateCell returns a disabled Check widget.
func (r *BooleanRenderer) CreateCell() core.CanvasObject {
	c := widget.NewCheck("", nil)
	c.Disable()
	return c
}

// UpdateCell sets the checkbox checked state.
func (r *BooleanRenderer) UpdateCell(obj core.CanvasObject, params Params) {
	check := obj.(*widget.Check)
	b, _ := params.Value.(bool)
	check.SetChecked(b)
}
