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

// TestDirectionalResource_Set pins which theme icons mirror under RTL: those
// whose Material symbol is marked android:autoMirrored="true" in Google's
// material-design-icons (#779). Every other icon keeps its orientation.
func TestDirectionalResource_Set(t *testing.T) {
	mirrored := map[bellui.ThemeIconName]bool{
		// Mirrored since RTL Phase 4.
		IconNameNavigateBack: true, IconNameNavigateNext: true, IconNameMenuExpand: true,
		IconNameMailForward: true, IconNameMailReply: true, IconNameMailReplyAll: true,
		// Added in #779.
		IconNameContentCopy: true, IconNameContentRedo: true, IconNameContentUndo: true,
		IconNameList: true, IconNameLogin: true, IconNameLogout: true, IconNameMailSend: true,
		IconNameHelp: true, IconNameQuestion: true,
		IconNameVolumeDown: true, IconNameVolumeMute: true, IconNameVolumeUp: true,
		IconNameReceiptLong: true, IconNameFile: true, IconNameFileText: true,
	}
	withRTL(t)
	for name, res := range icons {
		got := strings.Contains(res.Name(), "rtl_")
		assert.Equal(t, mirrored[name], got, "%s: mirrored under RTL", name)
	}
	for name := range mirrored {
		assert.Contains(t, icons, name)
	}
}
