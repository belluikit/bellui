package render

import (
	"fmt"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// StatusBarAccess is the interface the StatusBar needs from the DataGrid.
type StatusBarAccess interface {
	RowCount() int
	TotalRowCount() int
	SelectedRowCount() int
	CurrentPage() int
	PageCount() int
	IsPaginated() bool
}

// StatusBar is a bottom bar showing row count, selection, and pagination info.
type StatusBar struct {
	widget.BaseWidget

	Grid StatusBarAccess
}

// NewStatusBar creates a new status bar.
func NewStatusBar(grid StatusBarAccess) *StatusBar {
	sb := &StatusBar{Grid: grid}
	sb.ExtendBaseWidget(sb)
	return sb
}

func (sb *StatusBar) CreateRenderer() core.WidgetRenderer {
	sb.ExtendBaseWidget(sb)
	r := &statusBarRenderer{
		bar:   sb,
		label: canvas.NewText("", nil),
		bg:    canvas.NewRectangle(nil),
	}
	// The bar draws from its first frame: the grid lays it out but refreshes it
	// only from its own Refresh, so a renderer that waited for a Refresh drew
	// nothing until then (#1500).
	r.objects = []core.CanvasObject{r.bg, r.label}
	r.update()
	return r
}

type statusBarRenderer struct {
	bar     *StatusBar
	label   *canvas.Text
	bg      *canvas.Rectangle
	objects []core.CanvasObject
}

func (r *statusBarRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	r.label.Move(geo.NewPos(8, 2))
	r.label.Resize(geo.NewSize(size.Width-16, size.Height-4))
}

func (r *statusBarRenderer) MinSize() geo.Size {
	return geo.NewSize(100, 24)
}

func (r *statusBarRenderer) Refresh() {
	r.update()
}

// update sets the colours and the text from the grid's current state.
func (r *statusBarRenderer) update() {
	tc := resolveColors(r.bar.Grid)
	r.bg.FillColor = tc.HeaderBg
	r.label.Color = tc.Foreground

	grid := r.bar.Grid
	text := fmt.Sprintf("%d rows", grid.RowCount())

	total := grid.TotalRowCount()
	if total != grid.RowCount() {
		text = fmt.Sprintf("%d of %d rows", grid.RowCount(), total)
	}

	selected := grid.SelectedRowCount()
	if selected > 0 {
		text += fmt.Sprintf("  |  %d selected", selected)
	}

	if grid.IsPaginated() {
		text += fmt.Sprintf("  |  Page %d of %d", grid.CurrentPage()+1, grid.PageCount())
	}

	r.label.Text = text
	r.label.TextSize = 11
}

func (r *statusBarRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *statusBarRenderer) Destroy() {}
