package theme

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func withRTL(t *testing.T) {
	t.Helper()
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })
}

func TestDirectionalResource_MirrorsUnderRTL(t *testing.T) {
	back := NavigateBackIcon()
	uprightName := back.Name()
	uprightContent := back.Content()

	withRTL(t)

	// A distinct cache name keeps the mirrored artwork from colliding with the
	// upright one in the SVG raster cache.
	assert.NotEqual(t, uprightName, back.Name())
	assert.Contains(t, back.Name(), "rtl_")

	// The mirrored content carries the reflecting group transform and differs
	// from the upright artwork.
	assert.NotEqual(t, uprightContent, back.Content())
	assert.Contains(t, string(back.Content()), "matrix(-1,0,0,1,24,0)")
}

func TestDirectionalResource_NonDirectionalUnchanged(t *testing.T) {
	search := SearchIcon()
	uprightName := search.Name()
	uprightContent := search.Content()

	withRTL(t)

	assert.Equal(t, uprightName, search.Name())
	assert.Equal(t, uprightContent, search.Content())
	assert.NotContains(t, string(search.Content()), "matrix(-1,0,0,1")
}

func TestDirectionalResource_DirectionalSet(t *testing.T) {
	withRTL(t)
	for _, res := range []bellui.Resource{NavigateBackIcon(), NavigateNextIcon(), MenuExpandIcon()} {
		assert.True(t, strings.Contains(res.Name(), "rtl_"), "%s should mirror under RTL", res.Name())
	}
}
