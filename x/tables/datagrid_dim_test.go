package tables

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables/cell"
)

// labelCells is an app's CellRenderer: a widget.Label per cell.
type labelCells struct{}

func (labelCells) CreateCell() core.CanvasObject { return widget.NewLabel("") }
func (labelCells) UpdateCell(o core.CanvasObject, p cell.Params) {
	o.(*widget.Label).SetText(fmt.Sprint(p.Value))
}

// brightest is the highest luminance in r of img.
func brightest(img image.Image, r image.Rectangle) float64 {
	best := 0.0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			best = max(best, float64(g.Y)/255)
		}
	}
	return best
}

// A disabled grid draws its header and built-in cells in the disabled colour
// itself, and an app's CellRenderer cells are dimmed by the paint walk
// (belluikit/bellui#1393): all three end up about as bright. The grid's own
// text must not be dimmed a second time, which the header under its Clip and
// the built-in cells were until the grid's parts said they draw their own
// look. Meaningful against a toolkit with #1393, as CI builds this module.
func TestDataGrid_DisabledDimsOnce(t *testing.T) {
	g := newTestGrid(t, manyRows(5), []*ColumnDef{
		{ColID: "n", Field: "n", HeaderName: "Builtin", Width: 150},
		{ColID: "a", Field: "n", HeaderName: "App", Width: 150, CellRenderer: labelCells{}},
	})
	test.ApplyTheme(t, theme.DefaultTheme())
	w := test.NewTempWindow(t, g)
	w.Resize(geo.NewSize(400, 260))
	g.Disable()
	img := w.Canvas().Capture()
	scale := float64(img.Bounds().Dx()) / float64(w.Canvas().Size().Width)
	at := func(x, y, wd, h float64) image.Rectangle {
		return image.Rect(int(x*scale), int(y*scale), int((x+wd)*scale), int((y+h)*scale))
	}
	headerH := float64(g.HeaderHeight())
	rowY := headerH + float64(g.rowHeight())/2 - 8
	header := brightest(img, at(4, headerH/2-8, 100, 16))
	builtin := brightest(img, at(4, rowY, 100, 16))
	app := brightest(img, at(154, rowY, 100, 16))
	t.Logf("brightest: header %.3f, built-in %.3f, app %.3f", header, builtin, app)
	assert.Greater(t, app, 0.2, "the app cell is drawn")
	assert.InDelta(t, app, builtin, 0.1, "the built-in cell is dimmed once, as the app cell is")
	assert.InDelta(t, app, header, 0.1, "the header title is dimmed once")
}
