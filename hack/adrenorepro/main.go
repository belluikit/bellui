// Command adrenorepro reproduces pre-rebrand tracker #310 (Android Adreno/Vulkan WebGPU black canvas
// after first interaction). Matches the original harness: AppTabs across widgets,
// RichText, and an FBO-heavy effects tab (gradients + RepaintBoundary + Card) —
// the canvas-FBO / re-present path the issue implicated. Tab switching is the
// exact trigger.
package main

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("adrenorepro")

	// Widgets tab
	widgets := container.NewVBox(
		widget.NewButton("Button", func() {}),
		widget.NewCheck("Check", func(bool) {}),
		widget.NewSlider(0, 100),
		widget.NewProgressBar(),
		widget.NewEntry(),
		widget.NewLabel("widgets tab"),
	)

	// Effects tab: gradients + RepaintBoundary (FBO) + Card
	lin := canvas.NewLinearGradient(color.NRGBA{255, 40, 200, 255}, color.NRGBA{40, 200, 255, 255}, 45)
	lin.SetMinSize(geo.NewSize(300, 120))
	rad := canvas.NewRadialGradient(color.NRGBA{255, 220, 40, 255}, color.NRGBA{40, 40, 90, 255})
	rad.SetMinSize(geo.NewSize(300, 120))
	boundary := container.NewRepaintBoundary(widget.NewLabel("RepaintBoundary / FBO text"))
	card := widget.NewCard("Card title", "subtitle", widget.NewLabel("card body"))
	effects := container.NewVBox(lin, rad, boundary, card, widget.NewLabel("effects tab"))

	rich := widget.NewRichTextFromMarkdown("# RichText\n\nSome **bold** and *italic* text on the RichText tab.")

	tabs := container.NewAppTabs(
		container.NewTabItem("Widgets", widgets),
		container.NewTabItem("Effects", effects),
		container.NewTabItem("Rich", rich),
	)
	w.SetContent(tabs)
	w.Resize(geo.NewSize(400, 700))
	bellui.ShowAndRun(w)
}
