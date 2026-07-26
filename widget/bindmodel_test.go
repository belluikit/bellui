package widget_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func labelText(o core.CanvasObject) string { return o.(*widget.Label).Text }

func TestBindListModel_RendersAndTracks(t *testing.T) {
	test.NewTempApp(t)
	m := model.NewSliceModel("a", "b", "c")
	list := widget.NewList(nil, nil, nil)

	unbind := widget.BindListModel[string](
		list, m,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(s string, o core.CanvasObject) { o.(*widget.Label).SetText(s) },
	)
	defer unbind()

	assert.Equal(t, 3, list.Length())

	tmpl := list.CreateItem()
	list.UpdateItem(1, tmpl)
	assert.Equal(t, "b", labelText(tmpl))

	// A model change is reflected through the same callbacks.
	m.Append("d")
	assert.Equal(t, 4, list.Length())
	list.UpdateItem(3, tmpl)
	assert.Equal(t, "d", labelText(tmpl))
}

func TestBindListModel_OutOfRangeGuard(t *testing.T) {
	test.NewTempApp(t)
	m := model.NewSliceModel("a", "b", "c")
	list := widget.NewList(nil, nil, nil)
	widget.BindListModel[string](
		list, m,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(s string, o core.CanvasObject) { o.(*widget.Label).SetText(s) },
	)
	tmpl := list.CreateItem()
	m.Set([]string{"x"}) // shrink

	assert.NotPanics(t, func() { list.UpdateItem(2, tmpl) }, "stale id must be guarded")
}

func TestBindListModel_OverProxies(t *testing.T) {
	test.NewTempApp(t)
	src := model.NewSliceModel("delta", "alpha", "beta", "avocado")
	filtered := model.NewFilterProxy[string](src)
	filtered.SetPredicate(func(s string) bool { return strings.HasPrefix(s, "a") })
	sorted := model.NewSortProxy[string](filtered)
	sorted.SetLess(func(a, b string) bool { return a < b })

	list := widget.NewList(nil, nil, nil)
	widget.BindListModel[string](
		list, sorted,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(s string, o core.CanvasObject) { o.(*widget.Label).SetText(s) },
	)

	assert.Equal(t, 2, list.Length()) // alpha, avocado (beta/delta filtered out)
	tmpl := list.CreateItem()
	list.UpdateItem(0, tmpl)
	assert.Equal(t, "alpha", labelText(tmpl))
	list.UpdateItem(1, tmpl)
	assert.Equal(t, "avocado", labelText(tmpl))
}

func TestBindTableModel_RendersRowsAndCols(t *testing.T) {
	test.NewTempApp(t)
	type person struct {
		name string
		age  int
	}
	m := model.NewSliceModel(person{"Ada", 36}, person{"Bo", 41})
	table := widget.NewTable(nil, nil, nil)

	widget.BindTableModel[person](
		table, m, 2,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(p person, col int, cell core.CanvasObject) {
			if col == 0 {
				cell.(*widget.Label).SetText(p.name)
			} else {
				cell.(*widget.Label).SetText(strconv.Itoa(p.age))
			}
		},
	)

	rows, cols := table.Length()
	assert.Equal(t, 2, rows)
	assert.Equal(t, 2, cols)

	tmpl := table.CreateCell()
	table.UpdateCell(widget.TableCellID{Row: 1, Col: 0}, tmpl)
	assert.Equal(t, "Bo", labelText(tmpl))
	table.UpdateCell(widget.TableCellID{Row: 1, Col: 1}, tmpl)
	assert.Equal(t, "41", labelText(tmpl))
}

func TestBindListSelection_TapUpdatesModelAndSurvivesResort(t *testing.T) {
	test.NewTempApp(t)
	src := model.NewSliceModel("banana", "apple", "cherry")
	sorted := model.NewSortProxy[string](src)
	sorted.SetLess(func(a, b string) bool { return a < b }) // apple, banana, cherry
	sel := model.NewSelectionModel[string, string](sorted, func(s string) string { return s })

	list := widget.NewList(nil, nil, nil)
	widget.BindListModel[string](
		list, sorted,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(s string, o core.CanvasObject) { o.(*widget.Label).SetText(s) },
	)
	unbind := widget.BindListSelection[string, string](list, sorted, sel)
	defer unbind()

	// Simulate a tap on display row 1 ("banana").
	list.Select(1)
	assert.True(t, sel.IsSelected("banana"))
	assert.Equal(t, 1, sel.IndexOf("banana"))

	// Reverse the sort: banana moves; identity selection must follow, no loop.
	sorted.SetLess(func(a, b string) bool { return a > b }) // cherry, banana, apple
	assert.True(t, sel.IsSelected("banana"), "selection survives re-sort")
	assert.Equal(t, 1, sel.IndexOf("banana"), "still middle row after reverse")
}
