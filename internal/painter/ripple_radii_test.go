package painter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter"
)

// RippleCornerRadii gives the ripple shaders their radii in their order (top-
// right, bottom-right, top-left, bottom-left), each corner falling back to
// CornerRadius and kept within the size (#1156).
func TestRippleCornerRadii(t *testing.T) {
	size := geo.NewSize(100, 40)
	r := &canvas.Ripple{CornerRadius: 8}
	tr, br, tl, bl := painter.RippleCornerRadii(r, size)
	assert.Equal(t, [4]float32{8, 8, 8, 8}, [4]float32{tr, br, tl, bl}, "every corner falls back")

	r.TopLeftCornerRadius, r.BottomLeftCornerRadius = 20, 20
	r.CornerRadius = 0
	tr, br, tl, bl = painter.RippleCornerRadii(r, size)
	assert.Equal(t, [4]float32{0, 0, 20, 20}, [4]float32{tr, br, tl, bl}, "a segment rounded on its left")

	r = &canvas.Ripple{CornerRadius: 999}
	tr, br, tl, bl = painter.RippleCornerRadii(r, size)
	assert.Equal(t, [4]float32{20, 20, 20, 20}, [4]float32{tr, br, tl, bl}, "a pill's radius is kept to half its height")
}
