package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// disabledTab holds one of each widget that can be disabled, and three
// containers whose content a disable reaches, behind one switch, so a tester
// can check that everything is visible but inert and has the one disabled
// look (#1212–#1218, #1221, #1223, #1232, #1251).
func disabledTab() core.CanvasObject {
	status := widget.NewLabel("Everything below is enabled.")
	status.Wrapping = bellui.TextWrapWord
	tapped := func(what string) func() {
		return func() { status.SetText(fmt.Sprintf("%s tapped at %s", what, time.Now().Format("15:04:05"))) }
	}

	entry := widget.NewEntry()
	entry.SetText("Select and copy this text")
	sel := widget.NewSelect([]string{"One", "Two", "Three"}, nil)
	sel.SetSelected("One")
	slider := widget.NewSlider(0, 100)
	slider.SetValue(40)
	sw := widget.NewSwitch(nil)
	sw.On = true
	radio := widget.NewRadioGroup([]string{"Left", "Right"}, nil)
	radio.SetSelected("Left")
	segments := widget.NewSegmentedButton([]string{"Day", "Week", "Month"}, nil)
	rich := widget.NewRichEntry()
	rich.SetSegments([]widget.RichTextSegment{&widget.TextSegment{Style: widget.RichTextStyleInline, Text: " a rich entry with an orange image"}})
	rich.InsertImage(swatch(color.NRGBA{R: 255, G: 120, B: 0, A: 255}))
	bar := widget.NewTopAppBar("App bar title", theme.MenuIcon(), tapped("App bar menu"))
	bar.Actions = []widget.TopAppBarAction{{Icon: theme.SearchIcon(), OnTapped: tapped("App bar search")}}
	nav := widget.NewNavigationBar([]*widget.NavigationBarItem{
		{Label: "Home", Icon: theme.HomeIcon()}, {Label: "Search", Icon: theme.SearchIcon()}, {Label: "Settings", Icon: theme.SettingsIcon()},
	}, nil)
	carousel := widget.NewCarousel(carouselPage(1), carouselPage(2), carouselPage(3))

	rows := make([]string, 100)
	for i := range rows {
		rows[i] = fmt.Sprintf("List row %d", i+1)
	}
	list := widget.NewList(
		func() int { return len(rows) },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(rows[i]) },
	)
	list.Select(2)
	tree := widget.NewTreeWithStrings(map[string][]string{
		"": {"Fruit", "Vegetables"}, "Fruit": {"Apple", "Banana", "Cherry", "Date"}, "Vegetables": {"Carrot", "Daikon", "Eggplant"},
	})
	tree.OpenBranch("Fruit")
	tree.Select("Banana")
	treeView := newTreeView(false)

	// The containers: each holds an app-disabled check that must stay
	// disabled when the container is enabled again (#1218).
	appDisabled := widget.NewCheck("Disabled by the app", nil)
	appDisabled.Disable()
	tabs := container.NewAppTabs(
		container.NewTabItem("First tab", container.NewVBox(widget.NewEntry(), widget.NewButton("Button in a tab", tapped("Button in a tab")), appDisabled)),
		container.NewTabItem("Second tab", container.NewVBox(widget.NewButton("Button in the second tab", tapped("Button in the second tab")), widget.NewCheck("Check in the second tab", nil))),
	)
	docs := container.NewDocTabs(
		container.NewTabItem("Doc one", widget.NewButton("Button in a doc tab", tapped("Button in a doc tab"))),
		container.NewTabItem("Doc two", widget.NewLabel("Second document")),
	)
	inner := container.NewInnerWindow("Inner window", container.NewVBox(widget.NewEntry(), widget.NewButton("Button in the window", tapped("Button in the window"))))

	button := widget.NewButton("Button", tapped("Button"))
	iconButton := widget.NewButtonWithIcon("Icon button", theme.ContentAddIcon(), tapped("Icon button"))
	round := widget.NewIconButton(theme.InfoIcon(), tapped("Round icon button"))
	fab := widget.NewFAB(theme.ContentAddIcon(), tapped("FAB"))
	check := widget.NewCheck("Check", nil)
	chip := widget.NewChip("Assist chip", tapped("Assist chip"))
	filter := widget.NewFilterChip("Filter chip", nil)
	date := widget.NewDatePicker(time.Date(2026, 3, 9, 0, 0, 0, 0, time.Local), nil)
	clock := widget.NewTimePicker(9, 30, true, nil)

	widgets := []core.Disableable{
		button, iconButton, round, fab, entry, sel, slider, sw, check, radio, segments, chip, filter,
		date, clock, rich, bar, nav, carousel, list, tree, treeView, tabs, docs, inner,
	}
	setAll := func(off bool) {
		for _, d := range widgets {
			if off {
				d.Disable()
			} else {
				d.Enable()
			}
		}
		if off {
			status.SetText("Everything below is disabled.")
		} else {
			status.SetText("Everything below is enabled.")
		}
	}
	toggle := widget.NewCheck("Disable everything below", setAll)
	later := widget.NewButton("Disable everything in 3 seconds", func() {
		status.SetText("Disabling in 3 seconds…")
		time.AfterFunc(3*time.Second, func() { bellui.Do(func() { toggle.SetChecked(true) }) })
	})

	box := func(w, h float32, o core.CanvasObject) core.CanvasObject {
		return container.NewGridWrap(geo.NewSize(w, h), o)
	}
	return container.NewVScroll(container.NewVBox(
		toggle, later, status,
		widget.NewSeparator(),
		widget.NewCard("Actions and choices", "", container.NewVBox(
			container.NewHBox(button, iconButton, round, fab),
			entry, sel, slider,
			container.NewHBox(sw, check, radio),
			segments,
			container.NewHBox(chip, filter),
		)),
		widget.NewCard("Pickers", "", container.NewVBox(date, clock)),
		widget.NewCard("Rich entry", "", box(320, 90, rich)),
		widget.NewCard("Bars", "", container.NewVBox(bar, nav)),
		widget.NewCard("Collections (wheel or swipe over them)", "", container.NewVBox(
			box(320, 120, carousel), box(320, 180, list), box(320, 180, tree), box(320, 180, treeView),
		)),
		widget.NewCard("Containers", "Each holds a check the app disabled itself", container.NewVBox(
			box(360, 200, tabs), box(360, 120, docs), box(360, 180, inner),
		)),
	))
}

// swatch is a small solid image, so a dimmed image is easy to see.
func swatch(c color.Color) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 48, 24))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

// newTreeView is a small TreeView, its rows movable or not.
func newTreeView(reorder bool) *widget.TreeView {
	tv := widget.NewTreeView()
	tv.AllowReorder = reorder
	for _, b := range []string{"Inbox", "Projects"} {
		_ = tv.AddNode("", widget.TreeViewNode{ID: b, Text: b})
		for i := 1; i <= 4; i++ {
			id := fmt.Sprintf("%s %d", b, i)
			_ = tv.AddNode(b, widget.TreeViewNode{ID: id, Text: id})
		}
	}
	tv.OpenAllBranches()
	return tv
}

// drawerTab opens a NavigationDrawer from either edge, over the whole window
// or below its app bar, and shows what OnOpenChanged reports (#1171, #1186,
// #1238, #1280). The page under it scrolls, for the scrim check (#1173).
func drawerTab(w bellui.Window) core.CanvasObject {
	state := widget.NewLabel("Drawer: closed. OnOpenChanged has fired 0 times.")
	picked := widget.NewLabel("Destination: Inbox")
	drawer := widget.NewNavigationDrawer([]*widget.NavigationDrawerItem{
		{Label: "Inbox", Icon: theme.MailComposeIcon()},
		{Label: "Documents", Icon: theme.DocumentIcon()},
		{IsDivider: true},
		{Label: "Settings", Icon: theme.SettingsIcon()},
	}, nil)
	drawer.Header = widget.NewLabelWithStyle("Release check", bellui.TextAlignLeading, bellui.TextStyle{Bold: true})
	var bar *widget.TopAppBar
	bar = widget.NewTopAppBar("Drawer", theme.MenuIcon(), func() {
		if drawer.IsOpen() {
			drawer.Hide()
		} else {
			drawer.Show(w.Canvas())
		}
	})
	drawer.OnChanged = func(i int) { picked.SetText("Destination: " + drawer.Items[i].Label) }
	fired := 0
	drawer.OnOpenChanged = func(open bool) {
		fired++
		word, icon := "closed", theme.MenuIcon()
		if open {
			word, icon = "open", theme.CancelIcon()
		}
		state.SetText(fmt.Sprintf("Drawer: %s. OnOpenChanged has fired %d times.", word, fired))
		bar.NavigationIcon = icon
		bar.Refresh()
	}

	side := widget.NewRadioGroup([]string{"Start edge", "End edge"}, func(s string) {
		drawer.Side = widget.DrawerSideStart
		if s == "End edge" {
			drawer.Side = widget.DrawerSideEnd
		}
	})
	side.Horizontal = true
	side.SetSelected("Start edge")
	below := widget.NewCheck("Open below the app bar", func(on bool) {
		drawer.Top = 0
		if on {
			d := bellui.CurrentApp().Driver()
			drawer.Top = d.AbsolutePositionForObject(bar).Y + bar.Size().Height
		}
	})
	open := widget.NewButton("Show", func() { drawer.Show(w.Canvas()) })
	hideLater := widget.NewButton("Show twice, then Hide after 2 seconds", func() {
		drawer.Show(w.Canvas())
		drawer.Show(w.Canvas()) // already open: OnOpenChanged must not fire again
		time.AfterFunc(2*time.Second, func() { bellui.Do(drawer.Hide) })
	})

	rows := make([]core.CanvasObject, 40)
	for i := range rows {
		rows[i] = widget.NewLabel(fmt.Sprintf("Page row %d: scroll over the scrim must not move these", i+1))
	}
	page := container.NewVBox(append([]core.CanvasObject{
		side, below, open, hideLater, state, picked, widget.NewSeparator(),
	}, rows...)...)
	return container.NewBorder(bar, nil, nil, nil, container.NewVScroll(page))
}
