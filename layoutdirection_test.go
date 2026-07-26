package bellui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
)

func TestLayoutDirection_Default(t *testing.T) {
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	assert.Equal(t, bellui.DirectionAuto, bellui.CurrentLayoutDirection(),
		"default layout direction should be auto")

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.Equal(t, bellui.DirectionRTL, bellui.CurrentLayoutDirection())

	bellui.SetLayoutDirection(bellui.DirectionLTR)
	assert.Equal(t, bellui.DirectionLTR, bellui.CurrentLayoutDirection())

	bellui.SetLayoutDirection(bellui.DirectionAuto)
	assert.Equal(t, bellui.DirectionAuto, bellui.CurrentLayoutDirection())
}

func TestSetLayoutDirection_RefreshHook(t *testing.T) {
	bellui.SetLayoutDirection(bellui.DirectionLTR) // known starting point
	t.Cleanup(func() {
		bellui.RegisterLayoutDirectionRefresh(nil)
		bellui.SetLayoutDirection(bellui.DirectionAuto)
	})

	var refreshes int
	bellui.RegisterLayoutDirectionRefresh(func() { refreshes++ })

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.Equal(t, 1, refreshes, "a real direction change should refresh once")

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	assert.Equal(t, 1, refreshes, "setting the same direction should not refresh")

	bellui.SetLayoutDirection(bellui.DirectionAuto)
	assert.Equal(t, 2, refreshes, "changing back to auto should refresh")
}
