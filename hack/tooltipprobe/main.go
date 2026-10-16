package main

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// pre-rebrand tracker #827 probe: long-press the button to reveal its tooltip on mobile.
func main() {
	a := app.New()
	w := a.NewWindow("tooltip probe")
	btn := widget.NewButton("Long-press me", func() {})
	btn.SetToolTipText("I am a long-press tooltip (#827)")
	lbl := widget.NewLabel("Press and hold the button ~0.5s")
	w.SetContent(container.NewVBox(lbl, btn))
	w.Resize(geo.NewSize(400, 300))
	bellui.ShowAndRun(w)
}
