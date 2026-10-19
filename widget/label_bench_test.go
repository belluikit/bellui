package widget

import (
	"strconv"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
)

// BenchmarkLabelSetText measures re-texting a shown label, the per-label cost
// of a screen that updates many labels every frame (#488).
func BenchmarkLabelSetText(b *testing.B) {
	test.NewTempApp(b)
	labels := make([]*Label, 200)
	objs := make([]core.CanvasObject, len(labels))
	for i := range labels {
		labels[i] = NewLabel("0")
		objs[i] = labels[i]
	}
	box := &bellui.Container{Layout: layout.NewGridWrapLayout(geo.NewSize(48, 20)), Objects: objs}
	w := test.NewTempWindow(b, box)
	w.Resize(geo.NewSize(800, 600))
	texts := make([]string, 1000)
	for i := range texts {
		texts[i] = strconv.Itoa(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		labels[i%len(labels)].SetText(texts[i%len(texts)])
	}
}
