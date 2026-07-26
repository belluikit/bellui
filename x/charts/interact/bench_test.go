package interact

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/series"
)

func BenchmarkHitTester_FindNearest(b *testing.B) {
	// Set up a coordinate system
	xScale := axis.NewLinearScale(5, nil, nil)
	yScale := axis.NewLinearScale(5, nil, nil)
	xScale.SetRange(0, 99)
	yScale.SetRange(0, 100)
	cs := coord.NewCartesian2D(xScale, yScale)
	cs.SetBounds(coord.Rect{X: 0, Y: 0, W: 1000, H: 600})

	// Register 5 series with 100 points each
	ht := NewHitTester()
	for s := 0; s < 5; s++ {
		data := make([]float64, 100)
		for i := range data {
			data[i] = float64(i*s%100) + 10
		}
		ht.AddLineSeries(s, "series", series.TypeLine, cs, data)
	}

	queryPos := geo.NewPos(500, 300)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ht.FindNearest(queryPos, 30)
	}
}

func BenchmarkEventBus_Emit(b *testing.B) {
	bus := NewEventBus()
	count := 0
	for i := 0; i < 10; i++ {
		bus.On(EventClick, func(evt interface{}) {
			count++
		})
	}

	evt := DataPointEvent{
		Event:       Event{Type: EventClick},
		SeriesIndex: 0,
		DataIndex:   5,
		Value:       DataValue{X: 5, Y: 42},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit(evt)
	}
}
