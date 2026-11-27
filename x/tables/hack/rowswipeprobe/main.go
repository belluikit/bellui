// Command rowswipeprobe shows a DataGrid whose rows swipe sideways (#1483),
// to check the gesture on a real touch screen and painter.
//
// The grid has a pinned-left column and main columns wider than a phone, so
// the body scrolls sideways: a sideways swipe scrolls it until it reaches an
// edge, and only a swipe that starts at the edge swipes a row. A swipe right
// reveals Flag; a swipe left reveals Archive and Delete, and swiping most of
// the way across archives at once. The toggle at the top lays the grid out
// right to left, which puts Flag on the right.
//
// Every action and cell click is printed, which on Android reaches logcat
// under the tag Bellui:
//
//	rowswipe action=Delete row=3
//	rowswipe click row=5 col=1
//
// Run it on the desktop with go run ., or package it from this directory:
//
//	bellui package --target android/arm64 --app-id com.trendvidia.rowswipeprobe
package main

import (
	"fmt"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"

	"github.com/belluikit/bellui/x/tables"
)

func main() {
	a := app.NewWithID("com.trendvidia.rowswipeprobe")
	w := a.NewWindow("rowswipeprobe")

	status := widget.NewLabel("swipe a row")
	report := func(s string) {
		fmt.Println("rowswipe " + s)
		status.SetText(s)
	}

	body := container.NewStack()
	build := func(rtl bool) {
		g := newGrid(report)
		dir := bellui.DirectionLTR
		if rtl {
			dir = bellui.DirectionRTL
		}
		body.Objects = []core.CanvasObject{container.NewDirectionScope(g, dir)}
		body.Refresh()
		fmt.Printf("rowswipe layout rtl=%v\n", rtl)
	}
	rtl := widget.NewCheck("Right to left", build)
	build(false)

	w.SetContent(container.NewBorder(container.NewVBox(rtl, status), nil, nil, nil, body))
	w.Resize(geo.NewSize(400, 700))
	bellui.ShowAndRun(w)
}

// newGrid builds a grid of 200 rows with a pinned id column and four main
// columns of 160 points.
func newGrid(report func(string)) *tables.DataGrid {
	type rec struct {
		ID                       int
		Name, City, Team, Status string
	}
	rows := make([]rec, 200)
	cities := []string{"Lisbon", "Osaka", "Lima", "Oslo", "Accra"}
	for i := range rows {
		rows[i] = rec{
			ID: i, Name: fmt.Sprintf("Person %d", i), City: cities[i%len(cities)],
			Team: fmt.Sprintf("Team %d", i%7), Status: []string{"open", "closed"}[i%2],
		}
	}
	cols := []*tables.ColumnDef{
		{ColID: "id", Field: "ID", HeaderName: "ID", Width: 60, Pinned: tables.PinLeft},
		{ColID: "name", Field: "Name", HeaderName: "Name", Width: 160},
		{ColID: "city", Field: "City", HeaderName: "City", Width: 160},
		{ColID: "team", Field: "Team", HeaderName: "Team", Width: 160},
		{ColID: "status", Field: "Status", HeaderName: "Status", Width: 160},
	}
	g := tables.NewDataGrid(cols, func() int { return len(rows) }, func(i int) any { return rows[i] })
	act := func(name string) func(int) {
		return func(src int) { report(fmt.Sprintf("action=%s row=%d", name, src)) }
	}
	g.LeadingSwipeActions = func(int) []tables.RowSwipeAction {
		return []tables.RowSwipeAction{
			{Label: "Flag", Icon: theme.ConfirmIcon(), Importance: widget.HighImportance, OnTriggered: act("Flag")},
		}
	}
	g.TrailingSwipeActions = func(int) []tables.RowSwipeAction {
		return []tables.RowSwipeAction{
			{Label: "Archive", Icon: theme.FolderIcon(), Importance: widget.SuccessImportance, FullSwipe: true, OnTriggered: act("Archive")},
			{Label: "Delete", Icon: theme.DeleteIcon(), Importance: widget.DangerImportance, OnTriggered: act("Delete")},
		}
	}
	g.OnCellClicked = func(id tables.CellID) {
		report(fmt.Sprintf("click row=%d col=%d", g.SourceRow(id.Row), id.Col))
	}
	return g
}
