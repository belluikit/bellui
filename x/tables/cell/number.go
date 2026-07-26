package cell

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/theme"
)

// NumberRenderer displays numeric values right-aligned using canvas.Text.
type NumberRenderer struct {
	// Format is a fmt format string (e.g. "%.2f"). Defaults to "%v".
	Format string
}

// NewNumberRenderer creates a NumberRenderer.
func NewNumberRenderer() *NumberRenderer {
	return &NumberRenderer{}
}

// CreateCell returns a right-aligned canvas.Text.
func (r *NumberRenderer) CreateCell() core.CanvasObject {
	t := canvas.NewText("", theme.Color(theme.ColorNameForeground))
	t.Alignment = bellui.TextAlignTrailing
	t.TextSize = theme.TextSize()
	return t
}

// UpdateCell sets the text formatted as a number.
// Alignment is set every time because cells may be recycled across column types.
func (r *NumberRenderer) UpdateCell(obj core.CanvasObject, params Params) {
	t := obj.(*canvas.Text)
	ts := theme.TextSize()
	text := r.formatNumber(params.Value)
	if params.CellWidth > 0 {
		text = TruncateText(text, params.CellWidth, ts, bellui.TextStyle{})
	}
	t.Text = text
	t.Alignment = bellui.TextAlignTrailing
	t.Color = theme.Color(theme.ColorNameForeground)
	t.TextSize = ts
	t.Refresh()
}

// TextForValue returns the formatted number a screen reader should announce —
// the same text UpdateCell shows, without width truncation (AccessibleValueTexter).
func (r *NumberRenderer) TextForValue(v any) string {
	return r.formatNumber(v)
}

func (r *NumberRenderer) formatNumber(v any) string {
	if v == nil {
		return ""
	}
	if r.Format != "" {
		return fmt.Sprintf(r.Format, v)
	}
	return fmt.Sprint(v)
}
