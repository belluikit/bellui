package main

import (
	"image/png"
	"os"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
)

// TestSoftwareGolden renders the shared scene through the software painter (the
// issue's "correct" reference) and writes it to software_golden.png so it can be
// compared side-by-side with the on-device GPU screenshot. Run with:
//
//	go test ./hack/repro960/ -run TestSoftwareGolden -tags ci
func TestSoftwareGolden(t *testing.T) {
	test.NewTempApp(t)

	c := test.NewCanvasWithPainter(software.NewPainter())
	c.SetPadded(false)
	c.SetContent(BuildScene())
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
