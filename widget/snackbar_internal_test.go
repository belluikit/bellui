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
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
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
		// The cached renderer: TempWidgetRenderer needs a *testing.T.
		for _, child := range cache.Renderer(o).Objects() {
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

// TestSnackbar_ShowAgainWhileFading: a snackbar shown again while its
// previous popup is still fading out keeps the new popup on the canvas.
// OverlayStack.Remove drops every overlay above the one removed, so the old
// popup's fade ending after the new popup was added took the new one with
// it: a replaced message showed nothing in a real window
// (belluikit/player#342). The test driver ends every animation as it starts,
// so the fade is held open by hand: the old popup is put back on the canvas
// as a real driver still has it, and its fade is ended after Show.
func TestSnackbar_ShowAgainWhileFading(t *testing.T) {
	app := test.NewTempApp(t)
	w := app.NewWindow("Test")
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))
	c := w.Canvas()

	s := NewSnackbar("Asking…", "", nil)
	s.Duration = time.Minute
	s.Show(c)
	s.mu.Lock()
	old := s.popUp
	s.mu.Unlock()
	s.Hide()

	// A real driver: the fade has not ended, so the old popup is still on
	// the canvas when the next message shows.
	c.Overlays().Add(old)
	old.overlayShown = true

	s.Text = "Loading the song…"
	s.Show(c)
	s.mu.Lock()
	current := s.popUp
	s.mu.Unlock()

	old.finishHide() // the old fade ends now
	assert.Same(t, current, c.Overlays().Top(), "the new message must still be on the canvas after the old one's fade ends")
	assert.Len(t, c.Overlays().List(), 1)
	s.Hide()
}

// TestShowSnackbar_OneAtATime: MD3 shows one snackbar at a time. Each
// ShowSnackbar call builds its own Snackbar, and two of them stacked their
// bars (#1038).
func TestShowSnackbar_OneAtATime(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel("Content"))
	w.Resize(geo.NewSize(400, 300))
	c := w.Canvas()

	ShowSnackbar("First", "", nil, c)
	ShowSnackbar("Second", "", nil, c)
	overlays := c.Overlays().List()
	require.Len(t, overlays, 1, "two ShowSnackbar calls leave one snackbar")
	s, ok := canvasSnackbars.Load(c)
	require.True(t, ok)
	snack := s.(*Snackbar)
	assert.Equal(t, "Second", snack.Text)
	assert.Same(t, snack.popUp, overlays[0])
	snack.Hide()
	_, ok = canvasSnackbars.Load(c)
	assert.False(t, ok, "a hidden snackbar is no longer the canvas's")
}

// TestShowSnackbar_OneAtATimeWhileFading: a snackbar from another value,
// shown while the canvas's current one fades in, takes that one off at once.
func TestShowSnackbar_OneAtATimeWhileFading(t *testing.T) {
	c := holdAnimations(t)
	first := NewSnackbar("First", "", nil)
	first.Duration = time.Minute
	first.Show(c)
	second := NewSnackbar("Second", "", nil)
	second.Duration = time.Minute
	second.Show(c)
	t.Cleanup(second.Hide)
	assert.Equal(t, []core.CanvasObject{second.popUp}, c.Overlays().List())
}

// A message wider than the canvas wraps inside the snackbar's margins
// instead of running off both edges (#1106). The text is DaySong's, on a
// phone-width canvas.
func TestSnackbar_LongMessageWrapsInsideMargins(t *testing.T) {
	const message = "Like not kept: DaySong can't be reached right now. Check your connection and try again."
	for _, actionText := range []string{"", "Retry"} {
		w := test.NewTempWindow(t, NewLabel(""))
		w.Resize(geo.NewSize(360, 640))
		c := w.Canvas()

		s := NewSnackbar(message, actionText, func() {})
		s.Duration = time.Minute
		s.Show(c)
		t.Cleanup(s.Hide)

		margin := theme.Padding() * 2
		pos, size := s.popUp.innerPos, s.popUp.MinSize()
		assert.GreaterOrEqual(t, pos.X, margin, "action %q: left edge inside the margin", actionText)
		assert.LessOrEqual(t, pos.X+size.Width, c.Size().Width-margin, "action %q: right edge inside the margin", actionText)

		line := bellui.MeasureText("Hg", theme.TextSize(), bellui.TextStyle{}).Height
		layout := s.popUp.Content.(*bellui.Container).Objects[0].(*bellui.Container).Layout.(*snackbarLayout)
		lines := layout.textSize().Height / line
		assert.True(t, lines > 1.5 && lines < 2.5, "action %q: two lines, got %.2f", actionText, lines)
	}
}

// In a right-to-left canvas the action leads and the message follows, as
// they did in the HBox the snackbar's own layout replaced (#1106).
func TestSnackbar_RTLPutsActionFirst(t *testing.T) {
	w := test.NewTempWindow(t, NewLabel(""))
	w.Resize(geo.NewSize(360, 640))
	lang.SetLocaleForTests(t, "en-US")
	w.Canvas().(interface{ SetDirection(bellui.Direction) }).SetDirection(bellui.DirectionRTL)

	s := NewSnackbar("Like not kept: DaySong can't be reached right now. Check your connection.", "Retry", func() {})
	s.Duration = time.Minute
	s.Show(w.Canvas())
	t.Cleanup(s.Hide)

	content := s.popUp.Content.(*bellui.Container).Objects[0].(*bellui.Container)
	l := content.Layout.(*snackbarLayout)
	assert.Less(t, l.action.Position().X, l.text.Position().X, "the action leads")
	assert.Equal(t, content.Size().Width, l.text.Position().X+l.text.Size().Width-l.outset,
		"the message ends at the content's right edge")
}
