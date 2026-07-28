// Command geoprobe is the live-verify probe for the Geo tab freeze (#35, #39).
//
//	go run ./cmd/geoprobe          # window stays open, watch it draw
//	go run ./cmd/geoprobe -auto    # measure and exit non-zero if the UI stalls
//
// It builds the same two charts the demo's Geo tab does — a tile-backed
// MapSeries and a ChoroplethSeries pointed at the 14 MB country boundary file —
// and, from a background goroutine, repeatedly times a round trip onto the main
// goroutine. That latency *is* the beachball: while the choropleth downloaded
// and parsed its GeoJSON inside Render, and while every repaint re-rasterised
// every country, the main goroutine was busy and the round trip stalled.
//
// It needs the network. Tiles come from OpenStreetMap, so do not leave it
// running in a loop.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"sort"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"

	"github.com/belluikit/bellui/x/charts/chart"
	"github.com/belluikit/bellui/x/charts/series"
)

const countriesURL = "https://raw.githubusercontent.com/datasets/geo-countries/master/data/countries.geojson"

// stallBudget is what counts as a freeze. The reported symptom was 427 ms of
// blocked UI for the download alone, then 226 ms per repaint; a responsive app
// should round-trip in single-digit milliseconds.
const stallBudget = 150 * time.Millisecond

func main() {
	auto := flag.Bool("auto", false, "measure main-goroutine latency, then exit")
	samples := flag.Int("samples", 120, "number of latency samples in -auto mode")
	only := flag.String("only", "both", "which charts to build: map, choropleth or both")
	warmup := flag.Duration("warmup", 2*time.Second, "settle time before sampling, to exclude app/window startup")
	flag.Parse()

	a := app.New()
	w := a.NewWindow("Geo tab probe (#35, #39)")

	center := [2]float64{0, 20}
	zoom := float32(2)

	worldMap := chart.New(chart.Option{
		Title:  &chart.Title{Text: "Map", SubText: "OpenStreetMap tiles"},
		Series: []series.Series{&series.MapSeries{SeriesName: "Base", Center: center, Zoom: zoom}},
	})

	choropleth := chart.New(chart.Option{
		Title: &chart.Title{Text: "Choropleth", SubText: "World GDP by country"},
		Series: []series.Series{
			&series.MapSeries{SeriesName: "Base", Center: center, Zoom: zoom},
			&series.ChoroplethSeries{
				SeriesName: "GDP",
				GeoJSONURL: countriesURL,
				MapCenter:  center,
				MapZoom:    zoom,
				RegionData: map[string]float64{
					"United States of America": 25.5, "China": 18.3, "Japan": 4.2,
					"Germany": 4.1, "India": 3.7, "United Kingdom": 3.1,
					"France": 2.8, "Canada": 2.1, "Italy": 2.0, "Brazil": 1.9,
				},
				Min: 0, Max: 26,
			},
		},
	})

	switch *only {
	case "map":
		w.SetContent(worldMap)
	case "choropleth":
		w.SetContent(choropleth)
	default:
		w.SetContent(container.NewGridWithColumns(2, worldMap, choropleth))
	}
	w.Resize(geo.NewSize(900, 600))

	if *auto {
		go runAuto(a, *samples, *warmup)
	}
	bellui.ShowAndRun(w)
}

// runAuto times round trips onto the main goroutine while the charts load.
func runAuto(a bellui.App, samples int, warmup time.Duration) {
	// Window creation, the first layout and GPU/font init cost far more than any
	// steady-state frame; sampling through them would measure startup, not the
	// Geo tab. The GeoJSON download outlives this settle time either way.
	time.Sleep(warmup)

	latencies := make([]time.Duration, 0, samples)
	worstAt := 0
	for i := 0; i < samples; i++ {
		start := time.Now()
		bellui.DoAndWait(func() {})
		d := time.Since(start)
		if len(latencies) == 0 || d > latencies[worstAt] {
			worstAt = i
		}
		latencies = append(latencies, d)
		time.Sleep(50 * time.Millisecond)
	}

	sorted := append([]time.Duration(nil), latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	median := sorted[len(sorted)/2]
	worst := sorted[len(sorted)-1]

	fmt.Printf("main-goroutine latency over %v: median %v, p95 %v, worst %v (sample %d of %d, ~%v in)\n",
		time.Duration(samples)*50*time.Millisecond, median,
		sorted[len(sorted)*95/100], worst,
		worstAt, samples, time.Duration(worstAt)*50*time.Millisecond)

	verdict := 0
	if worst > stallBudget {
		fmt.Printf("FAIL — the UI stalled for %v (budget %v)\n", worst, stallBudget)
		verdict = 1
	} else {
		fmt.Printf("PASS — never stalled beyond %v\n", stallBudget)
	}

	if !reportRasterCost() {
		fmt.Println("FAIL — the choropleth never got its data, so the run proved nothing")
		verdict = 1
	}

	bellui.Do(func() { a.Quit() })
	os.Exit(verdict)
}

// reportRasterCost times the choropleth rasterisation directly, off the widget,
// using the data the chart above already loaded. It reports the one cost the
// memo cannot remove — the first rasterisation of a new view — and doubles as
// proof that the GeoJSON actually arrived, so a quiet run cannot pass by having
// drawn nothing at all. It returns false when there is no data to draw.
func reportRasterCost() bool {
	vp := coord.Rect{X: 0, Y: 0, W: 900, H: 600}
	gc := coord.NewGeoCoord(0, 20, 2)
	gc.SetBounds(vp)

	cs := &series.ChoroplethSeries{
		SeriesName: "GDP",
		GeoJSONURL: countriesURL,
		MapCenter:  [2]float64{0, 20},
		MapZoom:    2,
		RegionData: map[string]float64{"France": 2.8},
	}
	objs := series.NewRenderer(series.TypeChoropleth).Render(series.RenderContext{
		Series: cs, Coord: gc, Viewport: vp, Color: color.White,
	})
	if len(objs) == 0 {
		return false
	}
	rast, ok := objs[0].(*canvas.Raster)
	if !ok {
		return false
	}

	start := time.Now()
	first := rast.Generator(900, 600)
	firstCost := time.Since(start)
	if first == nil {
		return false
	}

	start = time.Now()
	for i := 0; i < 10; i++ {
		rast.Generator(900, 600)
	}
	repaintCost := time.Since(start) / 10

	fmt.Printf("choropleth rasterisation at 900x600: first %v, cached repaint %v\n",
		firstCost.Round(time.Microsecond), repaintCost.Round(time.Microsecond))
	return true
}
