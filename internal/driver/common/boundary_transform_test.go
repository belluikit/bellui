package common

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
)

// A RepaintBoundary is drawn through its texture only while its world
// transform at most moves it; under a rotation, a scale or a skew the walks
// draw its subtree directly, and the texture is used again once the transform
// is back to a move (#1003).
func TestRenderCacheNode_CachesAsBoundary(t *testing.T) {
	for name, tc := range map[string]struct {
		boundary bool
		wt       geo.Matrix
		want     bool
	}{
		"untransformed":                 {true, geo.Identity(), true},
		"moved":                         {true, geo.NewTranslation(30, -12.5), true},
		"rotated":                       {true, geo.NewRotationAround(math.Pi/2, 50, 50), false},
		"slightly tilted":               {true, geo.NewTranslation(10, 10).Multiply(geo.NewRotation(0.04)), false},
		"scaled":                        {true, geo.NewScale(2, 2), false},
		"skewed":                        {true, geo.NewSkew(0.1, 0), false},
		"not a boundary, untransformed": {false, geo.Identity(), false},
	} {
		t.Run(name, func(t *testing.T) {
			n := &RenderCacheNode{isBoundary: tc.boundary, worldTransform: tc.wt}
			assert.Equal(t, tc.want, n.CachesAsBoundary())
		})
	}

	n := &RenderCacheNode{isBoundary: true, worldTransform: geo.NewRotation(0.04)}
	assert.False(t, n.CachesAsBoundary(), "tilted")
	n.worldTransform = geo.NewTranslation(5, 5)
	assert.True(t, n.CachesAsBoundary(), "the texture is used again once the tilt is gone")
}
