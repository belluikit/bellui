//go:build partialrepaint

package common

import (
	"image/color"
	"strconv"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// BenchmarkPartialDirtyRegion is the dirty-region walk on hack/paintcullprobe's
// scene at a phone's canvas size: n MD3 cards in a scrolled grid, of which
// about ten are on screen, and one small object refreshed per frame. On a
// Pixel 9a the walk cost 2.5 ms at 500 cards and 6.3 ms at 2000, growing with
// the cards off screen, while every region walked every card; it now walks
// from the refreshed object alone (#599).
func BenchmarkPartialDirtyRegion(b *testing.B) {
	for _, n := range []int{500, 2000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			test.NewTempApp(b).Settings().SetTheme(theme.DefaultTheme()) // rounded cards, as in the app
			mark := canvas.NewRectangle(color.White)
			mark.SetMinSize(geo.NewSquareSize(1))
			cards := make([]core.CanvasObject, n)
			for i := range cards {
				body := container.NewVBox(widget.NewLabel(strconv.Itoa(i)), widget.NewProgressBar(), widget.NewButton("Open", func() {}))
				cards[i] = widget.NewCard("Tile "+strconv.Itoa(i), "", body)
			}
			scroll := container.NewVScroll(container.NewGridWrap(geo.NewSize(180, 200), cards...))
			content := container.NewBorder(container.NewHBox(mark), nil, nil, nil, scroll)
			size := geo.NewSize(411, 923) // the Pixel 9a's canvas
			content.Resize(size)
			c := newPartialCanvas(content, size)
			c.PartialDirtyRegion() // records every node's bounds
			c.PartialDirtyRegion()
			node, ok := c.NodeFor(mark)
			if !ok {
				b.Fatal("mark has no node")
			}

			b.ReportAllocs()
			b.ResetTimer()
			visits := 0
			for b.Loop() {
				c.notePaintRefresh(node)
				if _, _, set := c.PartialDirtyRegion(); !set {
					b.Fatal("the refresh found no region")
				}
				visits += c.RegionVisits()
			}
			b.ReportMetric(float64(visits)/float64(b.N), "visits/op")
		})
	}
}
