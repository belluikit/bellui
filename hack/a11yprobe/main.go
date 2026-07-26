// a11yprobe is the live-verification harness for the accessibility platform
// backends (epic #490, checked in per #561). It exercises every bridge
// feature from two windows so per-window state, focus routing and the full
// widget coverage can be driven and observed from the outside — on macOS via
// the raw-AX clients in darwin/ (see the README there for why System Events
// is not enough), on Windows via the UIA PowerShell clients in windows/.
//
// Build and run from the repository root:
//
//	go run -tags accessibility ./hack/a11yprobe
//
// Window "Probe A" carries the coverage: a slider (AXIncrement/AXDecrement),
// a multi-line entry and a password entry (text protocol, secure-field
// subrole), markdown headings (virtual nodes, heading rotor), a selectable
// label (read-only text protocol), a TreeView with rename/delete (outline
// rows, disclosure, custom actions) and Toast/Snackbar triggers
// (announcements). Window "Probe B" is minimal — verifying disjoint
// per-window trees and key-window focus gating — plus a virtualized Table
// for the coordinate patterns (UIA Grid/GridItem).
//
// Widget callbacks print PROBE-prefixed lines to stdout so an external
// driver can confirm an action reached the widget, not just the bridge.
package main

import (
	"fmt"

	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func main() {
	a := app.New()

	wA := a.NewWindow("Probe A")
	slider := widget.NewSlider(0, 100)
	slider.Step = 5
	slider.SetValue(50)
	slider.OnChanged = func(v float64) { fmt.Println("PROBE slider:", v) }

	entry := widget.NewMultiLineEntry()
	entry.SetText("hello\nworld wide\nrows")
	entry.OnChanged = func(s string) { fmt.Println("PROBE entry changed:", len(s), "bytes") }
	entry.OnSelectChanged = func(s string) { fmt.Printf("PROBE entry selection: %q\n", s) }

	pw := widget.NewPasswordEntry()
	pw.SetText("secret")

	rich := widget.NewRichTextFromMarkdown("# Top Title\n\nSome body text.\n\n## Second Section\n\nMore body.")

	selLabel := widget.NewLabel("selectme grabme copyme")
	selLabel.Selectable = true

	tree := widget.NewTreeView()
	tree.AllowRename = true
	tree.AllowDelete = true
	_ = tree.AddNode("", widget.TreeViewNode{ID: "branch", Text: "branch"})
	_ = tree.AddNode("branch", widget.TreeViewNode{ID: "leaf-1", Text: "leaf-1"})
	_ = tree.AddNode("branch", widget.TreeViewNode{ID: "leaf-2", Text: "leaf-2"})
	_ = tree.AddNode("", widget.TreeViewNode{ID: "solo", Text: "solo"})

	toastBtn := widget.NewButton("Show Toast", func() {
		fmt.Println("PROBE toast shown")
		widget.ShowToast("announcement test toast", wA.Canvas())
	})
	snackBtn := widget.NewButton("Show Snackbar", func() {
		fmt.Println("PROBE snackbar shown")
		widget.ShowSnackbar("announcement test snackbar", "Undo", func() {}, wA.Canvas())
	})

	// Border layout: the tree takes the remaining height. Rows outside a
	// height-starved widget's clip are culled from the accessibility tree,
	// which reads as "missing nodes" — give it real space.
	top := container.NewVBox(
		widget.NewLabel("window A"),
		slider,
		entry,
		pw,
		rich,
		selLabel,
		toastBtn,
		snackBtn,
	)
	wA.SetContent(container.NewBorder(top, nil, nil, nil, tree))
	wA.Resize(geo.NewSize(560, 760))

	wB := a.NewWindow("Probe B")
	bEntry := widget.NewEntry()
	bEntry.SetText("window b text")
	// A virtualized table: only visible cells realize semantic nodes, while
	// the container reports the full 20x3 extent (UIA Grid pattern / #487;
	// GridItem coordinates on the realized cells).
	table := widget.NewTable(
		func() (int, int) { return 20, 3 },
		func() core.CanvasObject { return widget.NewLabel("cell") },
		func(id widget.TableCellID, o core.CanvasObject) {
			o.(*widget.Label).SetText(fmt.Sprintf("r%dc%d", id.Row, id.Col))
		},
	)
	table.OnSelected = func(id widget.TableCellID) { fmt.Println("PROBE table selected:", id.Row, id.Col) }
	wB.SetContent(container.NewBorder(container.NewVBox(
		widget.NewLabel("window B"),
		widget.NewButton("B Button", func() { fmt.Println("PROBE B tapped") }),
		bEntry,
	), nil, nil, nil, table))
	wB.Resize(geo.NewSize(360, 420))

	wA.Show()
	wB.Show()
	a.Run()
}
