// Command boundaryverify draws the RepaintBoundary scenes the boundary fixes
// were found on (#559, #564, #565), side by side, while it scrolls them and
// changes their content every frame:
//
//   - left: a boundary holding a scroll
//   - middle: a scroll of boundary-wrapped cards (-variant plain wraps a plain
//     VBox instead, -variant noboundary leaves the cards unwrapped)
//   - right: a scroll holding a boundary holding a scroll
//
// -shot saves a capture at frame -shotframe, to compare painters or variants:
//
//	go run ./hack/boundaryverify -shot /tmp/webgpu.png -shotframe 61
//	go run -tags gl ./hack/boundaryverify -shot /tmp/gl.png -shotframe 61
//
// Built with -tags partialrepaint and run with BELLUI_PARTIAL_VERIFY=1, it is
// also a partial-repaint check over boundaries (#496).
package main

import (
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func labels(prefix string, n int) (*bellui.Container, []*widget.Label) {
	ls := make([]*widget.Label, n)
	objs := make([]core.CanvasObject, n)
	for i := range ls {
		ls[i] = widget.NewLabel(prefix + " " + strconv.Itoa(i))
		objs[i] = ls[i]
	}
	return container.NewVBox(objs...), ls
}

func main() {
	secs := flag.Float64("secs", 20, "seconds to run")
	shot := flag.String("shot", "", "save a capture here at -shotframe")
	shotFrame := flag.Int("shotframe", 80, "frame to capture at")
	variant := flag.String("variant", "card", "column 2: card | plain | noboundary | shadow | shadownoboundary")
	dump := flag.Bool("dump", false, "print the middle column's geometry at -shotframe")
	noBar := flag.Bool("nobar", false, "leave out the infinite progress bar, which moves with wall time")
	light := flag.Bool("light", false, "use the light theme variant, where shadows show")
	frac := flag.Bool("frac", false, "scroll by fractional amounts, as a touchpad does")
	flag.Parse()
	a := app.New()
	if *light {
		a.Settings().SetThemeVariant(theme.VariantLight)
	}
	w := a.NewWindow("boundaryverify")

	status := widget.NewLabel("frame 0")

	// 1. A boundary holding a scroll.
	c1, l1 := labels("b>s", 40)
	s1 := container.NewVScroll(c1)
	b1 := container.NewRepaintBoundary(s1)

	// 2. A scroll of boundary-wrapped cards, one with a spinner.
	cards := make([]core.CanvasObject, 12)
	var cardLabels []*widget.Label
	for i := range cards {
		l := widget.NewLabel("card " + strconv.Itoa(i))
		cardLabels = append(cardLabels, l)
		body := container.NewVBox(l)
		if i == 1 && !*noBar {
			body.Add(widget.NewProgressBarInfinite())
		}
		switch *variant {
		case "plain", "plainnoboundary":
			bg := canvas.NewRectangle(color.NRGBA{R: 40, G: 40, B: 60, A: 255})
			bg.CornerRadius = 12
			bg.StrokeColor = color.NRGBA{R: 200, G: 200, B: 220, A: 255}
			bg.StrokeWidth = 1
			var o core.CanvasObject = container.NewStack(bg, container.NewVBox(widget.NewLabel("Plain "+strconv.Itoa(i)), body))
			if *variant == "plain" {
				o = container.NewRepaintBoundary(o)
			}
			cards[i] = o
		case "shadow", "shadownoboundary":
			// A shadow blurred past its owner's bounds: 2×BlurRadius = 16 pt.
			sh := canvas.NewBoxShadow(color.NRGBA{A: 160}, 8)
			sh.OffsetY = 4
			box := canvas.NewRectangle(color.NRGBA{R: 230, G: 230, B: 240, A: 255})
			var o core.CanvasObject = container.NewPadded(container.NewStack(sh, box, container.NewVBox(widget.NewLabel("Shadow "+strconv.Itoa(i)), body)))
			if *variant == "shadow" {
				o = container.NewRepaintBoundary(o)
			}
			cards[i] = o
		case "noboundary":
			cards[i] = widget.NewCard("Card "+strconv.Itoa(i), "", body)
		default:
			cards[i] = container.NewRepaintBoundary(widget.NewCard("Card "+strconv.Itoa(i), "", body))
		}
	}
	s2 := container.NewVScroll(container.NewVBox(cards...))

	// 3. A scroll holding a boundary holding a scroll.
	c3, l3 := labels("s>b>s", 30)
	inner := container.NewVScroll(c3)
	inner.SetMinSize(geo.NewSize(200, 150))
	s3outer := container.NewVScroll(container.NewVBox(
		widget.NewLabel("above"), container.NewRepaintBoundary(inner), widget.NewLabel("below"),
		widget.NewLabel("more"), widget.NewLabel("more"), widget.NewLabel("more"), widget.NewLabel("more"),
	))

	grid := container.NewGridWithColumns(3, b1, s2, s3outer)
	w.SetContent(container.NewBorder(status, nil, nil, nil, grid))
	w.Resize(geo.NewSize(900, 500))

	go func() {
		time.Sleep(time.Second)
		start := time.Now()
		saved := false
		for f := 0; time.Since(start).Seconds() < *secs; f++ {
			bellui.DoAndWait(func() {
				status.SetText(fmt.Sprintf("frame %d", f))
				step := float32((f%40)-20) * 6
				if *frac {
					step *= 1.0371
				}
				if step < 0 {
					step = -step
				}
				s1.ScrollToOffset(geo.NewPos(0, step*4))
				s2.ScrollToOffset(geo.NewPos(0, step*8))
				s3outer.ScrollToOffset(geo.NewPos(0, step))
				inner.ScrollToOffset(geo.NewPos(0, step*3))
				if f%3 == 0 {
					l1[f%len(l1)].SetText(fmt.Sprintf("b>s %d @%d", f%len(l1), f))
					cardLabels[f%len(cardLabels)].SetText(fmt.Sprintf("card %d @%d", f%len(cardLabels), f))
					l3[f%len(l3)].SetText(fmt.Sprintf("s>b>s %d @%d", f%len(l3), f))
				}
			})
			if *dump && f == *shotFrame {
				bellui.DoAndWait(func() {
					fmt.Printf("DUMP frame %d: scroll offset %v, scroll size %v, content size %v, content min %v\n",
						f, s2.Offset, s2.Size(), s2.Content.Size(), s2.Content.MinSize())
					for i, c := range cards {
						p := bellui.CurrentApp().Driver().AbsolutePositionForObject(c)
						fmt.Printf("DUMP card %d: rel %v abs %v size %v visible %v\n", i, c.Position(), p, c.Size(), c.Visible())
					}
				})
			}
			if *shot != "" && !saved && f == *shotFrame {
				saved = true
				bellui.DoAndWait(func() {
					img := w.Canvas().Capture()
					f, _ := os.Create(*shot)
					png.Encode(f, img)
					f.Close()
				})
			}
			time.Sleep(30 * time.Millisecond)
		}
		bellui.DoAndWait(a.Quit)
	}()
	bellui.ShowAndRun(w)
}
