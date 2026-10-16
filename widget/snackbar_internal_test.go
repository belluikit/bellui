package widget

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/test"
)

// findButtonIn walks a rendered tree for the first *Button.
func findButtonIn(obj core.CanvasObject) *Button {
	if b, ok := obj.(*Button); ok {
		return b
	}
	switch o := obj.(type) {
	case *bellui.Container:
		for _, child := range o.Objects {
			if b := findButtonIn(child); b != nil {
				return b
			}
		}
	case core.Widget:
		for _, child := range test.TempWidgetRenderer(nil, o).Objects() {
			if b := findButtonIn(child); b != nil {
				return b
			}
		}
	}
	return nil
}

func TestSnackbar_DoesNotBlockContentBeneath(t *testing.T) {
	tapped := 0
	var action atomic.Int32
	btn := NewButton("Under", func() { tapped++ })
	w := test.NewTempWindow(t, btn)
	w.Resize(geo.NewSize(300, 300))
	c := w.Canvas()

	s := NewSnackbar("Saved", "Undo", func() { action.Add(1) })
	s.Duration = time.Minute
	s.Show(c)
	t.Cleanup(s.Hide)

	// A tap aimed at the button beneath the snackbar reaches it (pre-rebrand tracker #569) and
	// the snackbar persists (MD3: transient surfaces never block, and
	// outside interaction does not dismiss them).
	btnCenter := btn.Position().Add(geo.NewPos(btn.Size().Width/2, btn.Size().Height/2))
	test.TapCanvas(c, btnCenter)
	assert.Equal(t, 1, tapped, "tap passes through the snackbar overlay")
	assert.True(t, s.popUp.Visible(), "snackbar persists through outside taps")

	// The snackbar's own action button is inside the content and still
	// receives taps through the hit test.
	actionBtn := findButtonIn(s.popUp.Content)
	require.NotNil(t, actionBtn)
	pos := intdriver.AbsolutePositionForObject(actionBtn, []core.CanvasObject{s.popUp})
	test.TapCanvas(c, pos.Add(geo.NewPos(actionBtn.Size().Width/2, actionBtn.Size().Height/2)))
	// The snackbar defers OnAction until its hide animation has finished.
	assert.Eventually(t, func() bool { return action.Load() == 1 }, 2*time.Second, 10*time.Millisecond,
		"the snackbar action still works")
	assert.Equal(t, 1, tapped, "the action tap did not leak through")
}
