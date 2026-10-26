// Command app is the release-check companion app: one screen per toolkit
// behaviour that the gallery does not exercise, so the release checklist
// (go run ./hack/releasecheck list) can point at something on every platform.
// It uses the core toolkit only, so it builds for desktop, Android, iOS and the
// web from this checkout:
//
//	go run ./hack/releasecheck/app                      # desktop
//	go run -tags accessibility ./hack/releasecheck/app  # desktop, screen readers
//	cd hack/releasecheck/app && bellui serve            # web
//	cd hack/releasecheck/app && bellui package -os android
package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// The app ships English and Arabic for its own greeting but not Persian, the
// case of #342: a Persian user must get the English text, not the Arabic.
const (
	greetingEN = `messages = { "release.greeting": "Hello {Name}" }`
	greetingAR = `messages = { "release.greeting": "مرحبا {Name}" }`
)

func main() {
	a := app.NewWithID("io.bellui.releasecheck")
	if err := lang.AddTranslationsForLocale([]byte(greetingEN), "en"); err != nil {
		bellui.LogError("English catalog", err)
	}
	if err := lang.AddTranslationsForLocale([]byte(greetingAR), "ar"); err != nil {
		bellui.LogError("Arabic catalog", err)
	}

	w := a.NewWindow("Release check")
	tabs := container.NewAppTabs(
		container.NewTabItem("Locale", localeTab(w)),
		container.NewTabItem("Direction", directionTab(w)),
		container.NewTabItem("Theme override", overrideTab()),
		container.NewTabItem("Menus", menusTab(w)),
		container.NewTabItem("Touch & keys", touchTab()),
	)
	w.SetMainMenu(bellui.NewMainMenu(demoMenu("Check")))
	w.SetContent(tabs)
	w.Resize(geo.NewSize(900, 640))
	w.Show()
	a.Run()
}

// localeTab shows what the active locale resolves to and lets the tester
// switch it at run time.
func localeTab(w bellui.Window) core.CanvasObject {
	info := widget.NewLabel("")
	info.Wrapping = bellui.TextWrapWord
	refresh := func() {
		info.SetText(fmt.Sprintf(
			"Locale: %s\nRight-to-left: %v\nApp greeting: %s\nBuilt-in string \"Cancel\": %s\n"+
				"Date: %s\nNumber: %s\nCurrency: %s\nList: %s",
			lang.CurrentLocale(), lang.IsRTL(),
			lang.X("release.greeting", "Hello {Name}", map[string]any{"Name": "Ada"}),
			lang.L("Cancel"),
			lang.FormatDate(time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)),
			lang.FormatDecimal(1234567.891, 2),
			lang.FormatCurrency(1234.5, "EUR"),
			lang.FormatList([]string{"red", "green", "blue"}),
		))
	}
	refresh()

	var buttons []core.CanvasObject
	for _, loc := range []string{"", "en-US", "de-DE", "ar-EG", "fa-IR", "he-IL", "ur-PK", "pt-BR", "sr-RS", "ja-JP"} {
		label := loc
		if loc == "" {
			label = "System"
		}
		buttons = append(buttons, widget.NewButton(label, func() {
			lang.SetLocale(bellui.Locale(loc))
			refresh()
		}))
	}
	confirm := widget.NewButton("Open a confirm dialog", func() {
		dialog.ShowConfirm("Confirm", "Built-in dialog buttons follow the locale.", func(bool) {}, w)
	})
	return container.NewVScroll(container.NewVBox(
		widget.NewLabel("Switch the locale:"),
		container.NewAdaptiveGrid(5, buttons...),
		confirm,
		widget.NewSeparator(),
		info,
	))
}

// directionTab puts a right-to-left scope inside the window, next to controls
// that change the whole window's or the whole app's direction.
func directionTab(w bellui.Window) core.CanvasObject {
	where := widget.NewLabel("")
	scoped := widget.NewEntry()
	scoped.SetText("abc 123 שלום")
	sel := widget.NewSelect([]string{"One", "Two", "Three"}, nil)
	sel.SetSelected("One")
	slider := widget.NewSlider(0, 10)
	slider.SetValue(3)
	order := container.NewHBox(widget.NewButton("First", nil), widget.NewButton("Second", nil), widget.NewButton("Third", nil))
	scope := container.NewDirectionScope(widget.NewCard("Right-to-left scope", "", container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Entry", scoped),
			widget.NewFormItem("Select", sel),
			widget.NewFormItem("Slider", slider),
			widget.NewFormItem("Check", widget.NewCheck("Remember", nil)),
		),
		order,
		where,
	)), bellui.DirectionRTL)
	// DirectionOf reports the app-wide direction for an object not yet on a
	// canvas, so the label is filled once the tab is showing.
	showWhere := func() {
		where.SetText(fmt.Sprintf("DirectionOf(entry) = %v", directionName(container.DirectionOf(scoped))))
	}
	where.SetText("Press \"Report direction\" once this tab shows")
	report := widget.NewButton("Report direction", showWhere)

	windowRTL := false
	toggleWindow := widget.NewButton("Toggle this window's direction (RTL / auto)", func() {
		c, ok := w.Canvas().(interface{ SetDirection(bellui.Direction) })
		if !ok {
			dialog.ShowInformation("Not supported", "This driver's canvas has no SetDirection.", w)
			return
		}
		windowRTL = !windowRTL
		if windowRTL {
			c.SetDirection(bellui.DirectionRTL)
		} else {
			c.SetDirection(bellui.DirectionAuto)
		}
		showWhere()
	})
	appRTL := false
	toggleApp := widget.NewButton("Toggle the app's direction (RTL / auto)", func() {
		appRTL = !appRTL
		if appRTL {
			bellui.SetLayoutDirection(bellui.DirectionRTL)
		} else {
			bellui.SetLayoutDirection(bellui.DirectionAuto)
		}
		showWhere()
	})
	toggleTheme := widget.NewButton("Toggle light / dark", func() {
		s := bellui.CurrentApp().Settings()
		if s.ThemeVariant() == theme.VariantDark {
			s.SetThemeVariant(theme.VariantLight)
		} else {
			s.SetThemeVariant(theme.VariantDark)
		}
		showWhere()
	})

	outside := widget.NewEntry()
	outside.SetPlaceHolder("Outside the scope (window direction)")
	return container.NewVScroll(container.NewVBox(toggleWindow, toggleApp, toggleTheme, report, outside, scope))
}

func directionName(d bellui.Direction) string {
	switch d {
	case bellui.DirectionRTL:
		return "right-to-left"
	case bellui.DirectionLTR:
		return "left-to-right"
	}
	return "auto"
}

// overrideTab puts collections inside a ThemeOverride with loud colours, so a
// row, separator or header still drawn in the app theme stands out.
func overrideTab() core.CanvasObject {
	items := make([]string, 300)
	for i := range items {
		items[i] = fmt.Sprintf("Row %d", i+1)
	}
	list := widget.NewList(
		func() int { return len(items) },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(items[i]) },
	)
	table := widget.NewTableWithHeaders(
		func() (int, int) { return 200, 6 },
		func() core.CanvasObject { return widget.NewLabel("cell 000") },
		func(id widget.TableCellID, o core.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("r%d c%d", id.Row, id.Col))
		},
	)
	tree := widget.NewTreeWithStrings(map[string][]string{
		"":      {"Fruit", "Vegetables"},
		"Fruit": {"Apple", "Banana", "Cherry", "Date", "Elderberry", "Fig", "Grape"},
		"Vegetables": {
			"Asparagus", "Broccoli", "Carrot", "Daikon", "Eggplant", "Fennel", "Garlic",
		},
	})
	tree.OpenAllBranches()
	inner := container.NewAppTabs(
		container.NewTabItem("List", list),
		container.NewTabItem("Table", table),
		container.NewTabItem("Tree", tree),
	)
	return container.NewThemeOverride(inner, loudTheme{theme.DefaultTheme()})
}

// loudTheme recolours the default theme so its rows cannot be mistaken for the
// app theme in either variant.
type loudTheme struct{ bellui.Theme }

func (t loudTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground, theme.ColorNameInputBackground:
		return color.NRGBA{R: 40, G: 10, B: 70, A: 255} // deep purple
	case theme.ColorNameForeground:
		return color.NRGBA{R: 255, G: 220, B: 0, A: 255} // yellow
	case theme.ColorNameSeparator, theme.ColorNameOutlineVariant, theme.ColorNamePrimary:
		return color.NRGBA{R: 255, G: 120, B: 0, A: 255} // orange
	case theme.ColorNameHover, theme.ColorNameSelection:
		return color.NRGBA{R: 0, G: 160, B: 140, A: 255} // teal
	}
	return t.Theme.Color(n, v)
}

// menusTab opens a pop-up menu with every part a menu item can have. The
// window's main menu (desktop) carries the same items.
func menusTab(w bellui.Window) core.CanvasObject {
	var open *widget.Button
	open = widget.NewButton("Open the pop-up menu", func() {
		pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(open)
		widget.ShowPopUpMenuAtPosition(demoMenu(""), w.Canvas(), pos.Add(geo.NewPos(0, open.Size().Height)))
	})
	return container.NewVBox(
		widget.NewLabel("Open the menu, then switch to right-to-left on the Direction tab and open it again."),
		open,
	)
}

func demoMenu(label string) *bellui.Menu {
	checked := bellui.NewMenuItem("Checked item", nil)
	checked.Checked = true
	withIcon := bellui.NewMenuItemWithIcon("Item with icon", theme.DocumentSaveIcon(), func() {})
	withShortcut := bellui.NewMenuItem("Item with shortcut", func() {})
	withShortcut.Shortcut = &bellui.ShortcutCopy{}
	sub := bellui.NewMenuItem("Submenu", nil)
	sub.ChildMenu = bellui.NewMenu("", bellui.NewMenuItem("Child one", func() {}), bellui.NewMenuItem("Child two", func() {}))
	disabled := bellui.NewMenuItem("Disabled item", func() {})
	disabled.Disabled = true
	return bellui.NewMenu(label, checked, withIcon, withShortcut, bellui.NewMenuItemSeparator(), sub, disabled)
}

// touchTab holds the controls whose touch behaviour changed in v1.3.0, and a
// focusable target that shows the last key it received.
func touchTab() core.CanvasObject {
	value := widget.NewLabel("Slider value: 50.0")
	slider := widget.NewSlider(0, 100)
	slider.SetValue(50)
	slider.OnChanged = func(v float64) { value.SetText(fmt.Sprintf("Slider value: %.1f", v)) }

	pages := []core.CanvasObject{carouselPage(1), carouselPage(2), carouselPage(3)}
	carousel := widget.NewCarousel(pages...)
	pageLabel := widget.NewLabel("Carousel page 1 of 3")
	carousel.OnChanged = func(i int) { pageLabel.SetText(fmt.Sprintf("Carousel page %d of %d", i+1, len(carousel.Items))) }
	addPage := widget.NewButton("Add a carousel page", func() {
		carousel.Items = append(carousel.Items, carouselPage(len(carousel.Items)+1))
		carousel.Refresh()
		pageLabel.SetText(fmt.Sprintf("Carousel page %d of %d", carousel.CurrentItem+1, len(carousel.Items)))
	})

	rows := make([]string, 200)
	for i := range rows {
		rows[i] = fmt.Sprintf("Scroll row %d", i+1)
	}
	list := widget.NewList(
		func() int { return len(rows) },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(rows[i]) },
	)
	listBox := container.NewGridWrap(geo.NewSize(320, 220), list)

	return container.NewVScroll(container.NewVBox(
		value, slider,
		widget.NewSeparator(),
		pageLabel, container.NewGridWrap(geo.NewSize(320, 120), carousel), addPage,
		widget.NewSeparator(),
		widget.NewLabel("Flick this list:"), listBox,
		widget.NewSeparator(),
		newKeyTarget(),
	))
}

func carouselPage(n int) core.CanvasObject {
	return widget.NewCard(fmt.Sprintf("Page %d", n), "Swipe or flick sideways", nil)
}

// keyTarget is a focusable widget that takes keys but no text, like a chart:
// it shows the last key and rune it received (#666).
type keyTarget struct {
	widget.BaseWidget
	label   *widget.Label
	focused bool
}

func newKeyTarget() *keyTarget {
	k := &keyTarget{label: widget.NewLabel("Key target: tap or click here, then press keys")}
	k.ExtendBaseWidget(k)
	return k
}

func (k *keyTarget) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewCard("Key target", "", k.label))
}

func (k *keyTarget) Tapped(*event.PointEvent) {
	if c := bellui.CurrentApp().Driver().CanvasForObject(k); c != nil {
		c.Focus(k)
	}
}

func (k *keyTarget) FocusGained() {
	k.focused = true
	k.label.SetText("Key target: focused, press a key")
}
func (k *keyTarget) FocusLost() { k.focused = false; k.label.SetText("Key target: not focused") }
func (k *keyTarget) TypedRune(r rune) {
	k.label.SetText(fmt.Sprintf("Key target: rune %q", r))
}

func (k *keyTarget) TypedKey(e *event.KeyEvent) {
	k.label.SetText(fmt.Sprintf("Key target: key %s", e.Name))
}
