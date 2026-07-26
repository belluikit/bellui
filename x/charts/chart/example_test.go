package chart_test

import (
	"fmt"

	"github.com/belluikit/bellui/x/charts/chart"
	"github.com/belluikit/bellui/x/charts/series"
)

func ExampleNew() {
	opt := chart.Option{
		Title: &chart.Title{Text: "Hello"},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "Data", Data: []float64{1, 2, 3}},
		},
	}
	c := chart.New(opt)
	fmt.Println(c.Option().Title.Text)
	// Output:
	// Hello
}
