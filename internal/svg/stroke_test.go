package svg

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A stroke is as wide as its stroke-width scaled from the viewBox to the
// target, at every size. The stroke used to keep its width in pixels, so a
// 7-unit ring on a 120-unit viewBox was 7 px wide at 40 px and at 240 px
// alike, and small icons filled in (#1292).
func TestDecoder_Draw_StrokeWidthScales(t *testing.T) {
	const (
		viewBox = 120.0
		stroke  = 7.0
		radius  = 50.0
	)
	src := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %[1]g %[1]g">`+
		`<circle cx="%[2]g" cy="%[2]g" r="%[3]g" fill="none" stroke="#000" stroke-width="%[4]g"/></svg>`,
		viewBox, viewBox/2, radius, stroke)

	for _, px := range []int{24, 40, 50, 60, 120, 240} {
		t.Run(fmt.Sprint(px), func(t *testing.T) {
			d, err := NewDecoder(strings.NewReader(src))
			require.NoError(t, err)
			img, err := d.Draw(px, px)
			require.NoError(t, err)
			require.Equal(t, image.Rect(0, 0, px, px), img.Bounds())

			scale := float64(px) / viewBox
			want := stroke * scale
			// The ring crosses the left half of the middle row once: its
			// coverage there, summed, is the stroke's width in pixels.
			y := px / 2
			covered := 0.0
			for x := 0; x < px/2; x++ {
				covered += float64(img.NRGBAAt(x, y).A) / 255
			}
			t.Logf("%d px: stroke %.2f px, want %.2f", px, covered, want)
			assert.InDelta(t, want, covered, 1,
				"stroke at %d px is %.2f px wide, want %.2f", px, covered, want)

			// Inside the ring, well clear of the stroke, nothing is drawn.
			inner := int((radius - stroke) * scale)
			for x := px/2 - inner + 1; x < px/2+inner; x++ {
				if a := img.NRGBAAt(x, y).A; a != 0 {
					t.Fatalf("pixel (%d,%d) inside the ring has alpha %d, want 0", x, y, a)
				}
			}
		})
	}
}

// Dashes scale with the stroke: a dashed line keeps its number of dashes at
// every size (#1292).
func TestDecoder_Draw_DashesScale(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 12">` +
		`<line x1="0" y1="6" x2="120" y2="6" stroke="#000" stroke-width="4" stroke-dasharray="10 10"/></svg>`

	for _, px := range []int{60, 120, 240} {
		t.Run(fmt.Sprint(px), func(t *testing.T) {
			d, err := NewDecoder(strings.NewReader(src))
			require.NoError(t, err)
			img, err := d.Draw(px, px/10)
			require.NoError(t, err)

			y := img.Bounds().Dy() / 2
			dashes, on := 0, false
			for x := 0; x < img.Bounds().Dx(); x++ {
				opaque := img.NRGBAAt(x, y).A > 127
				if opaque && !on {
					dashes++
				}
				on = opaque
			}
			assert.Equal(t, 6, dashes, "dashes across the line at %d px", px)
		})
	}
}
