// Command modelprobe is the live-verify demo for the data/model sort/filter/
// selection layer (#539): a 100k-row list with a live search box, a locale-aware
// sort toggle, and multi-selection that survives re-sort and re-filter.
//
//	go run ./hack/modelprobe
//
// Type in the search box to filter (FilterProxy), press Sort to toggle
// collated ordering (SortProxy + lang.Compare via model.StringLess), and click
// rows to multi-select. The selected count in the status bar stays correct as
// rows are filtered out and back in — the whole point of identity-keyed
// selection. Scrolling stays smooth at 100k rows because the proxies do no
// per-frame work (see BenchmarkSortProxyFrameWalk: 0 alloc / 0 comparator calls).
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/data/model"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

const rowCount = 100_000

func main() {
	a := app.New()
	w := a.NewWindow("model layer probe — 100k rows")

	rows := make([]string, rowCount)
	for i := range rows {
		rows[i] = "Item " + strconv.Itoa(i)
	}
	src := model.NewSliceModel(rows...)

	// source -> filter -> sort -> list.
	filtered := model.NewFilterProxy[string](src)
	sorted := model.NewSortProxy[string](filtered)
	sel := model.NewSelectionModel[string, string](sorted, func(s string) string { return s })
	sel.SetMode(model.SelectMulti)

	list := widget.NewList(nil, nil, nil)
	widget.BindListModel[string](
		list, sorted,
		func() core.CanvasObject { return widget.NewLabel("") },
		func(item string, o core.CanvasObject) {
			lbl := o.(*widget.Label)
			if sel.IsSelected(item) { // app-driven multi-select highlight
				lbl.TextStyle.Bold = true
				lbl.SetText("● " + item)
			} else {
				lbl.TextStyle.Bold = false
				lbl.SetText(item)
			}
		},
	)

	status := widget.NewLabel("")
	updateStatus := func() {
		status.SetText(fmt.Sprintf("showing %d / %d   selected %d",
			sorted.Len(), src.Len(), sel.Len()))
	}
	sel.AddListener(listenerFunc(func() { list.Refresh(); updateStatus() }))
	sorted.AddListener(listenerFunc(updateStatus))

	// Tap toggles selection; we paint our own highlight so drop the built-in one.
	list.OnSelected = func(id widget.ListItemID) {
		list.Unselect(id)
		sel.ToggleIndex(id)
	}

	applyFilter := func(q string) {
		if q == "" {
			filtered.SetPredicate(nil)
			return
		}
		needle := strings.ToLower(q)
		filtered.SetPredicate(func(s string) bool {
			return strings.Contains(strings.ToLower(s), needle)
		})
	}
	search := widget.NewSearchField(applyFilter)
	search.OnChanged = applyFilter // live filter as the user types

	ascending := false
	sortBtn := widget.NewButton("Sort: off", nil)
	sortBtn.OnTapped = func() {
		ascending = !ascending
		if ascending {
			sorted.SetLess(model.StringLess)
			sortBtn.SetText("Sort: A→Z")
		} else {
			sorted.SetLess(nil)
			sortBtn.SetText("Sort: off")
		}
	}

	clearSel := widget.NewButton("Clear selection", func() { sel.Clear() })

	top := container.NewBorder(nil, nil, nil,
		container.NewHBox(sortBtn, clearSel), search)
	w.SetContent(container.NewBorder(top, status, nil, nil, list))
	updateStatus()

	w.Resize(geo.NewSize(420, 640))
	w.Show()
	a.Run()
}

// listenerFunc adapts a func to a binding.DataListener at the call site.
type listenerFunc func()

func (f listenerFunc) DataChanged() { f() }
