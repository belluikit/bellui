package mobile

import (
	"strconv"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// BenchmarkEnsureMinSizeTiles times the min-size pass of hack/perfscenes'
// tiles scene (#487): 500 cards, each a label, a progress bar and a button,
// with every label and bar changed between passes, so the pass walks a path
// into every card. Only EnsureMinSize is timed.
func BenchmarkEnsureMinSizeTiles(b *testing.B) {
	const n = 500
	values := make([]*widget.Label, n)
	bars := make([]*widget.ProgressBar, n)
	var c *canvas
	bellui.DoAndWait(func() {
		objs := make([]core.CanvasObject, n)
		for i := range objs {
			values[i], bars[i] = widget.NewLabel("0"), widget.NewProgressBar()
			body := container.NewVBox(values[i], bars[i], widget.NewButton("Open", func() {}))
			objs[i] = widget.NewCard("Tile "+strconv.Itoa(i), "", body)
		}
		c = newCanvas(bellui.CurrentDevice()).(*canvas)
		c.SetContent(container.NewVScroll(container.NewGridWrap(geo.NewSize(180, 200), objs...)))
		c.Resize(geo.NewSize(1280, 800))
		settle(c)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for f := 0; f < b.N; f++ {
		bellui.DoAndWait(func() {
			b.StopTimer()
			for i := range values {
				v := (f + i) % 100
				values[i].SetText(strconv.Itoa(v))
				bars[i].SetValue(float64(v) / 100)
			}
			b.StartTimer()
			c.EnsureMinSize()
		})
	}
}
