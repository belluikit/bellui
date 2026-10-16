// Command touchprobe live-verifies desktop touchscreen gesture dispatch
// (#266): the glfw fork's touch callback (belluikit/glfw#1, v1.10.0) feeds
// the shared TouchHandler, so a Windows (WM_POINTER) or Wayland (wl_touch)
// touchscreen reaches the mobile gesture vocabulary — tap, drag,
// double-tap, and hold-fire long-press (pre-rebrand tracker #758). On X11 and macOS the callback
// never fires and touch keeps arriving as an emulated mouse; the pad then
// logs mouse-path taps only, which is the expected outcome there.
//
// The gesture pad logs every dispatch with a counter to the label and
// stdout (`touchprobe <gesture>`). Expected on a touchscreen:
//
//	quick tap            → tap
//	press-and-hold ~300ms → long-press fires WHILE STILL DOWN; the release
//	                        then logs secondary (the hold-fire pattern)
//	drag                 → drag… then drag-end; no tap on release
//	double tap           → double-tap
//
// The list beside the pad verifies touch flick-scrolling (#291): a finger
// drag over it must scroll the content with the finger (`scroll y=…` lines),
// a flick must keep scrolling after the release (decaying offsets, stopped
// dead by touching again), and dragging the scrollbar thumb must still work.
// A MOUSE drag over the list must NOT scroll it — only the wheel does. On
// X11/macOS (emulated mouse) the list therefore does not touch-scroll, which
// is the expected outcome there.
//
// The "open-multiple dialog" button forces the canvas file dialog
// (UseNativeFileDialogs(false)) in open-multiple mode — the #259 acceptance:
// long-pressing a file item on a touchscreen must enter selection mode, and
// plain taps must then toggle set membership.
package main

import (
	"fmt"
	"log"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// gesturePad is a widget implementing the full gesture vocabulary, logging
// each dispatch.
type gesturePad struct {
	widget.BaseWidget
	rect *canvas.Rectangle
	logf func(string)
}

var (
	_ event.Tappable          = (*gesturePad)(nil)
	_ event.SecondaryTappable = (*gesturePad)(nil)
	_ event.DoubleTappable    = (*gesturePad)(nil)
	_ event.LongPressable     = (*gesturePad)(nil)
	_ event.Draggable         = (*gesturePad)(nil)
)

func (g *gesturePad) Tapped(*event.PointEvent)          { g.logf("tap") }
func (g *gesturePad) TappedSecondary(*event.PointEvent) { g.logf("secondary") }
func (g *gesturePad) DoubleTapped(*event.PointEvent)    { g.logf("double-tap") }
func (g *gesturePad) LongPressed(*event.PointEvent)     { g.logf("long-press") }
func (g *gesturePad) Dragged(*event.DragEvent)          { g.logf("drag") }
func (g *gesturePad) DragEnd()                          { g.logf("drag-end") }

func (g *gesturePad) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(g.rect)
}

func main() {
	a := app.New()
	w := a.NewWindow("touchprobe (#266)")

	status := widget.NewLabel("gestures: none yet — touch the pad")
	status.Wrapping = bellui.TextWrapWord
	count := 0
	logf := func(kind string) {
		count++
		line := fmt.Sprintf("%d: %s", count, kind)
		status.SetText("gestures: " + line)
		log.Println("touchprobe", line)
	}

	pad := &gesturePad{rect: canvas.NewRectangle(theme.Color(theme.ColorNamePrimary)), logf: logf}
	pad.ExtendBaseWidget(pad)
	pad.rect.SetMinSize(geo.NewSize(200, 200))

	openMulti := widget.NewButton("open-multiple dialog (#259 selection mode)", func() {
		dialog.UseNativeFileDialogs(false) // force the canvas dialog, the selection-mode host
		d := dialog.NewFileOpenMultiple(func(rs []bellui.URIReadCloser, err error) {
			log.Println("touchprobe picked", len(rs), "files, err:", err)
			for _, r := range rs {
				r.Close()
			}
		}, w)
		d.Show()
	})

	// Flick-scroll verification surface (#291): scrollable content beside the
	// pad, logging offsets so a flick's decay is visible in the output.
	items := make([]core.CanvasObject, 0, 60)
	for i := range 60 {
		items = append(items, widget.NewLabel(fmt.Sprintf("row %02d — drag/flick me", i)))
	}
	list := container.NewVScroll(container.NewVBox(items...))
	list.OnScrolled = func(off geo.Position) {
		log.Printf("touchprobe scroll y=%.1f", off.Y)
	}

	w.SetContent(container.NewBorder(
		widget.NewLabel("touchprobe: tap / hold / drag / double-tap the pad; drag/flick the list (#291)"),
		container.NewVBox(status, openMulti), nil, nil,
		container.NewGridWithColumns(2, pad, list),
	))
	w.Resize(geo.NewSize(500, 500))
	bellui.ShowAndRun(w)
}
