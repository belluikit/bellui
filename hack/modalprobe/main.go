// Command modalprobe live-verifies native window-modality for window-hosted
// dialogs (pre-rebrand tracker #498/#681). It opens a parent window with interactive content, then
// a window-hosted modal dialog over it. While the dialog is up the OS should
// block the parent: on Windows its title bar greys and it rejects input
// (EnableWindow), on X11 the dialog carries _NET_WM_STATE_MODAL, on Wayland an
// xdg_dialog_v1 is set modal (compositor-dependent), and on macOS the parent
// cannot become key (canBecomeKeyWindow=NO).
//
// Usage:
//
//	go run ./hack/modalprobe            # open the modal automatically after show
//	go run ./hack/modalprobe -manual    # wait for the button tap instead
//	go run ./hack/modalprobe -stacked   # open two modals over one parent
//
// The parent window is titled "modalprobe-parent" and each dialog window
// "modalprobe-dialog-N" so an introspector (xprop, IsWindowEnabled, AX) can
// target them by title.
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/dialog"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	manual := flag.Bool("manual", false, "open the modal on a button tap instead of automatically")
	stacked := flag.Bool("stacked", false, "open two stacked modal dialogs over one parent")
	checkwin := flag.Bool("checkwin", false, "Windows: assert the parent HWND is disabled while modal, then exit (headless self-check)")
	flag.Parse()

	a := app.New()
	w := a.NewWindow("modalprobe-parent")

	status := widget.NewLabel("parent window — you should NOT be able to type/click here while a modal dialog is open")
	entry := widget.NewEntry()
	entry.SetPlaceHolder("try typing here while the dialog is open (should be blocked)")
	tapped := 0
	btn := widget.NewButton("parent button (taps should be blocked while modal)", func() {
		tapped++
		status.SetText(fmt.Sprintf("parent button tapped %d times — if this rises while the dialog is open, the parent is NOT blocked", tapped))
	})

	n := 0
	openModal := func() {
		n++
		title := fmt.Sprintf("modalprobe-dialog-%d", n)
		d := dialog.NewCustom(title, "Close", widget.NewLabel("This is a window-hosted MODAL dialog.\nThe parent window behind it must be blocked by the OS."), w)
		d.SetWindowHosted(true)
		d.Show()
		fmt.Printf("opened window-hosted modal %q over parent %q\n", title, "modalprobe-parent")
	}

	open := widget.NewButton("open window-hosted modal", openModal)
	w.SetContent(container.NewVBox(status, entry, btn, open))
	w.Resize(geo.NewSize(520, 260))

	switch {
	case *checkwin:
		go runWindowsCheck(a, w)
	case !*manual:
		// Open shortly after the window maps so the parent native handle exists.
		go func() {
			time.Sleep(600 * time.Millisecond)
			bellui.Do(func() {
				openModal()
				if *stacked {
					openModal()
				}
			})
		}()
	}

	w.Show()
	a.Run()
}
