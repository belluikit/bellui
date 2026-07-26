package container_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func TestClip_Refresh(t *testing.T) {
	hello := widget.NewLabel("Hello Bellui!")
	clip := container.NewClip(hello)
	clip.Resize(geo.NewSize(28, 20))

	w := test.NewWindow(container.NewWithoutLayout(clip))
	w.Resize(geo.NewSquareSize(50))

	clip.Content = canvas.NewRectangle(color.White)
	clip.Refresh()
	test.AssertImageMatches(t, "clip/rect.png", w.Canvas().Capture().(*image.NRGBA))

	clip.Content = &widget.Button{Text: "Bye", Importance: widget.HighImportance}
	clip.Refresh()
	test.AssertImageMatches(t, "clip/button.png", w.Canvas().Capture().(*image.NRGBA))
}

func TestClip(t *testing.T) {
	hello := widget.NewLabel("Hello Bellui!")
	clip := container.NewClip(hello)
	clip.Resize(geo.NewSize(28, 20))

	w := test.NewWindow(container.NewWithoutLayout(clip))
	w.Resize(geo.NewSquareSize(50))
	test.AssertImageMatches(t, "clip/text.png", w.Canvas().Capture().(*image.NRGBA))
}
