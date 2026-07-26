package software_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// darkColumnRange returns the first and last image columns that contain a
// noticeably dark (drawn glyph) pixel. It returns (-1, -1) when nothing was
// drawn, so callers can locate text within a wider container.
func darkColumnRange(img image.Image) (minX, maxX int) {
	minX, maxX = -1, -1
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 0x8000 && r < 0x8000 && g < 0x8000 && bl < 0x8000 {
				if minX == -1 {
					minX = x
				}
				maxX = x
				break
			}
		}
	}
	return minX, maxX
}

// TestDrawText_LeadingAlignmentRTL verifies that TextAlignLeading resolves to
// the right edge under an RTL layout while remaining on the left under LTR. The
// canvas is made much wider than the glyph so the alignment offset is visible.
func TestDrawText_LeadingAlignmentRTL(t *testing.T) {
	test.ApplyTheme(t, test.Theme())
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	txt := canvas.NewText("A", color.Black)
	txt.Alignment = bellui.TextAlignLeading

	c := test.NewCanvas()
	c.SetContent(txt)
	c.Resize(geo.NewSize(200+2*theme.Padding(), 40))
	p := software.NewPainter()

	bellui.SetLayoutDirection(bellui.DirectionLTR)
	ltrImg := p.Paint(c)
	ltrMin, ltrMax := darkColumnRange(ltrImg)

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	rtlImg := p.Paint(c)
	rtlMin, _ := darkColumnRange(rtlImg)

	if !assert.NotEqual(t, -1, ltrMin, "glyph should render under LTR") ||
		!assert.NotEqual(t, -1, rtlMin, "glyph should render under RTL") {
		return
	}

	width := ltrImg.Bounds().Dx()
	// Leading under LTR hugs the left edge; under RTL it hugs the right edge.
	assert.Less(t, ltrMax, width/2, "leading text should be on the left half under LTR")
	assert.Greater(t, rtlMin, width/2, "leading text should be on the right half under RTL")
}

// TestDrawText_LeadingAlignmentContainerScope verifies the RTL Phase 7 (#178)
// paint-time fix: with the process-global direction left as LTR, a leading-aligned
// canvas.Text wrapped in a container.DirectionScope(RTL) still hugs the right edge.
// This exercises Seam B — paint-time alignment resolved against a *container*
// scope (not just a per-canvas scope) via the uidir push in the software paint walk.
func TestDrawText_LeadingAlignmentContainerScope(t *testing.T) {
	test.ApplyTheme(t, test.Theme())
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	txt := canvas.NewText("A", color.Black)
	txt.Alignment = bellui.TextAlignLeading

	// A per-container RTL scope around the slack-width text; the global stays LTR.
	scope := container.NewDirectionScope(txt, bellui.DirectionRTL)

	c := test.NewCanvas()
	c.SetContent(scope)
	c.Resize(geo.NewSize(200+2*theme.Padding(), 40))
	p := software.NewPainter()

	img := p.Paint(c)
	minX, _ := darkColumnRange(img)
	if !assert.NotEqual(t, -1, minX, "glyph should render") {
		return
	}

	width := img.Bounds().Dx()
	// The RTL container scope alone (global is LTR) must push leading text right.
	assert.Greater(t, minX, width/2, "leading text in an RTL container scope should hug the right edge")
}

// TestDrawText_LeadingAlignmentCanvasScope verifies the per-canvas paint path
// (#186): with the process-global direction left as LTR, a leading-aligned
// canvas.Text on a canvas put into RTL via Canvas.SetDirection still hugs the
// right edge. This is the software-painter analogue of the GL drivers' per-canvas
// paint wrap, now exercisable without a GPU.
func TestDrawText_LeadingAlignmentCanvasScope(t *testing.T) {
	test.ApplyTheme(t, test.Theme())
	bellui.SetLayoutDirection(bellui.DirectionLTR)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	txt := canvas.NewText("A", color.Black)
	txt.Alignment = bellui.TextAlignLeading

	c := test.NewCanvas()
	c.SetContent(txt)
	c.Resize(geo.NewSize(200+2*theme.Padding(), 40))
	// Scope the whole canvas to RTL while the process-global stays LTR.
	c.(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	p := software.NewPainter()

	img := p.Paint(c)
	minX, _ := darkColumnRange(img)
	if !assert.NotEqual(t, -1, minX, "glyph should render") {
		return
	}

	width := img.Bounds().Dx()
	assert.Greater(t, minX, width/2, "leading text on an RTL-scoped canvas should hug the right edge")
}
