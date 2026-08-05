// Command touchprobe live-verifies desktop touchscreen gesture dispatch
// (#266): the glfw fork's touch callback (belluikit/glfw#1, v1.10.0) feeds
// the shared TouchHandler, so a Windows (WM_POINTER) or Wayland (wl_touch)
// touchscreen reaches the mobile gesture vocabulary — tap, drag,
// double-tap, and hold-fire long-press (#758). On X11 and macOS the callback
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

	w.SetContent(container.NewBorder(
		widget.NewLabel("touchprobe: tap / hold / drag / double-tap the pad"),
		container.NewVBox(status, openMulti), nil, nil, pad,
	))
	w.Resize(geo.NewSize(500, 500))
	bellui.ShowAndRun(w)
}
