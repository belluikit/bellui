// Command winposprobe reports where the OS/GLFW places bellui windows that never
// ask for a position (pre-rebrand tracker #955). Modes (first arg):
//
//	one <tag>  – create one window, report its position, exit (run it several
//	             times in a row to model a CI run's sequential test binaries,
//	             each of which is a fresh process)
//	multi      – create three concurrent windows and report all three
//
// Output appends to winposprobe.log in the OS temp directory, so it can run
// detached on an interactive desktop (on Windows CI/verify boxes, launch it
// via `schtasks /Create ... /IT` — an SSH session's window station places
// windows differently than the real desktop).
//
// Expected behavior: the first window of a process with no visible sibling is
// centered on every desktop platform — GLFW centers it on macOS, bellui seats it
// on Windows (pre-rebrand tracker #955), and X11/Wayland window managers apply their own policy.
// Additional concurrent windows cascade (Cocoa's cascade point on macOS, the
// OS default placement on Windows).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	logPath := filepath.Join(os.TempDir(), "winposprobe.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		panic(err)
	}

	mode, tag := "one", "seq"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if len(os.Args) > 2 {
		tag = os.Args[2]
	}

	a := app.New()

	report := func(tag string, w bellui.Window) {
		if pw, ok := w.(desktop.PositionableWindow); ok {
			x, y := pw.Position()
			fmt.Fprintf(f, "%s: pos=(%d,%d)\n", tag, x, y)
		} else {
			fmt.Fprintf(f, "%s: not positionable\n", tag)
		}
	}

	newWin := func(title string) bellui.Window {
		w := a.NewWindow(title)
		w.SetContent(widget.NewLabel("probe"))
		w.Resize(geo.NewSize(220, 120))
		w.Show()
		return w
	}

	go func() {
		var ws []bellui.Window
		n := 1
		if mode == "multi" {
			n = 3
		}
		bellui.DoAndWait(func() {
			for i := 0; i < n; i++ {
				ws = append(ws, newWin(fmt.Sprintf("%s-%d", tag, i)))
			}
		})
		// Let the window map and the moved callback record its real position.
		time.Sleep(900 * time.Millisecond)
		bellui.DoAndWait(func() {
			for i, w := range ws {
				report(fmt.Sprintf("%s-%d", tag, i), w)
			}
		})
		f.Close()
		bellui.Do(a.Quit)
	}()

	a.Run()
}
