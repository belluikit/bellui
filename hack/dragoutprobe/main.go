// Command dragoutprobe is a live-verify harness for pre-rebrand tracker #515 Part B (drag-out to
// other applications). It shows a box you can drag OUT of the bellui window into
// Finder (drops a .txt file) or a text editor (drops the text).
//
// On desktop the box uses the push path (desktop.StartDrag from Dragged); macOS,
// Windows and Linux have native backends, other desktops log that StartDrag
// returned false. On the web target (GOOS=js GOARCH=wasm) drag-out is pull-based
// (pre-rebrand tracker #874): the box also implements desktop.DragOutProvider, which the wasm HTML5
// Drag-and-Drop backend consults at drag start (pre-rebrand tracker #857). Build the web version
// with the bellui CLI and drag the box onto a browser drop zone or another app.
//
//	go run ./hack/dragoutprobe                       # desktop
//	bellui package -os web && (serve bellui-web/)        # web
package main

import (
	"log"
	"os"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/storage"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// dragBox is a Draggable that starts a native drag-out on the first Dragged
// event of a gesture, carrying a sample text flavor and a temp .txt file.
type dragBox struct {
	widget.BaseWidget
	started bool
	file    string
}

// dragData builds the drag-out payload, lazily creating a temp .txt to drag out
// as a file (best-effort — the web target has no real filesystem, so the file is
// simply omitted there and the text flavor is dragged alone).
func (b *dragBox) dragData() desktop.DragData {
	if b.file == "" {
		f, err := os.CreateTemp("", "bellui-dragout-*.txt")
		if err == nil {
			_, _ = f.WriteString("Dragged out of a bellui app! (#515 Part B)\n")
			_ = f.Close()
			b.file = f.Name()
		}
	}

	data := desktop.DragData{
		Text: "Hello from a bellui drag-out (#515 Part B)",
		// pre-rebrand tracker #842: supply a CanvasObject ghost instead of a raster image — it is
		// software-rendered to the drag image so one visual serves in-canvas and
		// drag-out.
		DragImageObject: dragGhost(),
		// pre-rebrand tracker #841: react to the drop outcome on the main goroutine.
		OnCompleted: func(accepted bool) {
			log.Printf("drag-out completed: accepted=%v", accepted)
		},
	}
	if b.file != "" {
		data.URIs = []bellui.URI{storage.NewFileURI(b.file)}
	}
	return data
}

// Dragged starts a native drag-out on the first Dragged of a gesture via the
// push path (desktop.StartDrag). On the web target Dragged never fires during a
// native drag — the pull path (DragOutData) is used there instead.
func (b *dragBox) Dragged(*event.DragEvent) {
	if b.started {
		return // one drag-out per gesture
	}
	b.started = true

	data := b.dragData()
	if desktop.StartDrag(data) {
		log.Printf("drag-out started (push): text=%q file=%q (ghost + OnCompleted wired)", data.Text, b.file)
	} else {
		log.Println("push drag-out NOT supported on this platform/driver (StartDrag returned false; web uses the pull path)")
	}
}

// DragOutData is the pull-based drag-out seam (pre-rebrand tracker #874) the web backend consults at
// drag start (pre-rebrand tracker #857); on desktop it is inert (the push path above is used).
func (b *dragBox) DragOutData() (desktop.DragData, bool) {
	data := b.dragData()
	log.Printf("drag-out started (pull): text=%q file=%q", data.Text, b.file)
	return data, true
}

func (b *dragBox) DragEnd() { b.started = false }

// dragGhost builds a small labeled pill used as the drag-out image (pre-rebrand tracker #842): a
// core.CanvasObject the toolkit software-renders to the native drag image.
func dragGhost() core.CanvasObject {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameSecondary))
	bg.CornerRadius = 8
	bg.Resize(geo.NewSize(120, 40))
	txt := canvas.NewText("  dragging  ", theme.Color(theme.ColorNameForegroundOnPrimary))
	txt.Move(geo.NewPos(8, 10))
	ghost := container.NewWithoutLayout(bg, txt)
	ghost.Resize(geo.NewSize(120, 40))
	return ghost
}

func (b *dragBox) CreateRenderer() core.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNamePrimary))
	bg.CornerRadius = 12
	label := canvas.NewText("  drag me out  ", theme.Color(theme.ColorNameForegroundOnPrimary))
	label.Alignment = bellui.TextAlignCenter
	return &dragBoxRenderer{bg: bg, label: label, objects: []core.CanvasObject{bg, label}}
}

type dragBoxRenderer struct {
	bg      *canvas.Rectangle
	label   *canvas.Text
	objects []core.CanvasObject
}

func (r *dragBoxRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	r.label.Resize(size)
	r.label.Move(geo.NewPos(0, (size.Height-r.label.MinSize().Height)/2))
}
func (r *dragBoxRenderer) MinSize() geo.Size            { return geo.NewSize(220, 90) }
func (r *dragBoxRenderer) Refresh()                     { canvas.Refresh(r.bg) }
func (r *dragBoxRenderer) Objects() []core.CanvasObject { return r.objects }
func (r *dragBoxRenderer) Destroy()                     {}

var (
	_ event.Draggable         = (*dragBox)(nil)
	_ desktop.DragOutProvider = (*dragBox)(nil)
	_ core.WidgetRenderer     = (*dragBoxRenderer)(nil)
)

func main() {
	a := app.New()
	w := a.NewWindow("Drag-out probe (#515B)")

	box := &dragBox{}
	box.ExtendBaseWidget(box)

	w.SetContent(container.NewVBox(
		widget.NewLabel("Drag the box into Finder (drops a .txt) or a text editor (drops text)."),
		box,
	))
	w.Resize(geo.NewSize(420, 200))
	w.Show()
	a.Run()
}
