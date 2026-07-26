package widget

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

var globalRadioRenderer core.WidgetRenderer

func BenchmarkRadioCreateRenderer(b *testing.B) {
	var renderer core.WidgetRenderer
	widget := &radioItem{}
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		renderer = widget.CreateRenderer()
	}

	// Avoid having the value optimized out by the compiler.
	globalRadioRenderer = renderer
}

func TestRadioItem_FocusIndicator_Centered_Vertically(t *testing.T) {
	item := newRadioItem("Hello", nil)
	render := test.TempWidgetRenderer(t, item).(*radioItemRenderer)
	render.Layout(geo.NewSize(200, 100))

	focusIndicatorSize := theme.IconInlineSize() + 2*theme.Padding()
	heightCenterOffset := (100 - focusIndicatorSize) / 2
	assert.Equal(t, geo.NewPos(theme.Padding()/2, heightCenterOffset), render.focusIndicator.Position1)
}
