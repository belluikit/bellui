package main

import (
	"fmt"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/widget"
)

// swipesTab is a scrolling page with a widget of every kind that can take a
// touch drag inside it, so a tester can check which one keeps a swipe and
// which hands it to the page (#662, #801, #986, #1229, #1266), plus the
// wheel picker and swipe card (#977, #989).
func swipesTab() core.CanvasObject {
	box := func(w, h float32, o core.CanvasObject) core.CanvasObject {
		return container.NewGridWrap(geo.NewSize(w, h), o)
	}
	value := widget.NewLabel("Slider value: 50")
	slider := widget.NewSlider(0, 100)
	slider.SetValue(50)
	slider.OnChanged = func(v float64) { value.SetText(fmt.Sprintf("Slider value: %.0f", v)) }
	off := widget.NewSlider(0, 100)
	off.SetValue(50)
	off.Disable()

	var chips []core.CanvasObject
	for i := 1; i <= 20; i++ {
		chips = append(chips, widget.NewChip(fmt.Sprintf("Chip %d", i), nil))
	}
	sideways := container.NewHScroll(container.NewHBox(chips...))

	rows := make([]string, 60)
	for i := range rows {
		rows[i] = fmt.Sprintf("Inner list row %d", i+1)
	}
	list := widget.NewList(
		func() int { return len(rows) },
		func() core.CanvasObject { return widget.NewLabel("template") },
		func(i widget.ListItemID, o core.CanvasObject) { o.(*widget.Label).SetText(rows[i]) },
	)
	table := widget.NewTableWithHeaders(
		func() (int, int) { return 40, 6 },
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
	moved := widget.NewLabel("Moved: nothing yet")
	reorder := newTreeView(true)
	reorder.OnMoved = func(id, parent string, index int) {
		moved.SetText(fmt.Sprintf("Moved: %s to position %d of %s", id, index+1, parent))
	}

	text := widget.NewLabel(strings.Repeat("Selectable text: a finger drag over it scrolls the page, a long-press selects a word. ", 3))
	text.Selectable = true
	text.Wrapping = bellui.TextWrapWord
	multi := widget.NewMultiLineEntry()
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("Line %d of a multi-line entry", i+1)
	}
	multi.SetText(strings.Join(lines, "\n"))

	picked := widget.NewLabel("Wheel: Item 1")
	var items []core.CanvasObject
	for i := 1; i <= 20; i++ {
		items = append(items, widget.NewLabelWithStyle(fmt.Sprintf("Item %d", i), bellui.TextAlignCenter, bellui.TextStyle{}))
	}
	wheel := widget.NewWheelPicker(items, func(i int) { picked.SetText(fmt.Sprintf("Wheel: Item %d", i+1)) })
	swiped := widget.NewLabel("Card: not swiped yet")
	card := widget.NewSwipeArea(widget.NewCard("Swipe card", "Drag it sideways and let go", nil), nil)
	card.Follow = true
	card.OnSwiped = func(d widget.SwipeDirection) {
		swiped.SetText(fmt.Sprintf("Card: swiped %s", d))
		card.Content = widget.NewCard("Swipe card", "A new card: swipe it too", nil)
		card.Refresh()
	}

	// A second card treats each way differently (#1103): left throws it,
	// right springs it back, up and down leave it at rest.
	moded := widget.NewLabel("Directions card: not swiped yet")
	modes := widget.NewSwipeArea(widget.NewCard("Directions card", "Left throws, right springs back, up or down does nothing", nil), nil)
	modes.Follow = true
	modes.Directions = map[widget.SwipeDirection]widget.SwipeCardMode{
		widget.SwipeRight: widget.SwipeStay, widget.SwipeUp: widget.SwipeIgnore, widget.SwipeDown: widget.SwipeIgnore,
	}
	n := 0
	modes.OnSwiped = func(d widget.SwipeDirection) {
		n++
		moded.SetText(fmt.Sprintf("Directions card: swiped %s (%d so far)", d, n))
		if d == widget.SwipeLeft {
			modes.Content = widget.NewCard("Directions card", "A new card: left throws, right springs back", nil)
			modes.Refresh()
		}
	}

	var filler []core.CanvasObject
	for i := 1; i <= 15; i++ {
		filler = append(filler, widget.NewLabel(fmt.Sprintf("Page row %d", i)))
	}
	parts := []core.CanvasObject{
		widget.NewLabel("Swipe up and down starting on each widget below."),
		value, slider,
		widget.NewLabel("A disabled slider:"), off,
		widget.NewLabel("Carousel:"), box(320, 120, widget.NewCarousel(carouselPage(1), carouselPage(2), carouselPage(3))),
		widget.NewLabel("A row that scrolls sideways:"), box(320, 56, sideways),
		widget.NewLabel("Inner list:"), box(320, 200, list),
		widget.NewLabel("Table:"), box(320, 200, table),
		widget.NewLabel("Tree:"), box(320, 200, tree),
		widget.NewLabel("Tree view, rows fixed:"), box(320, 200, newTreeView(false)),
		widget.NewLabel("Tree view, rows can be dragged to a new place:"), box(320, 200, reorder), moved,
		text,
		widget.NewLabel("Multi-line entry:"), box(320, 120, multi),
		picked, box(200, 160, wheel),
		swiped, box(320, 120, card),
		moded, box(320, 120, modes),
	}
	return container.NewVScroll(container.NewVBox(append(parts, filler...)...))
}

// textTab holds the text a long-press selects a word of (#1267), and the
// layouts a wrapping label used to collapse in (#1099, #1195, #1198, #1199,
// #1202), with snackbars too long for one line (#1038, #1106).
func textTab(w bellui.Window) core.CanvasObject {
	press := widget.NewLabel("Long-press the word unwrapped in this sentence, then drag right across the words after it, and back.")
	press.Selectable = true
	press.Wrapping = bellui.TextWrapWord
	field := widget.NewEntry()
	field.SetText("an unwrapped word in a text field")

	wrap := func(s string) *widget.Label {
		l := widget.NewLabel(s)
		l.Wrapping = bellui.TextWrapWord
		return l
	}
	long := "This wrapping label is longer than the row it sits in, so it wraps over several lines."
	form := container.New(
		layout.NewFormLayout(),
		wrap("A form label long enough that it has to wrap"), widget.NewEntry(),
		widget.NewLabel("Short"), widget.NewEntry(),
	)
	flow := container.New(
		layout.NewRowWrapLayout(),
		widget.NewButton("One", nil), widget.NewButton("Two", nil), wrap(long), widget.NewButton("Three", nil),
	)
	french := wrap("Il a dit : « bonjour », puis il a marché 10 km jusqu’à la gare ; elle était fermée !")

	snack := "This snackbar message is far too long to fit on one line of a phone, so it has to wrap inside the bar."
	one := widget.NewButton("Show a long snackbar", func() {
		widget.ShowSnackbar(snack, "Undo", func() {}, w.Canvas())
	})
	two := widget.NewButton("Show two snackbars at once", func() {
		widget.ShowSnackbar("First snackbar", "", nil, w.Canvas())
		widget.ShowSnackbar("Second snackbar: only this one shows", "", nil, w.Canvas())
	})

	return container.NewVScroll(container.NewVBox(
		widget.NewCard("Long-press a word", "", container.NewVBox(press, field)),
		widget.NewCard("Wrapping text keeps its width", "", container.NewVBox(
			widget.NewLabel("Beside a button:"), container.NewHBox(wrap(long), widget.NewButton("Beside", nil)),
			widget.NewLabel("Centred:"), container.NewCenter(wrap(long)),
			widget.NewLabel("In a row-wrap flow:"), flow,
			widget.NewLabel("In a form's label column:"), form,
			widget.NewLabel("French with no-break spaces, in a narrow column:"), container.NewGridWrap(geo.NewSize(220, 140), french),
		)),
		widget.NewCard("Snackbars", "", container.NewVBox(one, two)),
	))
}
