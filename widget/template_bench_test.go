package widget_test

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// BenchmarkCollectionRefresh measures a Refresh of each collection widget
// that measures an item template, and reports how many renderers are cached
// once the benchmark has run: a template that keeps its renderer adds one per
// Refresh (#722).
func BenchmarkCollectionRefresh(b *testing.B) {
	label := func() core.CanvasObject { return widget.NewLabel("template") }
	set := func(id int, o core.CanvasObject) { o.(*widget.Label).SetText(fmt.Sprint(id)) }
	for name, build := range map[string]func() core.Widget{
		"List": func() core.Widget {
			return widget.NewList(func() int { return 1000 }, label,
				func(id widget.ListItemID, o core.CanvasObject) { set(id, o) })
		},
		"Table": func() core.Widget {
			return widget.NewTableWithHeaders(func() (int, int) { return 1000, 10 }, label,
				func(id widget.TableCellID, o core.CanvasObject) { set(id.Row, o) })
		},
	} {
		b.Run(name, func(b *testing.B) {
			a := test.NewApp()
			defer a.Quit()
			w := build()
			c := test.NewCanvas()
			c.SetContent(w)
			c.Resize(geo.NewSize(400, 400))
			b.ReportAllocs()
			for b.Loop() {
				w.Refresh()
			}
			b.ReportMetric(float64(len(cache.RenderedWidgets())), "renderers")
			for w := range cache.RenderedWidgets() {
				cache.DestroyRenderer(w)
			}
		})
	}
}
