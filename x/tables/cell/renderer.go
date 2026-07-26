package cell

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// RendererFactory creates and updates cell CanvasObjects for a column type.
// This follows Fyne's CreateCell/UpdateCell callback pattern.
type RendererFactory interface {
	// CreateCell returns a new template CanvasObject for a cell.
	// The object will be recycled via the cell pool.
	CreateCell() core.CanvasObject

	// UpdateCell updates the template with data for a specific cell.
	UpdateCell(cell core.CanvasObject, params Params)
}

// AccessibleValueTexter is an optional interface a RendererFactory may implement
// to supply the plain, untruncated text a screen reader should announce for a
// value — matching the cell's visible formatting (e.g. a NumberRenderer's
// thousands separators, a DateRenderer's layout). DataGrid.AccessibleCellText
// prefers it over the generic fmt.Sprint fallback (see AccessibleText) when the
// column has no explicit ValueFormatter, so AT hears what is shown.
type AccessibleValueTexter interface {
	TextForValue(value any) string
}

// Params provides context to cell renderers and editors.
type Params struct {
	RowIndex   int
	ColIndex   int
	Value      any
	RowData    any
	CellWidth  float32 // available width for content
	IsSelected bool
	IsHovered  bool
	IsFocused  bool
}

// TruncateText truncates text to fit within maxWidth pixels, appending "…"
// if the text is too long. Uses bellui.MeasureText for accurate measurement.
func TruncateText(text string, maxWidth float32, textSize float32, style bellui.TextStyle) string {
	if maxWidth <= 0 || text == "" {
		return text
	}
	measured := bellui.MeasureText(text, textSize, style)
	if measured.Width <= maxWidth {
		return text
	}
	ellipsis := "…"
	ellipsisW := bellui.MeasureText(ellipsis, textSize, style).Width
	available := maxWidth - ellipsisW
	if available <= 0 {
		return ellipsis
	}

	// Binary search for the longest prefix that fits
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		w := bellui.MeasureText(string(runes[:mid]), textSize, style).Width
		if w <= available {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 {
		return ellipsis
	}
	return string(runes[:lo]) + ellipsis
}
