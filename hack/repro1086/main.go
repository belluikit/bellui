// Command repro1086 drives the #1086 aligned-oversized-container scene on a real
// window (WebGPU/Metal by default) so the anchor point can be eyeballed against
// the software golden (hack/repro1086 TestSoftwareGolden). With the #1086 fix the
// scaled glyphs pivot on the text-box origin (they stay left, matching software);
// a pre-fix draw.go shifts center/trailing glyphs right of the magenta guide.
package main

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/geo"
)

func main() {
	a := app.New()
	w := a.NewWindow("repro1086")

	w.SetContent(BuildAlignedScene())
	w.Resize(geo.NewSize(400, 700))

	bellui.ShowAndRun(w)
}
