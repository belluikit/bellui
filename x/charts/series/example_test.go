package series_test

import (
	"fmt"

	"github.com/belluikit/bellui/x/charts/series"
)

func ExampleLineSeries() {
	ls := &series.LineSeries{
		SeriesName: "Temperature",
		Data:       []float64{22, 25, 28, 24, 21},
		Symbol:     series.SymbolCircle,
	}
	fmt.Println(ls.Name())
	fmt.Println(ls.SeriesType())
	// Output:
	// Temperature
	// line
}

func ExampleBarSeries() {
	bs := &series.BarSeries{
		SeriesName:   "Revenue",
		Data:         []float64{100, 200, 150, 300},
		CornerRadius: 4,
	}
	fmt.Println(bs.Name())
	fmt.Println(bs.SeriesType())
	// Output:
	// Revenue
	// bar
}

func ExampleScatterSeries() {
	ss := &series.ScatterSeries{
		SeriesName: "Points",
		Data:       []float64{1, 2, 3, 4, 5, 6},
		Symbol:     series.SymbolDiamond,
	}
	fmt.Println(ss.Name())
	fmt.Println(ss.SeriesType())
	fmt.Println(ss.PointCount())
	// Output:
	// Points
	// scatter
	// 3
}

func ExamplePieSeries() {
	ps := &series.PieSeries{
		SeriesName: "Share",
		Data: []series.PieDataItem{
			{Name: "A", Value: 60},
			{Name: "B", Value: 40},
		},
		ShowLabels: true,
	}
	fmt.Println(ps.Name())
	fmt.Println(ps.SeriesType())
	fmt.Println(len(ps.Data))
	// Output:
	// Share
	// pie
	// 2
}
