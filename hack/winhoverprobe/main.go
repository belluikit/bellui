//go:build windows

// Command winhoverprobe live-verifies Windows drag-motion hover feedback
// (bellui pre-rebrand tracker #926 / glfw OLE IDropTarget). It starts a REAL OLE drag from its own
// window via desktop.StartDrag (a file payload, so the full CF_HDROP drop path
// runs) and walks the cursor across a DropHoverable widget with synthetic
// input: in (enter+over), out (leave), back in, release (drop). Events land in
// %TEMP%\winhover.log. Must run on the interactive desktop (schtasks /IT).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/widget"
)

var (
	user32        = windows.NewLazySystemDLL("user32.dll")
	pSetCursorPos = user32.NewProc("SetCursorPos")
	pSendInput    = user32.NewProc("SendInput")
)

const (
	mouseeventfLeftdown = 0x0002
	mouseeventfLeftup   = 0x0004
)

type mouseInput struct {
	typ         uint32
	_           uint32
	dx, dy      int32
	mouseData   uint32
	dwFlags     uint32
	time        uint32
	_           uint32
	dwExtraInfo uintptr
}

func setCursor(x, y int) { pSetCursorPos.Call(uintptr(x), uintptr(y)) }

func mouse(flags uint32) {
	in := mouseInput{typ: 0 /* INPUT_MOUSE */, dwFlags: flags}
	pSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

type probeLog struct {
	mu sync.Mutex
	f  *os.File
}

func (l *probeLog) log(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.f, s)
}

type hoverZone struct {
	widget.BaseWidget
	l     *probeLog
	overs int
}

func (z *hoverZone) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel("hover zone"))
}

func (z *hoverZone) DragEnter(ev *bellui.DragHoverEvent) {
	z.l.log(fmt.Sprintf("ENTER at %.0f,%.0f", ev.Position.X, ev.Position.Y))
}

func (z *hoverZone) DragOver(ev *bellui.DragHoverEvent) {
	z.overs++
	if z.overs == 1 {
		z.l.log(fmt.Sprintf("OVER first at %.0f,%.0f", ev.Position.X, ev.Position.Y))
	}
}
func (z *hoverZone) DragLeave() { z.l.log(fmt.Sprintf("LEAVE (overs so far: %d)", z.overs)) }
func (z *hoverZone) Dropped(ev *bellui.DropEvent) {
	z.l.log(fmt.Sprintf("DROPPED %d item(s) at %.0f,%.0f", len(ev.URIs), ev.Position.X, ev.Position.Y))
}

func main() {
	f, err := os.Create(filepath.Join(os.TempDir(), "winhover.log"))
	if err != nil {
		panic(err)
	}
	l := &probeLog{f: f}

	payload := filepath.Join(os.TempDir(), "winhover-payload.txt")
	os.WriteFile(payload, []byte("drag me"), 0o644)

	a := app.New()
	w := a.NewWindow("winhoverprobe")
	z := &hoverZone{l: l}
	z.ExtendBaseWidget(z)
	w.SetContent(z)
	w.Resize(geo.NewSize(500, 300))
	if dw, ok := w.(desktop.Window); ok {
		dw.RequestPosition(200, 200)
	}
	w.Show()

	go func() {
		time.Sleep(3 * time.Second)
		_, isSource := bellui.CurrentApp().Driver().(desktop.DragSource)
		l.log(fmt.Sprintf("driver is DragSource: %v", isSource))

		// Press INSIDE our own window: a drag starts with the button down on
		// the source, and pressing over the desktop hands Explorer the mouse
		// capture, starving the OLE loop.
		setCursor(450, 380)
		time.Sleep(150 * time.Millisecond)
		mouse(mouseeventfLeftdown)
		l.log("mouse down sent (inside window)")
		time.Sleep(150 * time.Millisecond)

		go bellui.Do(func() {
			l.log("StartDrag: calling")
			ok := desktop.StartDrag(desktop.DragData{URIs: []bellui.URI{storage.NewFileURI(payload)}})
			l.log(fmt.Sprintf("StartDrag returned %v", ok))
		})
		time.Sleep(400 * time.Millisecond)

		// Into the window (enter + overs), out (leave), back in, drop.
		for _, p := range [][2]int{{465, 390}, {480, 400}, {60, 60}, {450, 380}, {470, 395}} {
			setCursor(p[0], p[1])
			time.Sleep(200 * time.Millisecond)
		}
		l.log("path walked; sending mouse up")
		mouse(mouseeventfLeftup)
		time.Sleep(1500 * time.Millisecond)

		l.log(fmt.Sprintf("done (total overs: %d)", z.overs))
		f.Close()
		bellui.Do(a.Quit)
	}()

	a.Run()
}
