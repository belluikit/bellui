//go:build !wasm

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

// directionBenchScene is a 1000-row window with a block of animated
// progress bars above the rows: wrapped in a right-to-left DirectionScope when
// scoped, plain otherwise (#749, #760). The scope must be the only one in the
// process, so run each benchmark in a process of its own (-bench with an
// anchored name): uidir remembers that a scope was rendered.
type directionBenchScene struct {
	w      *window
	c      *canvas
	labels []*widget.Label
	bars   []*widget.ProgressBar
}

func newDirectionBenchScene(tb testing.TB, scoped bool) *directionBenchScene {
	const rows, bars = 1000, 20
	s := &directionBenchScene{labels: make([]*widget.Label, rows), bars: make([]*widget.ProgressBar, bars)}
	bellui.DoAndWait(func() {
		objs := make([]core.CanvasObject, rows)
		for i := range objs {
			s.labels[i] = widget.NewLabel("row " + strconv.Itoa(i))
			objs[i] = container.NewHBox(s.labels[i], widget.NewLabel("detail"), widget.NewCheck("", nil))
		}
		barObjs := make([]core.CanvasObject, bars)
		for i := range barObjs {
			s.bars[i] = widget.NewProgressBar()
			barObjs[i] = s.bars[i]
		}
		var top core.CanvasObject = container.NewGridWithColumns(4, barObjs...)
		if scoped {
			top = container.NewDirectionScope(top, bellui.DirectionRTL)
		}
		s.w = d.CreateWindow("direction bench").(*window)
		s.c = s.w.Canvas().(*canvas)
		s.c.SetContent(container.NewBorder(top, nil, nil, nil, container.NewVScroll(container.NewVBox(objs...))))
		s.c.Resize(geo.NewSize(1280, 800))
		settle(s.c)
	})
	tb.Cleanup(func() { bellui.DoAndWait(s.w.Close) })
	return s
}

// frame advances the animation as an animation tick does — each bar's
// SetValue refreshes it outside any layout walk — then runs the frame's
// min-size pass.
func (s *directionBenchScene) frame(f int) {
	for i, b := range s.bars {
		b.SetValue(float64((f+i)%100) / 100)
	}
	s.c.EnsureMinSize()
}

func benchmarkDirectionFrame(b *testing.B, scoped bool) {
	s := newDirectionBenchScene(b, scoped)
	b.ReportAllocs()
	b.ResetTimer()
	for f := 0; f < b.N; f++ {
		bellui.DoAndWait(func() { s.frame(f) })
	}
}

// BenchmarkDirectionFrameNoScope is a frame of the scene with no scope in the
// app: the cost #749 must leave unchanged.
func BenchmarkDirectionFrameNoScope(b *testing.B) { benchmarkDirectionFrame(b, false) }

// BenchmarkDirectionFrameRTLScope is a frame of the scene with the animated
// bars inside a right-to-left scope.
func BenchmarkDirectionFrameRTLScope(b *testing.B) { benchmarkDirectionFrame(b, true) }

// BenchmarkEnsureMinSizeRowsWithScope times only the min-size pass of the
// scoped scene after every row label changed, so the pass measures text and
// re-lays the rows out — all outside the scope, where every Resize inside the
// walk would look its direction up if the walk did not push one (#760).
func BenchmarkEnsureMinSizeRowsWithScope(b *testing.B) {
	s := newDirectionBenchScene(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for f := 0; f < b.N; f++ {
		bellui.DoAndWait(func() {
			b.StopTimer()
			for i, l := range s.labels {
				l.SetText("row " + strconv.Itoa(i) + " " + strconv.Itoa((f+i)%1000))
			}
			b.StartTimer()
			s.c.EnsureMinSize()
		})
	}
}
