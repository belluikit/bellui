package container_test

import (
	"strconv"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// BenchmarkDirectionScope_AddRemoveRows adds 500 rows one at a time to a box
// inside a right-to-left scope, then removes them one at a time, as a handler
// filling and clearing a list does: each Add and Remove lays the box out
// outside a walk, in the scope's direction (#749).
func BenchmarkDirectionScope_AddRemoveRows(b *testing.B) {
	const rows = 500
	test.NewTempApp(b)
	lang.SetLocaleForTests(b, "en-US")
	box := container.NewVBox()
	w := test.NewTempWindow(b, container.NewDirectionScope(container.NewVScroll(box), bellui.DirectionRTL))
	w.Resize(geo.NewSize(400, 400))
	items := make([]*bellui.Container, rows)
	for i := range items {
		items[i] = container.NewHBox(widget.NewLabel("row "+strconv.Itoa(i)), widget.NewLabel("detail"), widget.NewCheck("", nil))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, it := range items {
			box.Add(it)
		}
		for _, it := range items {
			box.Remove(it)
		}
	}
}

// BenchmarkDirectedCanvas_RelabelRows changes the text of 1000 labels in a
// window whose canvas has its own direction and no scope, each SetText a
// Refresh outside any layout walk (#749).
func BenchmarkDirectedCanvas_RelabelRows(b *testing.B) {
	const rows = 1000
	test.NewTempApp(b)
	lang.SetLocaleForTests(b, "en-US")
	labels := make([]*widget.Label, rows)
	objs := make([]core.CanvasObject, rows)
	for i := range labels {
		labels[i] = widget.NewLabel("row " + strconv.Itoa(i))
		objs[i] = labels[i]
	}
	w := test.NewTempWindow(b, container.NewVScroll(container.NewVBox(objs...)))
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	w.Resize(geo.NewSize(400, 400))
	b.ReportAllocs()
	b.ResetTimer()
	for f := range b.N {
		for i, l := range labels {
			l.SetText("row " + strconv.Itoa(i) + " " + strconv.Itoa(f))
		}
	}
}
