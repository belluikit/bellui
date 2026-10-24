package test_test

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// renderTree builds a tree of widgets and renders it on a test canvas, which
// creates and caches a renderer for every visible widget in it. It returns the
// tree and every widget in it: those the tree holds directly and those only a
// renderer holds.
func renderTree(size int) (core.CanvasObject, []core.Widget) {
	text := strings.Repeat("some text to lay out ", 20)
	box := container.NewVBox()
	for range size {
		box.Add(widget.NewLabel("label"))
		e := widget.NewMultiLineEntry()
		e.SetText(text)
		box.Add(e)
	}
	card := widget.NewCard("title", "subtitle", box)

	c := test.NewCanvas()
	c.SetContent(card)
	c.Resize(geo.NewSize(400, 800))
	return card, renderedUnder(card)
}

// renderedUnder lists the widgets under o that have a cached renderer.
func renderedUnder(o core.CanvasObject) []core.Widget {
	var ws []core.Widget
	var walk func(core.CanvasObject)
	walk = func(o core.CanvasObject) {
		switch t := o.(type) {
		case *bellui.Container:
			for _, c := range t.Objects {
				walk(c)
			}
		case core.Widget:
			r, ok := cache.CachedRenderer(t)
			if !ok {
				return
			}
			ws = append(ws, t)
			for _, c := range r.Objects() {
				walk(c)
			}
		}
	}
	walk(o)
	return ws
}

func TestDestroyRenderers(t *testing.T) {
	test.NewTempApp(t)

	unrelated := widget.NewButton("keep", nil)
	test.TempWidgetRenderer(t, unrelated)
	before := cache.RenderedWidgets()

	tree, rendered := renderTree(3)
	// A hidden widget with a renderer: a visible walk would skip it.
	hidden := widget.NewLabel("hidden")
	cache.Renderer(hidden)
	hidden.Hide()
	tree.(*widget.Card).Content.(*bellui.Container).Add(hidden)
	rendered = append(rendered, hidden)

	// The card, its box's labels and entries, and widgets only a renderer
	// holds (each entry's scroller and text): more than the tree holds
	// directly.
	assert.Greater(t, len(rendered), 3*2+2)
	for _, w := range rendered {
		assert.True(t, cache.IsRendered(w), "%T has no renderer before", w)
	}

	test.DestroyRenderers(t, tree)

	for _, w := range rendered {
		assert.False(t, cache.IsRendered(w), "%T kept its renderer", w)
	}
	assert.True(t, cache.IsRendered(unrelated), "a widget outside the tree lost its renderer")
	// Nothing the tree made is left: not even the renderers of the label and
	// entry selections, which are not among their renderers' objects.
	assert.Equal(t, typesOf(before), typesOf(cache.RenderedWidgets()))

	// Releasing a tree with no renderers creates none.
	test.DestroyRenderers(t, tree)
	assert.Equal(t, typesOf(before), typesOf(cache.RenderedWidgets()))
	test.DestroyRenderers(t, nil)
}

// typesOf lists the types of the widgets in ws, so that a failure shows what
// was left behind.
func typesOf(ws map[core.Widget]struct{}) []string {
	var ts []string
	for w := range ws {
		ts = append(ts, fmt.Sprintf("%T", w))
	}
	slices.Sort(ts)
	return ts
}

// TestDestroyRenderers_LoopStaysFlat builds a tree per iteration, as a
// benchmark does, and checks that releasing each leaves the cache where it
// started.
func TestDestroyRenderers_LoopStaysFlat(t *testing.T) {
	test.NewTempApp(t)
	start := len(cache.RenderedWidgets())
	for range 20 {
		tree, _ := renderTree(5)
		test.DestroyRenderers(t, tree)
	}
	assert.Equal(t, start, len(cache.RenderedWidgets()))
}

// BenchmarkDestroyRenderers reports the live heap, after a GC, once the
// benchmark has built b.N trees, with and without releasing each one.
func BenchmarkDestroyRenderers(b *testing.B) {
	for _, release := range []bool{false, true} {
		name := "kept"
		if release {
			name = "released"
		}
		b.Run(name, func(b *testing.B) {
			a := test.NewApp()
			defer a.Quit()
			heap := liveHeap()
			for b.Loop() {
				tree, _ := renderTree(20)
				if release {
					test.DestroyRenderers(b, tree)
				}
			}
			grown := float64(liveHeap() - heap)
			b.ReportMetric(grown/(1<<20), "live-MB")
			b.ReportMetric(grown/float64(b.N)/(1<<10), "live-KB/op")
			b.ReportMetric(float64(len(cache.RenderedWidgets())), "renderers")
			// Leave the cache as it was for the next sub-benchmark.
			for w := range cache.RenderedWidgets() {
				cache.DestroyRenderer(w)
			}
		})
	}
}

func liveHeap() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}
