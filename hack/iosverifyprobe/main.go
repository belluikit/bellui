// Command iosverifyprobe is a combined on-device verification harness for the
// iOS-device-dependent fixes landed together:
//
//   - #739 iOS SendNotification / SendNotificationWithOptions + SetBadge
//   - #742 iOS Hide-based multi-window pop-back (CAMetalLayer show/hide)
//   - #645 iOS Close-based multi-window pop-back (already landed via #741)
//   - #580 iOS clipboard image paste (ImageClipboard)
//
// On mobile there is no programmatic tap injection, so the flow self-drives on a
// timer and appends every result to an on-screen log (screenshots ARE the log).
// The multi-window steps also change which window covers the screen, so a
// screenshot taken during a step shows whether the child covers the parent or
// the parent was revealed (pop-back). The interactive buttons re-run each step.
package main

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func makeTestImage() image.Image {
	const w, h = 96, 96
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: 255})
		}
	}
	return img
}

func main() {
	a := app.NewWithID("com.trendvidia.iosverifyprobe")
	w := a.NewWindow("iOS Verify Probe")

	logLbl := widget.NewLabel("")
	logLbl.Wrapping = bellui.TextWrapWord
	var lines []string
	logf := func(format string, args ...any) {
		msg := time.Now().Format("15:04:05") + "  " + fmt.Sprintf(format, args...)
		lines = append(lines, msg)
		if len(lines) > 16 {
			lines = lines[len(lines)-16:]
		}
		text := ""
		for _, l := range lines {
			text += l + "\n"
		}
		bellui.Do(func() { logLbl.SetText(text) })
	}

	pasted := canvas.NewImageFromImage(makeTestImage())
	pasted.FillMode = canvas.ImageFillContain
	pasted.SetMinSize(geo.NewSize(72, 72))

	// #739 — notifications + badge.
	doNotify := func() {
		_, plain := a.(bellui.NotifierApp)
		_, rich := a.(bellui.NotifierWithOptionsApp)
		_, badge := a.(bellui.BadgeApp)
		logf("#739 caps: NotifierApp=%v WithOptions=%v BadgeApp=%v", plain, rich, badge)
		bellui.SendNotificationWithOptions(&bellui.NotificationOptions{
			Title:    "Bellui iOS",
			Content:  "Rich notification from the probe",
			Actions:  []bellui.NotificationAction{{Label: "Reply", Callback: func() { logf("#739 action fired") }}},
			OnTapped: func() { logf("#739 body tapped") },
		})
		if b, ok := a.(bellui.BadgeApp); ok {
			b.SetBadge(3)
			logf("#739 SetBadge(3) called (see app-icon badge on Home)")
		}
		logf("#739 notification requested (allow prompt if shown)")
	}

	// #580 — clipboard image round-trip.
	doClipboard := func() {
		ok := bellui.SetClipboardImage(makeTestImage())
		logf("#580 SetClipboardImage supported=%v", ok)
		img, got := bellui.ClipboardImage()
		if !got || img == nil {
			logf("#580 ClipboardImage read: NONE")
			return
		}
		b := img.Bounds()
		bellui.Do(func() { pasted.Image = img; pasted.Refresh() })
		logf("#580 ClipboardImage read back %dx%d — round-trip OK", b.Dx(), b.Dy())
	}

	// #645 — Close-based pop-back.
	openCloseChild := func() {
		child := a.NewWindow("Close Child")
		child.SetContent(container.NewVBox(
			widget.NewLabelWithStyle("CHILD (Close test)\ncovers the parent", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
			widget.NewButton("Close → pop back", func() { child.Close(); logf("#645 child Closed → parent revealed") }),
		))
		child.Resize(geo.NewSize(500, 360))
		child.Show()
		logf("#645 close-child shown (covers parent)")
		time.AfterFunc(3*time.Second, func() {
			bellui.Do(func() { child.Close(); logf("#645 child.Close() → parent should be revealed+repainted") })
		})
	}

	// #742 — Hide-based pop-back + re-show.
	openHideChild := func() {
		child := a.NewWindow("Hide Child")
		child.SetContent(container.NewVBox(
			widget.NewLabelWithStyle("CHILD (Hide test)\ncovers the parent", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
			widget.NewButton("Hide → pop back", func() { child.Hide(); logf("#742 child Hidden → parent revealed") }),
		))
		child.Resize(geo.NewSize(500, 360))
		child.Show()
		logf("#742 hide-child shown (covers parent)")
		time.AfterFunc(3*time.Second, func() {
			bellui.Do(func() { child.Hide(); logf("#742 child.Hide() → parent revealed (layer hidden, not removed)") })
		})
		time.AfterFunc(6*time.Second, func() {
			bellui.Do(func() { child.Show(); logf("#742 child.Show() → covers parent again (layer un-hidden+front)") })
		})
		time.AfterFunc(9*time.Second, func() {
			bellui.Do(func() { child.Close(); logf("#742 child.Close() → done, parent revealed") })
		})
	}

	w.SetContent(container.NewVBox(
		widget.NewLabelWithStyle("iOS device verification probe", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}),
		container.NewHBox(
			widget.NewButton("① Notify+Badge", doNotify),
			widget.NewButton("② Clipboard", doClipboard),
			widget.NewButton("③ Close child", openCloseChild),
			widget.NewButton("④ Hide child", openHideChild),
		),
		container.NewHBox(widget.NewLabel("clipboard img:"), pasted),
		widget.NewSeparator(),
		logLbl,
	))
	w.Resize(geo.NewSize(700, 900))

	// Self-drive: notify+badge → clipboard → close-child cycle → hide-child cycle.
	// Wide spacing so a screenshot lands cleanly inside each phase.
	time.AfterFunc(2*time.Second, func() { bellui.Do(doNotify) })
	time.AfterFunc(6*time.Second, func() { bellui.Do(doClipboard) })
	time.AfterFunc(10*time.Second, func() { bellui.Do(openCloseChild) }) // child covers ~10-13s, pops back at 13s
	time.AfterFunc(16*time.Second, func() { bellui.Do(openHideChild) })  // covers 16-19, hide 19-22, reshow 22-25, close 25
	logf("probe started — self-driving sequence")

	bellui.ShowAndRun(w)
}
