package common

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// A rectangle clip that its world transform turns or skews is also a stencil
// clip with no corner radius, as the scissor alone would clip to the box
// around the turned rectangle; a move or a scale keeps it a scissor clip only
// (#1003). A rounded clip keeps its radius under any transform.
func TestPaintClips_RotatedRectangleClipUsesStencil(t *testing.T) {
	rect := &bellui.Container{ClipToBounds: true}
	rounded := &bellui.Container{ClipToBounds: true, ClipCornerRadius: 12}
	plain := &bellui.Container{}
	for name, tc := range map[string]struct {
		obj  *bellui.Container
		wt   geo.Matrix
		want driver.Clips
	}{
		"untransformed":        {rect, geo.Identity(), driver.Clips{Rect: true}},
		"moved":                {rect, geo.NewTranslation(30, -12.5), driver.Clips{Rect: true}},
		"scaled":               {rect, geo.NewScale(2, 0.5), driver.Clips{Rect: true}},
		"rotated":              {rect, geo.NewRotationAround(math.Pi/4, 50, 50), driver.Clips{Rect: true, Rounded: true}},
		"slightly tilted":      {rect, geo.NewTranslation(10, 10).Multiply(geo.NewRotation(0.04)), driver.Clips{Rect: true, Rounded: true}},
		"skewed":               {rect, geo.NewSkew(0.1, 0), driver.Clips{Rect: true, Rounded: true}},
		"rounded, rotated":     {rounded, geo.NewRotation(math.Pi / 4), driver.Clips{Rect: true, Rounded: true, Radius: 12}},
		"not a clip, rotated":  {plain, geo.NewRotation(math.Pi / 4), driver.Clips{}},
		"rotated quarter turn": {rect, geo.NewRotationAround(math.Pi/2, 50, 50), driver.Clips{Rect: true, Rounded: true}},
	} {
		t.Run(name, func(t *testing.T) {
			n := &RenderCacheNode{obj: tc.obj, worldTransform: tc.wt}
			assert.Equal(t, tc.want, PaintClips(n))
		})
	}

	culled := &RenderCacheNode{obj: rect, worldTransform: geo.NewRotation(math.Pi / 4), paintCulled: true}
	assert.Equal(t, driver.Clips{}, PaintClips(culled), "a culled clip-owner sets up no clip")
}
