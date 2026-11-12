// Command rowpressprobe shows the press feedback of list rows, tabs and
// navigation bar items (#1091) on a device, for a screen recording.
//
// Tabs sit at the top, a list of 100 rows fills the middle, and a navigation
// bar sits at the bottom. Every tap logs
//
//	rowpressprobe: TAP <what>
//
// Expected, with the change: a press on a row, a tab or a navigation item
// shows a state layer from the press to the release; on a row it fills the
// row, on a navigation item it fills the indicator pill. A fling that starts
// on a row leaves no row pressed and logs no TAP. Before #1091 none of them
// showed anything while pressed.
//
//	go run ./hack/rowpressprobe
//
// On Android, package it and read logcat for "rowpressprobe".
package main

import (
	"fmt"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func main() {
	log.SetPrefix("rowpressprobe: ")
	a := app.NewWithID("com.bellui.rowpressprobe")
	w := a.NewWindow("rowpressprobe")

	list := widget.NewList(func() int { return 100 },
		func() core.CanvasObject { return widget.NewLabel("row") },
		func(id widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(fmt.Sprintf("Row %d", id)) })
	list.OnSelected = func(id widget.ListItemID) { log.Printf("TAP row %d", id) }

	tabs := container.NewAppTabs(
		container.NewTabItem("Rows", list),
		container.NewTabItem("Second", widget.NewLabel("Second tab")),
		container.NewTabItem("Third", widget.NewLabel("Third tab")),
	)
	tabs.OnSelected = func(t *container.TabItem) { log.Printf("TAP tab %s", t.Text) }

	nav := widget.NewNavigationBar([]*widget.NavigationBarItem{
		{Label: "Home", Icon: theme.HomeIcon()},
		{Label: "Mail", Icon: theme.MailComposeIcon()},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}, nil)
	nav.OnChanged = func(i int) {
		log.Printf("TAP nav %d", i)
		nav.Selected = i
		nav.Refresh()
	}

	w.SetContent(container.NewBorder(nil, nav, nil, nil, tabs))
	w.Resize(geo.NewSize(400, 700))
	bellui.ShowAndRun(w)
}
