// Command singleinstance is the live-verify probe for single-instance mode with
// argument forwarding (#509 / bellui.SingleInstanceApp).
//
//	go run ./hack/singleinstance            # first launch -> becomes primary, opens a window
//	go run ./hack/singleinstance a.txt --x  # while the primary runs -> forwards [a.txt --x] and exits
//
// The running primary appends every forwarded argument list to its window and
// to $TMPDIR/bellui-si-probe.log, proving that "open with MyApp <file>" reaches
// the already-running instance instead of spawning a second one. This is the
// desktop building block behind OS file-association / URL-scheme handling
// (#510). Set BELLUI_SI_PROBE_QUIT=1 to make the primary quit after the first
// forwarded launch (used by the automated two-process check).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

const probeID = "io.bellui.singleinstance.probe"

func logPath() string { return filepath.Join(os.TempDir(), "bellui-si-probe.log") }

func main() {
	a := app.NewWithID(probeID)

	si, ok := a.(bellui.SingleInstanceApp)
	if !ok {
		fmt.Println("this platform does not support single-instance mode")
		return
	}

	log := widget.NewLabel("Primary instance running.\nForward arguments from a second launch.")
	quitOnForward := os.Getenv("BELLUI_SI_PROBE_QUIT") == "1"

	primary, err := si.SetSingleInstance(func(args []string) {
		line := "OPENED " + strings.Join(args, " ")
		fmt.Println(line)
		appendLine(line)
		log.SetText(log.Text + "\n" + line)
		if quitOnForward {
			a.Quit()
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "single-instance error:", err)
		os.Exit(1)
	}

	if !primary {
		fmt.Printf("SECONDARY (pid %d) forwarded %v and exiting\n", os.Getpid(), os.Args[1:])
		return
	}

	fmt.Printf("PRIMARY (pid %d) ready — log at %s\n", os.Getpid(), logPath())
	w := a.NewWindow("single-instance probe")
	w.SetContent(container.NewVBox(log))
	w.Resize(geo.NewSize(420, 200))
	w.Show()
	a.Run()
}

func appendLine(line string) {
	f, err := os.OpenFile(logPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}
