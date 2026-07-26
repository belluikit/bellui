package builder_test

import (
	"fmt"

	"github.com/belluikit/bellui/x/charts/builder"
	"github.com/belluikit/bellui/x/charts/series"
)

func ExampleChartBuilder() {
	b := builder.Chart().
		Title("Sales").
		CategoryX("Q1", "Q2", "Q3", "Q4").
		AddBar("Revenue", 100, 200, 150, 300).Done().
		AddLine("Trend", 120, 180, 160, 280).Done()
	opt := b.Build()
	fmt.Println(opt.Title.Text)
	fmt.Println(len(opt.Series))
	// Output:
	// Sales
	// 2
}

func ExampleLineChart() {
	opt := builder.LineChart().
		Title("Temperatures").
		CategoryX("Mon", "Tue", "Wed", "Thu", "Fri").
		ValueY().
		AddLine("High", 28, 30, 32, 29, 27).Symbol(series.SymbolCircle).Done().
		AddLine("Low", 18, 20, 22, 19, 17).AreaFill(0.3).Done().
		Build()
	fmt.Println(opt.Title.Text)
	fmt.Println(len(opt.Series))
	// Output:
	// Temperatures
	// 2
}

func ExamplePieChart() {
	opt := builder.PieChart().
		Title("Market Share").
		AddPie("Browsers").
		Slice("Chrome", 65).
		Slice("Firefox", 15).
		Slice("Safari", 12).
		Labels().
		Done().
		Build()
	fmt.Println(opt.Title.Text)
	fmt.Println(len(opt.Series))
	ps := opt.Series[0].(*series.PieSeries)
	fmt.Println(len(ps.Data))
	// Output:
	// Market Share
	// 1
	// 3
}
