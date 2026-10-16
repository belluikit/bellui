// Command openwith is the live-verify probe for the desktop "open with" events
// (pre-rebrand tracker #510 Phase 1: bellui.OpenHandlerApp — SetOnFileOpened / SetOnURLOpened).
//
//	go run ./hack/openwith /tmp/hello.txt     # launch payload -> OnFileOpened
//	go run ./hack/openwith 'myapp://deep/link' # launch payload -> OnURLOpened
//
// While a primary is running, a second launch forwards its file/URL argument to
// it via single-instance mode and the primary's OnFileOpened/OnURLOpened fires —
// this is the already-running "open with" path on Windows/Linux. The primary
// appends every event to its window and to $TMPDIR/bellui-openwith-probe.log.
// Set BELLUI_OW_PROBE_QUIT=1 to make the primary quit after the first event.
package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

const probeID = "io.bellui.openwith.probe"

func logPath() string { return filepath.Join(os.TempDir(), "bellui-openwith-probe.log") }

func main() {
	a := app.NewWithID(probeID)

	oh, ok := a.(bellui.OpenHandlerApp)
	if !ok {
		fmt.Println("this platform does not support open-with events")
		return
	}

	log := widget.NewLabel("Primary running.\nOpen a file/URL with this app.")
	quitOnEvent := os.Getenv("BELLUI_OW_PROBE_QUIT") == "1"
	note := func(line string) {
		fmt.Println(line)
		appendLine(line)
		log.SetText(log.Text + "\n" + line)
		if quitOnEvent {
			a.Quit()
		}
	}

	oh.SetOnFileOpened(func(u bellui.URI) { note("FILE " + u.String()) })
	oh.SetOnURLOpened(func(u *url.URL) { note("URL " + u.String()) })

	// Enable single-instance so a second launch forwards its arg to us (the
	// already-running path) instead of opening a second window.
	if si, ok := a.(bellui.SingleInstanceApp); ok {
		if primary, err := si.SetSingleInstance(nil); err == nil && !primary {
			fmt.Printf("SECONDARY (pid %d) forwarded %v and exiting\n", os.Getpid(), os.Args[1:])
			return
		}
	}

	fmt.Printf("PRIMARY (pid %d) ready — log at %s\n", os.Getpid(), logPath())
	w := a.NewWindow("open-with probe")
	w.SetContent(container.NewVBox(log))
	w.Resize(geo.NewSize(460, 220))
	w.Show()
	a.Run()
}

func appendLine(line string) {
	f, err := os.OpenFile(logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, strings.TrimRight(line, "\n"))
}
