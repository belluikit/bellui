package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/widget"
)

// chipsTab holds a chip group with the excluded state (#1383), a selected
// chip's check mark (#1364), row alignment (#1371), the wrapping and flex
// layouts under right-to-left (#1372), and a stroked SVG at two sizes
// (#1292).
func chipsTab() core.CanvasObject {
	return container.NewVScroll(container.NewVBox(
		excludableCard(),
		alignmentCard(),
		directionCard(),
		strokeCard(),
	))
}

// excludableCard is a multiple-choice group whose chips cycle off →
// selected → excluded → off, reporting every change, and refusing to
// exclude the last style the way DaySong does.
func excludableCard() core.CanvasObject {
	status := widget.NewLabel("")
	status.Wrapping = bellui.TextWrapWord
	g := widget.NewChipGroup([]string{"Rock", "Jazz", "Pop", "Folk"}, true, nil)
	g.Excludable = true
	last := "nothing yet"
	show := func() {
		status.SetText(fmt.Sprintf("Selected: %s\nExcluded: %s\nLast change: %s",
			listOrNone(g.Selected), listOrNone(g.Excluded), last))
	}
	g.OnStateChanged = func(opt string, s widget.ChipState) {
		if s == widget.ChipStateExcluded && len(g.Excluded) == len(g.Options) {
			g.SetExcludedSilently(slices.DeleteFunc(slices.Clone(g.Excluded), func(e string) bool { return e == opt }))
			g.SetSelectedSilently(append(slices.Clone(g.Selected), opt))
			last = opt + " stays selected: the last style cannot be excluded"
		} else {
			last = opt + " is now " + chipStateName(s)
		}
		show()
	}
	show()
	disable := widget.NewCheck("Disable the group", func(on bool) {
		if on {
			g.Disable()
		} else {
			g.Enable()
		}
	})

	single := widget.NewChipGroup([]string{"Small", "Medium", "Large"}, false, nil)
	single.Excludable = true // does nothing without Multiple
	return widget.NewCard("Chips: selected and excluded", "Tap a chip to move it off → selected → excluded → off",
		container.NewVBox(g, status, disable,
			widget.NewLabel("One at a time (Excludable is ignored without Multiple):"), single))
}

func chipStateName(s widget.ChipState) string {
	switch s {
	case widget.ChipStateSelected:
		return "selected"
	case widget.ChipStateExcluded:
		return "excluded"
	}
	return "off"
}

func listOrNone(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return strings.Join(l, ", ")
}

// alignmentCard wraps a chip group over several rows in a narrow box, with a
// choice of where the rows sit.
func alignmentCard() core.CanvasObject {
	g := widget.NewChipGroup([]string{"Ocean", "Bugs and plants", "Dinosaurs", "Space", "Animals of the farm", "Trains"}, true, nil)
	pick := widget.NewRadioGroup([]string{"Start", "Center", "End"}, func(s string) {
		g.Alignment = map[string]bellui.TextAlign{
			"Start": bellui.TextAlignLeading, "Center": bellui.TextAlignCenter, "End": bellui.TextAlignTrailing,
		}[s]
		g.Refresh()
	})
	pick.Horizontal = true
	pick.SetSelected("Start")
	return widget.NewCard("Chip rows: start, center, end", "",
		container.NewVBox(pick, container.NewGridWrap(geo.NewSize(300, 170), g)))
}

// directionCard puts a row-wrap flow, a flex row and a flex column next to a
// switch for the app's direction, numbered so their order is easy to read.
func directionCard() core.CanvasObject {
	numbered := func(n int) []core.CanvasObject {
		out := make([]core.CanvasObject, n)
		for i := range out {
			out[i] = widget.NewButton(fmt.Sprintf("%d", i+1), nil)
		}
		return out
	}
	flow := container.New(layout.NewRowWrapLayout(), numbered(9)...)
	row := container.New(layout.NewFlexHBoxLayout([]float32{0, 1, 0}),
		widget.NewButton("1 first", nil), widget.NewButton("2 grows", nil), widget.NewButton("3 last", nil))
	col := layout.NewFlexVBoxLayout(nil)
	col.CrossAlign = layout.CrossStart
	column := container.New(col, widget.NewButton("Start-aligned", nil), widget.NewButton("Also start-aligned", nil))

	rtl := false
	toggle := widget.NewButton("Toggle the app's direction (RTL / auto)", func() {
		rtl = !rtl
		if rtl {
			bellui.SetLayoutDirection(bellui.DirectionRTL)
		} else {
			bellui.SetLayoutDirection(bellui.DirectionAuto)
		}
	})
	return widget.NewCard("Wrapping and flex layouts", "Numbered so the order shows; toggle the direction",
		container.NewVBox(toggle,
			widget.NewLabel("Row-wrap flow:"), container.NewGridWrap(geo.NewSize(300, 130), flow),
			widget.NewLabel("Flex row (the middle one grows):"), row,
			widget.NewLabel("Flex column, CrossStart:"), column))
}

// strokeSVG is a ring with a 7-unit stroke on a 120-unit viewBox, the case
// #1292 measured: about 2.3 px wide at 40 px and 14 px at 240 px.
const strokeSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120">
<circle cx="60" cy="60" r="45" fill="none" stroke="#e65100" stroke-width="7"/>
<line x1="20" y1="110" x2="100" y2="110" stroke="#1565c0" stroke-width="7" stroke-dasharray="14 10"/>
</svg>`

// strokeCard draws the same stroked SVG small and large.
func strokeCard() core.CanvasObject {
	res := bellui.NewStaticResource("stroke.svg", []byte(strokeSVG))
	img := func(px float32) core.CanvasObject {
		i := canvas.NewImageFromResource(res)
		i.FillMode = canvas.ImageFillContain
		return container.NewGridWrap(geo.NewSquareSize(px), i)
	}
	return widget.NewCard("SVG strokes scale", "The same ring at 40 and 240 px",
		container.NewHBox(img(40), img(240)))
}
