package svg

import (
	"bytes"
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const arrowBackSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M0 0h24v24H0z" fill="none"/><path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z"/></svg>`

func TestFlipHorizontal_WrapsInReflectingGroup(t *testing.T) {
	out := FlipHorizontal([]byte(arrowBackSVG))
	str := string(out)

	// The reflecting transform reflects [0,24] onto itself: matrix(-1,0,0,1,24,0).
	assert.Contains(t, str, "matrix(-1,0,0,1,24,0)")
	// Geometry must be preserved, just wrapped — the original path data survives.
	assert.Contains(t, str, "M20 11H7.83")
	// The result is still valid, decodable SVG.
	_, err := NewDecoder(bytes.NewReader(out))
	require.NoError(t, err)
}

func TestFlipHorizontal_MirrorsPixels(t *testing.T) {
	// A glyph entirely in the left half of the viewBox should land in the right
	// half after a horizontal flip.
	left := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><rect x="2" y="10" width="6" height="4"/></svg>`

	origImg := rasterise(t, []byte(left))
	flipImg := rasterise(t, FlipHorizontal([]byte(left)))

	assert.Greater(t, filledMass(origImg, leftHalf), filledMass(origImg, rightHalf),
		"original glyph should sit in the left half")
	assert.Greater(t, filledMass(flipImg, rightHalf), filledMass(flipImg, leftHalf),
		"flipped glyph should sit in the right half")
}

func TestFlipHorizontal_InvalidReturnsSource(t *testing.T) {
	junk := []byte("not an svg at all")
	assert.Equal(t, junk, FlipHorizontal(junk))
}

func TestFlipHorizontal_NoViewBoxUsesWidth(t *testing.T) {
	noViewBox := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24"><rect x="2" y="10" width="6" height="4"/></svg>`
	out := string(FlipHorizontal([]byte(noViewBox)))
	assert.Contains(t, out, "matrix(-1,0,0,1,24,0)")
}

func rasterise(t *testing.T, src []byte) *image.NRGBA {
	t.Helper()
	dec, err := NewDecoder(bytes.NewReader(src))
	require.NoError(t, err)
	img, err := dec.Draw(24, 24)
	require.NoError(t, err)
	return img
}

type half int

const (
	leftHalf half = iota
	rightHalf
)

func filledMass(img *image.NRGBA, side half) int {
	b := img.Bounds()
	mid := b.Min.X + b.Dx()/2
	total := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			inLeft := x < mid
			if (side == leftHalf) != inLeft {
				continue
			}
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				total++
			}
		}
	}
	return total
}
