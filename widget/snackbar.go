package widget

import (
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

const snackbarDefaultDuration = 4 * time.Second

// Snackbar shows a brief message at the bottom of the screen with an optional action button.
// It auto-dismisses after Duration.
type Snackbar struct {
	Text       string
	ActionText string
	OnAction   func() `json:"-"`
	Duration   time.Duration

	canvas core.Canvas

	mu          sync.Mutex
	popUp       *PopUp
	timer       *time.Timer
	actionTimer *time.Timer
}

// NewSnackbar creates a snackbar. Call Show() to display it.
func NewSnackbar(text, actionText string, action func()) *Snackbar {
	return &Snackbar{
		Text:       text,
		ActionText: actionText,
		OnAction:   action,
		Duration:   snackbarDefaultDuration,
	}
}

// Show displays the snackbar at the bottom of the given canvas.
func (s *Snackbar) Show(c core.Canvas) {
	s.canvas = c
	s.Hide() // dismiss any previous

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bgColor := th.Color(theme.ColorNameInverseSurface, v)
	textColor := th.Color(theme.ColorNameInverseOnSurface, v)

	label := canvas.NewText(s.Text, textColor)
	label.TextSize = th.Size(theme.SizeNameText)

	objects := []core.CanvasObject{label}

	if s.ActionText != "" {
		action := s.OnAction
		actionBtn := NewButton(s.ActionText, func() {
			s.Hide()
			// Delay action until after hide animation completes. Track the
			// timer so a later Hide (e.g. a replacing snackbar) cancels it.
			if action != nil {
				// The user callback belongs on the UI goroutine, like the
				// auto-hide below — not on the raw timer goroutine.
				t := time.AfterFunc(canvas.DurationShort2+50*time.Millisecond, func() {
					bellui.Do(action)
				})
				s.mu.Lock()
				s.actionTimer = t
				s.mu.Unlock()
			}
		})
		actionBtn.Importance = HighImportance

		objects = append(objects, layout.NewSpacer(), actionBtn)
	}

	content := &bellui.Container{Layout: layout.NewHBoxLayout(), Objects: objects}
	pad := th.Size(theme.SizeNameInnerPadding)
	padded := &bellui.Container{Layout: layout.NewPaddedLayout(), Objects: []core.CanvasObject{content}}
	_ = pad

	s.mu.Lock()
	s.popUp = newPopUpWithBgColor(padded, c, bgColor)
	s.popUp.noDismissOnTap = true // snackbar handles its own dismissal via timer/action
	// Like Toast, a snackbar never blocks interaction with the UI beneath
	// it (pre-rebrand tracker #569); its own action button remains tappable.
	s.popUp.passThrough = true
	s.mu.Unlock()

	// Position at bottom center.
	canvasSize := c.Size()
	popSize := padded.MinSize()
	margin := th.Size(theme.SizeNamePadding) * 2
	x := (canvasSize.Width - popSize.Width) / 2
	y := canvasSize.Height - popSize.Height - margin
	if x < margin {
		x = margin
	}
	// Slide up from bottom.
	if bellui.CurrentApp().Settings().ShowAnimations() {
		startY := canvasSize.Height
		s.popUp.ShowAtPosition(geo.NewPos(x, startY))
		anim := bellui.NewAnimation(canvas.DurationShort4, func(done float32) {
			cy := startY + (y-startY)*done
			s.popUp.Move(geo.NewPos(x, cy))
		})
		anim.Curve = bellui.AnimationEaseStandardDecelerate
		anim.Start()
	} else {
		s.popUp.ShowAtPosition(geo.NewPos(x, y))
	}

	// A snackbar auto-dismisses before a screen-reader user would discover it
	// in the tree, so push the text as a polite live-region announcement
	// (pre-rebrand tracker #525). The action button stays a normal tree node; only the message is
	// announced.
	accessibility.Announce(s.Text, false)

	// Auto-dismiss.
	dur := s.Duration
	if dur <= 0 {
		dur = snackbarDefaultDuration
	}
	t := time.AfterFunc(dur, func() {
		// Hide touches popUp/timer, which the UI goroutine also mutates;
		// hop onto it rather than racing from the timer goroutine.
		bellui.Do(s.Hide)
	})
	s.mu.Lock()
	s.timer = t
	s.mu.Unlock()
}

// Hide dismisses the snackbar. It is safe to call from any goroutine.
func (s *Snackbar) Hide() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.actionTimer != nil {
		s.actionTimer.Stop()
		s.actionTimer = nil
	}
	p := s.popUp
	s.popUp = nil
	s.mu.Unlock()

	if p != nil {
		p.Hide()
	}
}

// ShowSnackbar is a convenience function to show a snackbar on the given canvas.
func ShowSnackbar(text, actionText string, action func(), c core.Canvas) {
	s := NewSnackbar(text, actionText, action)
	s.Show(c)
}
