package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// TestSoftwareGolden renders the pre-rebrand tracker #1086 aligned scene through the software painter
// (the always-correct reference — it anchors at the box origin and applies the
// alignment offset inside the scaled bitmap) and writes software_golden.png for
// side-by-side comparison with the on-GPU screenshot. Run with:
//
//	go test ./hack/repro1086/ -run TestSoftwareGolden -tags ci
func TestSoftwareGolden(t *testing.T) {
	test.NewTempApp(t)

	c := test.NewCanvasWithPainter(software.NewPainter())
	c.SetPadded(false)
	c.SetContent(BuildAlignedScene())
	c.Resize(geo.NewSize(400, 700))

	img := c.Capture()

	f, err := os.Create("software_golden.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote software_golden.png (%dx%d)", img.Bounds().Dx(), img.Bounds().Dy())
}
