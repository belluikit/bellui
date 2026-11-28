// Command scrollprobe live-verifies pointer scrolling on a touch device
// (#1510): whether a mouse wheel or a trackpad's two-finger scroll reaches
// bellui on Android and iPadOS, and in what form.
//
// The top half is a pad that logs every event it is given, one
// machine-greppable line each (logcat tag Bellui on Android):
//
//	scrollprobe ready scale=2.62 pad=0,0+412x400pt list=0,400+412x400pt
//	scrollprobe scrolled src=wheel phase=unknown dx=0 dy=-40 at=200,180
//	scrollprobe scroll-end
//	scrollprobe dragged dx=0 dy=-12 at=200,168
//	scrollprobe drag-end
//	scrollprobe tap at=200,180
//	scrollprobe long-press at=200,180
//	scrollprobe pinched scale=1.040 at=200,180
//	scrollprobe pinch-end
//
// The bottom half is a plain vertical scroll of 60 rows; it logs its offset
// as it moves (`scrollprobe list y=…`), so a scroll the driver delivers
// visibly scrolls real content too.
//
// On Android, drive it with the two scripts beside it, which plug a virtual
// USB mouse or a virtual multitouch touchpad in through the uinput shell
// command (no root; Android 14's `input` has no mouse-scroll command):
//
//	adb push mouse.sh touchpad.sh /data/local/tmp/
//	adb logcat -c; adb shell sh /data/local/tmp/mouse.sh     # hover, wheel, click
//	adb logcat -c; adb shell sh /data/local/tmp/touchpad.sh  # two-finger swipes, a pinch
//	adb logcat -d -s Bellui:I MS:I TP:I
//
// Expected since #1510: a hover logs nothing; each wheel notch logs one
// `scrolled src=wheel phase=unknown` over the pad and scrolls the list one
// step; each two-finger swipe logs `phase=began`, then `phase=changed` steps,
// then `scroll-end`; the pinch logs `pinched` steps and `pinch-end`. The
// mouse starts in the middle of the screen; the scripts move it in pixels
// for a 1080x2400 screen, the pixel143 AVD's.
//
// It quits by itself after ten minutes.
//
// Android: package with `bellui package --target android/arm64 --app-id
// com.trendvidia.scrollprobe` from this directory.
package main

import (
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("scrollprobe")

	label := widget.NewLabel("scrollprobe: scroll, drag or tap the pad")
	p := newPad(label)

	rows := make([]core.CanvasObject, 60)
	for i := range rows {
		rows[i] = widget.NewLabel(fmt.Sprintf("row %d", i))
	}
	list := container.NewVScroll(container.NewVBox(rows...))
	list.OnScrolled = func(o geo.Position) {
		fmt.Printf("scrollprobe list y=%.0f\n", o.Y)
	}

	w.SetContent(container.NewGridWithRows(2, p, container.NewBorder(label, nil, nil, nil, list)))
	w.Resize(geo.NewSize(412, 800))

	go func() {
		time.Sleep(2 * time.Second)
		bellui.Do(func() {
			c := w.Canvas()
			pp := bellui.CurrentApp().Driver().AbsolutePositionForObject(p)
			lp := bellui.CurrentApp().Driver().AbsolutePositionForObject(list)
			fmt.Printf("scrollprobe ready scale=%.3f pad=%.0f,%.0f+%.0fx%.0fpt list=%.0f,%.0f+%.0fx%.0fpt\n",
				c.Scale(), pp.X, pp.Y, p.Size().Width, p.Size().Height,
				lp.X, lp.Y, list.Size().Width, list.Size().Height)
		})
		time.Sleep(10 * time.Minute)
		fmt.Println("scrollprobe timeout")
		os.Exit(0)
	}()

	bellui.ShowAndRun(w)
}

// pad logs every pointer event it receives.
type pad struct {
	widget.BaseWidget
	label *widget.Label
	n     int
}

var (
	_ event.ScrollEnder   = (*pad)(nil)
	_ event.Draggable     = (*pad)(nil)
	_ event.Tappable      = (*pad)(nil)
	_ event.LongPressable = (*pad)(nil)
	_ event.Pinchable     = (*pad)(nil)
)

func newPad(label *widget.Label) *pad {
	p := &pad{label: label}
	p.ExtendBaseWidget(p)
	return p
}

func (p *pad) log(format string, args ...any) {
	p.n++
	s := fmt.Sprintf(format, args...)
	fmt.Println("scrollprobe " + s)
	p.label.SetText(fmt.Sprintf("%d: %s", p.n, s))
}

func (p *pad) Scrolled(e *event.ScrollEvent) {
	p.log("scrolled src=%s phase=%s dx=%.1f dy=%.1f at=%.0f,%.0f",
		sourceName(e.Source), phaseName(e.Phase), e.Scrolled.DX, e.Scrolled.DY, e.Position.X, e.Position.Y)
}

func (p *pad) ScrollEnded() { p.log("scroll-end") }

func (p *pad) Dragged(e *event.DragEvent) {
	p.log("dragged dx=%.1f dy=%.1f at=%.0f,%.0f", e.Dragged.DX, e.Dragged.DY, e.Position.X, e.Position.Y)
}

func (p *pad) DragEnd() { p.log("drag-end") }

func (p *pad) Tapped(e *event.PointEvent) { p.log("tap at=%.0f,%.0f", e.Position.X, e.Position.Y) }

func (p *pad) LongPressed(e *event.PointEvent) {
	p.log("long-press at=%.0f,%.0f", e.Position.X, e.Position.Y)
}

func (p *pad) Pinched(e *event.PinchEvent) {
	p.log("pinched scale=%.3f at=%.0f,%.0f", e.Scale, e.Position.X, e.Position.Y)
}

func (p *pad) PinchEnded() { p.log("pinch-end") }

func (p *pad) CreateRenderer() core.WidgetRenderer {
	bg := canvas.NewRectangle(color.NRGBA{R: 0x30, G: 0x50, B: 0x90, A: 0xff})
	return widget.NewSimpleRenderer(bg)
}

func sourceName(s event.ScrollSource) string {
	switch s {
	case event.ScrollSourceMouseWheel:
		return "wheel"
	case event.ScrollSourceTouchpad:
		return "touchpad"
	case event.ScrollSourceTouch:
		return "touch"
	default:
		return "unknown"
	}
}

func phaseName(ph event.ScrollPhase) string {
	switch ph {
	case event.ScrollPhaseBegan:
		return "began"
	case event.ScrollPhaseChanged:
		return "changed"
	case event.ScrollPhaseMomentum:
		return "momentum"
	default:
		return "unknown"
	}
}
