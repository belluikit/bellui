package chart

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

func makeTestChart(name string, data []float64) *Chart {
	return New(Option{
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
		Series: []series.Series{
			&series.LineSeries{SeriesName: name, Data: data},
		},
	})
}

func TestChartGroup_Layout(t *testing.T) {
	_ = test.NewApp()

	price := makeTestChart("price", []float64{10, 20, 30})
	volume := makeTestChart("volume", []float64{100, 200, 300})

	g := NewChartGroup(
		Pane{Chart: price, Height: 3},
		Pane{Chart: volume, Height: 1},
	)
	g.Resize(geo.NewSize(800, 400))

	renderer := g.CreateRenderer()
	renderer.Layout(geo.NewSize(800, 400))

	// Price chart should get 75% of height (3/4), volume 25%.
	priceSize := price.Size()
	volumeSize := volume.Size()

	if priceSize.Width != 800 {
		t.Errorf("expected price width 800, got %v", priceSize.Width)
	}
	if priceSize.Height != 300 {
		t.Errorf("expected price height 300 (75%% of 400), got %v", priceSize.Height)
	}
	if volumeSize.Height != 100 {
		t.Errorf("expected volume height 100 (25%% of 400), got %v", volumeSize.Height)
	}

	// Volume chart should be positioned below price.
	volumePos := volume.Position()
	if volumePos.Y != 300 {
		t.Errorf("expected volume Y=300, got %v", volumePos.Y)
	}
}

func TestChartGroup_ZoomSync(t *testing.T) {
	_ = test.NewApp()

	price := makeTestChart("price", []float64{10, 20, 30, 40, 50})
	volume := makeTestChart("volume", []float64{100, 200, 300, 400, 500})

	g := NewChartGroup(
		Pane{Chart: price, Height: 3},
		Pane{Chart: volume, Height: 1},
	)
	g.Resize(geo.NewSize(800, 400))

	// Simulate a zoom on the price chart.
	price.Events.Emit(interact.ZoomChangeEvent{
		Event:    interact.Event{Type: interact.EventZoomChange},
		Start:    0.2,
		End:      0.8,
		FromUser: true,
	})

	// Volume chart should now have the same zoom window.
	volumeWindow := volume.ZoomWindow()
	if volumeWindow.Start != 0.2 || volumeWindow.End != 0.8 {
		t.Errorf("expected volume zoom [0.2, 0.8], got [%v, %v]", volumeWindow.Start, volumeWindow.End)
	}
}

func TestChartGroup_ZoomSyncDisabled(t *testing.T) {
	_ = test.NewApp()

	price := makeTestChart("price", []float64{10, 20, 30})
	volume := makeTestChart("volume", []float64{100, 200, 300})

	g := NewChartGroup(
		Pane{Chart: price, Height: 3},
		Pane{Chart: volume, Height: 1},
	)
	g.SetSyncEnabled(false)

	// Simulate a zoom on the price chart.
	price.Events.Emit(interact.ZoomChangeEvent{
		Event:    interact.Event{Type: interact.EventZoomChange},
		Start:    0.3,
		End:      0.7,
		FromUser: true,
	})

	// Volume chart should NOT have changed.
	volumeWindow := volume.ZoomWindow()
	if volumeWindow.Start != 0 || volumeWindow.End != 1 {
		t.Errorf("expected volume zoom [0, 1] with sync disabled, got [%v, %v]", volumeWindow.Start, volumeWindow.End)
	}
}

func TestChartGroup_MinSize(t *testing.T) {
	_ = test.NewApp()

	price := makeTestChart("price", []float64{10, 20})
	volume := makeTestChart("volume", []float64{100, 200})

	g := NewChartGroup(
		Pane{Chart: price, Height: 3},
		Pane{Chart: volume, Height: 1},
	)

	minSize := g.MinSize()
	if minSize.Width < 200 {
		t.Errorf("expected min width >= 200, got %v", minSize.Width)
	}
	if minSize.Height < 300 {
		t.Errorf("expected min height >= 300 (two panes), got %v", minSize.Height)
	}
}

func TestChartGroup_DefaultHeight(t *testing.T) {
	_ = test.NewApp()

	price := makeTestChart("price", []float64{10, 20})
	volume := makeTestChart("volume", []float64{100, 200})

	// Height=0 should be treated as 1 (equal split).
	g := NewChartGroup(
		Pane{Chart: price},
		Pane{Chart: volume},
	)
	g.Resize(geo.NewSize(800, 400))

	renderer := g.CreateRenderer()
	renderer.Layout(geo.NewSize(800, 400))

	// Both panes should get 50% of height.
	priceSize := price.Size()
	if priceSize.Height != 200 {
		t.Errorf("expected equal split height 200, got %v", priceSize.Height)
	}
}
