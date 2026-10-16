// Command clipimageprobe live-verifies mobile image clipboard (pre-rebrand tracker #580,
// bellui.ImageClipboard on Android/iOS): a generated pattern is copied to the
// system clipboard with SetClipboardImage, then read back with ClipboardImage
// and displayed — a full round-trip through the OS ClipboardManager /
// UIPasteboard (not an in-memory shortcut). "Paste" also works for an image
// copied from another app.
package main

import (
	"fmt"
	"image"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// makePattern builds a distinctive 96x96 image so a paste is visually
// identifiable: a magenta→cyan diagonal gradient with a yellow square marker.
func makePattern() image.Image {
	const n = 96
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			t := float64(x+y) / float64(2*n)
			r := uint8(255 * (1 - t))
			b := uint8(255 * t)
			g := uint8(40 + 160*t)
			if x >= 20 && x < 44 && y >= 20 && y < 44 {
				r, g, b = 255, 220, 40 // marker
			}
			img.Set(x, y, color.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func main() {
	a := app.NewWithID("io.bellui.clipimageprobe")
	w := a.NewWindow("Image Clipboard Probe (#580)")

	status := widget.NewLabel("Tap Copy, then Paste. Paste should show the pattern.")
	status.Wrapping = bellui.TextWrapWord

	src := makePattern()
	srcImg := canvas.NewImageFromImage(src)
	srcImg.FillMode = canvas.ImageFillContain
	srcImg.SetMinSize(geo.NewSize(96, 96))

	pasted := canvas.NewRectangle(color.Gray{Y: 40})
	pasted.SetMinSize(geo.NewSize(96, 96))
	pastedBox := container.NewStack(pasted)

	copyBtn := widget.NewButton("Copy image → clipboard", func() {
		ok := bellui.SetClipboardImage(src)
		status.SetText(fmt.Sprintf("SetClipboardImage supported=%v", ok))
	})
	pasteBtn := widget.NewButton("Paste image ← clipboard", func() {
		img, ok := bellui.ClipboardImage()
		if !ok || img == nil {
			status.SetText("ClipboardImage: no image (ok=false)")
			pastedBox.Objects = []core.CanvasObject{pasted}
			pastedBox.Refresh()
			return
		}
		b := img.Bounds()
		ci := canvas.NewImageFromImage(img)
		ci.FillMode = canvas.ImageFillContain
		ci.SetMinSize(geo.NewSize(96, 96))
		pastedBox.Objects = []core.CanvasObject{ci}
		pastedBox.Refresh()
		status.SetText(fmt.Sprintf("ClipboardImage OK — %dx%d", b.Dx(), b.Dy()))
	})

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("Image clipboard round-trip (#580)", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		status,
		widget.NewSeparator(),
		container.NewHBox(
			container.NewVBox(widget.NewLabel("Source:"), srcImg),
			container.NewVBox(widget.NewLabel("Pasted:"), pastedBox),
		),
		copyBtn,
		pasteBtn,
	))
	w.Resize(geo.NewSize(520, 480))
	bellui.ShowAndRun(w)
}
