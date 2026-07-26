package cell

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/theme"
)

// DateRenderer displays date/time values formatted as canvas.Text.
type DateRenderer struct {
	// Format is a Go time format string. Defaults to "2006-01-02".
	Format string
}

// NewDateRenderer creates a DateRenderer.
func NewDateRenderer() *DateRenderer {
	return &DateRenderer{Format: "2006-01-02"}
}

// CreateCell returns a canvas.Text for displaying formatted dates.
func (r *DateRenderer) CreateCell() core.CanvasObject {
	t := canvas.NewText("", theme.Color(theme.ColorNameForeground))
	t.TextSize = theme.TextSize()
	return t
}

// UpdateCell formats the date value and sets the text.
func (r *DateRenderer) UpdateCell(obj core.CanvasObject, params Params) {
	t := obj.(*canvas.Text)
	ts := theme.TextSize()
	text := r.formatDate(params.Value)
	if params.CellWidth > 0 {
		text = TruncateText(text, params.CellWidth, ts, bellui.TextStyle{})
	}
	t.Text = text
	t.Alignment = bellui.TextAlignLeading
	t.Color = theme.Color(theme.ColorNameForeground)
	t.TextSize = ts
	t.Refresh()
}

// TextForValue returns the formatted date a screen reader should announce — the
// same text UpdateCell shows, without width truncation (AccessibleValueTexter).
func (r *DateRenderer) TextForValue(v any) string {
	return r.formatDate(v)
}

func (r *DateRenderer) formatDate(v any) string {
	if v == nil {
		return ""
	}
	format := r.Format
	if format == "" {
		format = "2006-01-02"
	}
	switch val := v.(type) {
	case time.Time:
		return val.Format(format)
	case *time.Time:
		if val == nil {
			return ""
		}
		return val.Format(format)
	case string:
		return val
	}
	return ""
}
