package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
)

// The popup lives in its window's overlays, which a per-window direction
// (Canvas.SetDirection) covers. Its layout follows that direction, so its
// hit-testing must too, though taps arrive outside any layout walk (#696).
func TestCompletionPopup_InfoIconFollowsCanvasDirection(t *testing.T) {
	w, p := newTestCompletionPopup(t, []CompletionItem{{Text: "alpha", Detail: "d"}})
	lang.SetLocaleForTests(t, "en-US")
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	assert.Less(t, p.infoIconRect(0).Pos.X, p.Size().Width/2, "info icon leads on the left in an RTL window")
}
