package tables

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// ShowFilterPopup shows a filter popup for the given column at the given position.
func (g *DataGrid) ShowFilterPopup(col int, pos geo.Position) {
	rc := g.visibleCol(col)
	if rc == nil {
		return
	}
	if !rc.def.isFilterable(g.DefaultColDef) {
		return
	}

	colID := rc.def.ID()

	c := bellui.CurrentApp().Driver().CanvasForObject(g)
	if c == nil {
		return
	}

	entry := widget.NewEntry()
	entry.PlaceHolder = "Filter " + rc.def.Header() + "..."

	clearBtn := widget.NewButton("Clear", nil)
	applyBtn := widget.NewButton("Apply", nil)

	content := container.NewVBox(
		widget.NewLabel("Filter: "+rc.def.Header()),
		entry,
		container.NewHBox(clearBtn, applyBtn),
	)

	popup := widget.NewPopUp(content, c)

	clearBtn.OnTapped = func() {
		g.setColumnFilter(colID, nil)
		popup.Hide()
	}

	applyBtn.OnTapped = func() {
		text := entry.Text
		if text == "" {
			g.setColumnFilter(colID, nil)
		} else {
			lower := toLower(text)
			g.setColumnFilter(colID, func(v any) bool {
				return containsLower(formatForFilter(v), lower)
			})
		}
		popup.Hide()
	}

	entry.OnSubmitted = func(_ string) {
		applyBtn.OnTapped()
	}

	popup.ShowAtPosition(pos)
}

func formatForFilter(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
