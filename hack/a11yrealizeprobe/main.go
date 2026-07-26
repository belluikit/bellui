// Command a11yrealizeprobe is a live-verify harness for the off-viewport
// virtualized-cell realize seam (fyne#637). It shows a window holding one
// widget that reports a huge grid extent (AccessibleGridDimensioner) but
// realizes only a small visible window of cells up front
// (AccessibleVirtualNodes); every other cell is built on demand through
// core.AccessibleVirtualCellRealizer / core.AccessibleScrollIntoView. A UIA
// client (hack/a11yprobe/windows/uiarealize.ps1), a raw AX client
// (hack/a11yprobe/darwin/axcell) or a screen reader can then read a cell far
// past the viewport via GridPattern.GetItem / AXCellForColumnAndRow, which the
// bridge routes through the realize seam. On mobile (#817; package with
// `bellui package --tags accessibility`) the grid publishes as a virtualized
// collection: VoiceOver addresses cells through the data-table container
// (vertical navigation from a visible cell realizes the off-viewport target),
// TalkBack hears the full extent via CollectionInfo and can request
// scroll-to-position.
//
// Build with the accessibility tag so the platform bridge is compiled in:
//
//	go run -tags accessibility ./hack/a11yrealizeprobe
//
// then drive it from another terminal:
//
//	powershell -File hack\a11yprobe\windows\uiarealize.ps1 <pid> 500000 1
package main

import (
	"bufio"
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"

	// Bundle the CJK fallback font so IME checks on the text inputs show real
	// glyphs on every target (wasm has no system fonts to scan — tofu without).
	_ "github.com/belluikit/bellui/theme/cjk"
	"github.com/belluikit/bellui/widget"
)

const (
	cellW, cellH = 130, 32
	visibleRows  = 10 // rows [0,visibleRows) are walked + drawn; the rest realize on demand
)

func main() {
	a := app.New()
	w := a.NewWindow("a11y realize probe")

	grid := newVirtualGrid(1_000_000, 3)
	info := widget.NewLabel(
		"A grid reporting 1,000,000 x 3; rows 0-9 are visible,\n" +
			"every other cell realizes on demand through the a11y seam.",
	)
	// An editable field on the same screen as the grid, so the mobile pass can
	// confirm the screen-reader text-input proxy swap (fyne#570) still works
	// with a data-table container present. Multiline + pre-filled so VoiceOver
	// caret navigation (characters/words/lines rotor) has content to traverse.
	single := widget.NewEntry()
	single.SetText("Single line golf hotel india")
	entry := widget.NewMultiLineEntry()
	entry.SetText("Line one alpha\nLine two bravo\nLine three charlie")
	// A RichEntry beside the plain Entry: it rides the same AccessibleTexter /
	// text-input mirror seam (#773) and declares rotor custom actions (#1025),
	// so the screen-reader pass covers the rich editor path too.
	rich := widget.NewRichEntry()
	rich.SetSegments([]widget.RichTextSegment{
		&widget.TextSegment{Text: "Rich bold line", Style: widget.RichTextStyleStrong},
		&widget.TextSegment{Text: "Rich plain line", Style: widget.RichTextStyleParagraph},
	})

	// On-device trace console (#951 diagnosis): tails the bellui951 NSLog
	// instrumentation emitted by the iOS a11y bridge, so the VO query/answer
	// conversation is visible on the iPad while gesturing.
	console := newTraceConsole()
	console.start()

	w.SetContent(container.NewBorder(
		container.NewVBox(info, single, entry, rich), console.label, nil, nil, grid,
	))
	w.Resize(geo.NewSize(460, 560))
	w.Show()
	a.Run()
}

// traceConsole tails the process's stderr (where NSLog writes) through a pipe,
// keeps the bellui951/PROBE lines, collapses consecutive duplicates, and shows
// the tail in a label at ~5 Hz — batched so console updates add minimal
// main-thread work while VoiceOver is querying.
type traceConsole struct {
	label *widget.Label

	mu       sync.Mutex
	lines    []string // display ring, last entry may carry a (xN) counter
	lastRaw  string   // last raw message, for duplicate collapsing
	lastSeen int      // consecutive count of lastRaw
	dirty    bool
}

const consoleRows = 22

func newTraceConsole() *traceConsole {
	c := &traceConsole{label: widget.NewLabel("trace console\n")}
	c.label.TextStyle = bellui.TextStyle{Monospace: true}
	return c
}

func (c *traceConsole) start() {
	r, echo, ok := tapStderr()
	if !ok {
		c.label.SetText("trace console unavailable on this platform")
		return
	}

	go func() {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 64*1024)
		for sc.Scan() {
			line := sc.Text()
			echo(line)
			c.ingest(line)
		}
	}()

	// BELLUI951_NO_CONSOLE freezes the on-screen pane (lines still echo to the
	// attached console): every SetText grows/relayouts the label, repainting
	// and republishing the a11y tree between VoiceOver gestures — which is
	// itself a suspect for the erratic flick rejections under test.
	if os.Getenv("BELLUI951_NO_CONSOLE") != "" {
		c.label.SetText("on-screen console disabled (BELLUI951_NO_CONSOLE)")
		return
	}

	go func() {
		for range time.Tick(200 * time.Millisecond) {
			c.mu.Lock()
			if !c.dirty {
				c.mu.Unlock()
				continue
			}
			c.dirty = false
			text := strings.Join(c.lines, "\n")
			c.mu.Unlock()
			bellui.Do(func() { c.label.SetText(text) })
		}
	}()
}

// ingest filters one stderr line and appends it to the display ring.
// NSLog lines look like "2026-07-22 22:23:49.073 main[2773:832114] bellui951 …";
// the date and process token are stripped, the ms timestamp kept (it is the
// evidence for the late-answer theory).
func (c *traceConsole) ingest(line string) {
	idx := strings.Index(line, "bellui951 ")
	if idx < 0 {
		if !strings.Contains(line, "PROBE ") {
			return
		}
		idx = strings.Index(line, "PROBE ")
	}
	msg := line[idx:]
	msg = strings.TrimPrefix(msg, "bellui951 ")
	ts := ""
	if len(line) > 24 && line[4] == '-' { // NSLog prefix present
		ts = line[11:23] + " " // "22:23:49.073"
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if msg == c.lastRaw && len(c.lines) > 0 {
		c.lastSeen++
		c.lines[len(c.lines)-1] = fmt.Sprintf("%s%s (x%d)", ts, msg, c.lastSeen)
	} else {
		c.lastRaw = msg
		c.lastSeen = 1
		c.lines = append(c.lines, ts+msg)
		if len(c.lines) > consoleRows {
			c.lines = c.lines[len(c.lines)-consoleRows:]
		}
	}
	c.dirty = true
}

// virtualGrid is a virtualized data grid: it advertises a large extent but
// realizes only a small visible window of cell elements (like a scrolled
// widget.Table / fytables DataGrid would), so the accessibility bridge can
// reach any other cell only through the realize seam — or by scrolling the
// window (event.Scrollable), the path mobile screen readers actually use
// (#952/#953): a VoiceOver three-finger / TalkBack two-finger swipe pages
// winOffset and the new window's cells are realized as they are walked.
// This is the minimal stand-in for fytables' DataGrid (which implements the
// same core interfaces).
type virtualGrid struct {
	widget.BaseWidget
	rows, cols int
	winOffset  int    // first visible row
	lastScroll [2]int // records the most recent AccessibleScrollCellIntoView

	cellTexts []*canvas.Text // row-major visibleRows×cols, set by CreateRenderer
	footer    *canvas.Text
}

func newVirtualGrid(rows, cols int) *virtualGrid {
	g := &virtualGrid{rows: rows, cols: cols, lastScroll: [2]int{-1, -1}}
	g.ExtendBaseWidget(g)
	return g
}

// CreateRenderer draws the visible window with bare canvas primitives — they
// implement no core.Accessible, so the drawn cells contribute nothing to the
// a11y tree; the walked cell elements come only from AccessibleVirtualNodes,
// at matching positions so a screen reader's focus ring lands on the paint.
func (g *virtualGrid) CreateRenderer() core.WidgetRenderer {
	c := container.NewWithoutLayout()
	g.cellTexts = make([]*canvas.Text, visibleRows*g.cols)
	for r := 0; r < visibleRows; r++ {
		for col := 0; col < g.cols; col++ {
			box := canvas.NewRectangle(color.Transparent)
			box.StrokeColor = color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}
			box.StrokeWidth = 1
			box.Resize(geo.NewSize(cellW, cellH))
			box.Move(geo.NewPos(float32(col)*cellW, float32(r)*cellH))
			txt := canvas.NewText("", color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xff})
			txt.TextSize = 14
			txt.Move(geo.NewPos(float32(col)*cellW+8, float32(r)*cellH+6))
			g.cellTexts[r*g.cols+col] = txt
			c.Add(box)
			c.Add(txt)
		}
	}
	g.footer = canvas.NewText("", color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xff})
	g.footer.TextSize = 12
	g.footer.Move(geo.NewPos(0, float32(visibleRows)*cellH+8))
	c.Add(g.footer)
	g.updateWindowTexts()
	return widget.NewSimpleRenderer(c)
}

// updateWindowTexts refreshes the drawn window for the current winOffset.
func (g *virtualGrid) updateWindowTexts() {
	for r := 0; r < visibleRows; r++ {
		for col := 0; col < g.cols; col++ {
			if t := g.cellTexts[r*g.cols+col]; t != nil {
				t.Text = fmt.Sprintf("R%dC%d", g.winOffset+r, col)
			}
		}
	}
	if g.footer != nil {
		g.footer.Text = fmt.Sprintf("window rows %d-%d of %d — scroll to realize more",
			g.winOffset, g.winOffset+visibleRows-1, g.rows)
	}
}

// setWindow slides the visible window to start at row off (clamped), redrawing
// and republishing the cell elements: the rows entering the window are
// realized on demand, the ones leaving are dropped.
func (g *virtualGrid) setWindow(off int) {
	if maxOff := g.rows - visibleRows; off > maxOff {
		off = maxOff
	}
	if off < 0 {
		off = 0
	}
	if off == g.winOffset {
		return
	}
	g.winOffset = off
	fmt.Fprintf(os.Stderr, "PROBE window: rows %d-%d\n", off, off+visibleRows-1)
	g.updateWindowTexts()
	g.Refresh()
}

// Scrolled implements event.Scrollable: the screen-reader scroll gesture
// (VoiceOver three-finger swipe via accessibilityScroll:, #953; TalkBack
// two-finger swipe, #762) arrives as a synthetic page-sized ScrollEvent and
// slides the window — the scroll-driven on-demand realization path.
func (g *virtualGrid) Scrolled(ev *event.ScrollEvent) {
	rows := int(-ev.Scrolled.DY / cellH)
	if rows == 0 {
		if ev.Scrolled.DY < 0 {
			rows = 1
		} else {
			rows = -1
		}
	}
	g.setWindow(g.winOffset + rows)
}

// core.Accessible.
func (g *virtualGrid) AccessibleRole() core.AccessibleRole { return core.RoleTable }
func (g *virtualGrid) AccessibleLabel() string             { return "Virtual data grid" }

// core.AccessibleGridDimensioner: the full extent the UIA Grid pattern reports.
func (g *virtualGrid) AccessibleGridDimensions() (rows, cols int) { return g.rows, g.cols }

// core.AccessibleVirtualNodeProvider: the visible window a real virtualized
// grid would have walked — the screen reader's entry point into the table.
func (g *virtualGrid) AccessibleVirtualNodes() []core.AccessibleVirtualNode {
	var out []core.AccessibleVirtualNode
	for r := 0; r < visibleRows; r++ {
		for c := 0; c < g.cols; c++ {
			n, _ := g.cellNode(g.winOffset+r, c)
			out = append(out, n)
		}
	}
	return out
}

// core.AccessibleVirtualCellRealizer: build a cell on demand for any in-range
// (row, col), regardless of the viewport.
func (g *virtualGrid) AccessibleRealizeCell(row, col int) (core.AccessibleVirtualNode, bool) {
	if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
		return core.AccessibleVirtualNode{}, false
	}
	fmt.Fprintf(os.Stderr, "PROBE realize: row=%d col=%d\n", row, col)
	return g.cellNode(row, col)
}

// cellNode positions the cell relative to the current window: rows inside it
// land on their drawn slots, off-viewport realized rows get their legitimate
// off-screen bounds (they carry no clip by design).
func (g *virtualGrid) cellNode(row, col int) (core.AccessibleVirtualNode, bool) {
	// BELLUI951_NO_ACTIONS strips the cells' custom action: VoiceOver's
	// "Actions Available" hint machinery engages only after the main
	// announcement completes — inside the slow-flick failure window (#951).
	var actions []core.AccessibleAction
	if os.Getenv("BELLUI951_NO_ACTIONS") == "" {
		actions = []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this cell"}}
	}
	return core.AccessibleVirtualNode{
		AccessibleNode: core.AccessibleNode{
			Role:            core.RoleTableCell,
			Label:           fmt.Sprintf("R%dC%d", row, col),
			Actions:         actions,
			HasCellPosition: true,
			CellRow:         row,
			CellCol:         col,
		},
		Pos:  geo.NewPos(float32(col)*cellW, float32(row-g.winOffset)*cellH),
		Size: geo.NewSize(cellW, cellH),
		ID:   [2]int{row, col},
	}, true
}

// core.AccessibleScrollIntoView: the explicit bring-into-view path. Centers
// the window on the requested row, so an AT's activate-realized-cell visibly
// scrolls the grid there.
func (g *virtualGrid) AccessibleScrollCellIntoView(row, col int) {
	g.lastScroll = [2]int{row, col}
	fmt.Fprintf(os.Stderr, "PROBE scrollIntoView: row=%d col=%d\n", row, col)
	g.setWindow(row - visibleRows/2)
}
