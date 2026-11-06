package widget

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
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

// TestSnackbar_BottomInset: the inset raises the snackbar by exactly that
// many dp, so an app can keep it clear of controls pinned to the bottom.
func TestSnackbar_BottomInset(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Test")
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	restingY := func(inset float32) float32 {
		s := NewSnackbar("Message", "", nil)
		s.Duration = time.Minute
		s.BottomInset = inset
		s.Show(w.Canvas())
		defer s.Hide()
		// Let the slide-up animation (if animations are on) come to rest.
		time.Sleep(canvas.DurationShort4 + 100*time.Millisecond)
		s.mu.Lock()
		defer s.mu.Unlock()
		require.NotNil(t, s.popUp)
		return s.popUp.innerPos.Y
	}

	base := restingY(0)
	raised := restingY(72)
	assert.InDelta(t, 72, base-raised, 0.01, "inset 72 should raise the snackbar by 72dp (base %v, raised %v)", base, raised)
}

// TestSnackbar_HideWhileSlidingIn: hiding a snackbar before its slide-up
// animation ends must not leave the animation moving a popup that Hide
// already dropped. It did (a nil PopUp.Move on the animation tick), which
// crashed an app that replaced or dismissed a message right after showing
// it (belluikit/player#342).
func TestSnackbar_HideWhileSlidingIn(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Test")
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	require.True(t, bellui.CurrentApp().Settings().ShowAnimations(), "the slide-up only runs with animations on")

	s := NewSnackbar("Asking…", "", nil)
	s.Duration = time.Minute
	s.Show(w.Canvas())
	s.mu.Lock()
	slide := s.slide
	s.mu.Unlock()
	require.NotNil(t, slide, "Show keeps its slide-up animation")
	s.Hide()
	// The test driver ticks an animation once, at its start, so tick the
	// slide by hand as a real driver would after Hide: it must not panic.
	assert.NotPanics(t, func() { slide.Tick(0.5) })
}
