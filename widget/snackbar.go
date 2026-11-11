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

// canvasSnackbars holds the Snackbar showing on each canvas (core.Canvas →
// *Snackbar). MD3 shows one snackbar at a time, so Show takes the canvas's
// current one off first, whichever Snackbar value showed it (#1038).
var canvasSnackbars sync.Map

// Snackbar shows a brief message at the bottom of the screen with an optional action button.
// It auto-dismisses after Duration.
type Snackbar struct {
	Text       string
	ActionText string
	OnAction   func() `json:"-"`
	Duration   time.Duration

	// BottomInset raises the snackbar this many dp above where it would
	// otherwise sit, so it clears controls pinned to the bottom of the
	// canvas, such as a player's transport bar. Zero keeps the default
	// place at the bottom edge.
	//
	// Since: 1.3
	BottomInset float32

	canvas core.Canvas

	mu          sync.Mutex
	popUp       *PopUp
	slide       *bellui.Animation // the slide-up; Hide stops it
	fading      *PopUp            // the popup Hide is fading out; Show removes it at once
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
//
// A snackbar already showing on the canvas, from this Snackbar or another, is
// taken off at once: MD3 shows one at a time.
func (s *Snackbar) Show(c core.Canvas) {
	s.removeNow() // dismiss any previous
	if prev, ok := canvasSnackbars.Swap(c, s); ok && prev != s {
		prev.(*Snackbar).removeNow()
	}
	s.mu.Lock()
	s.canvas = c
	s.mu.Unlock()

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bgColor := th.Color(theme.ColorNameInverseSurface, v)

	label := NewRichText(&TextSegment{Text: s.Text, Style: RichTextStyle{
		ColorName: theme.ColorNameInverseOnSurface,
		Inline:    true,
	}})
	label.Wrapping = bellui.TextWrapWord
	msg := &snackbarLayout{
		text:   label,
		outset: th.Size(theme.SizeNameInnerPadding),
		pad:    th.Size(theme.SizeNamePadding),
	}

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

		objects = append(objects, actionBtn)
		msg.action = actionBtn
	}

	content := &bellui.Container{Layout: msg, Objects: objects}
	padded := &bellui.Container{Layout: layout.NewPaddedLayout(), Objects: []core.CanvasObject{content}}

	// The message keeps to one line while it fits, and otherwise wraps at
	// the width the canvas leaves inside the snackbar's margins (#1106).
	margin := th.Size(theme.SizeNamePadding) * 2
	room := c.Size().Width - 2*margin - 2*msg.pad // the padded layout's insets
	if msg.action != nil {
		room -= msg.pad + msg.action.MinSize().Width
	}
	oneLine := bellui.MeasureText(s.Text, th.Size(theme.SizeNameText), bellui.TextStyle{}).Width
	msg.textWidth = max(min(oneLine, room), 1)

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
	x := (canvasSize.Width - popSize.Width) / 2
	y := canvasSize.Height - popSize.Height - margin
	if s.BottomInset > 0 {
		y -= s.BottomInset
	}
	if x < margin {
		x = margin
	}
	// Slide up from bottom.
	if bellui.CurrentApp().Settings().ShowAnimations() {
		startY := canvasSize.Height
		s.popUp.ShowAtPosition(geo.NewPos(x, startY))
		// The tick moves this popup, not s.popUp: Hide drops s.popUp, and a
		// tick that lands after it moved a nil popup and crashed the app
		// (belluikit/player#342). Hide also stops the slide.
		p := s.popUp
		anim := bellui.NewAnimation(canvas.DurationShort4, func(done float32) {
			cy := startY + (y-startY)*done
			p.Move(geo.NewPos(x, cy))
		})
		anim.Curve = bellui.AnimationEaseStandardDecelerate
		s.mu.Lock()
		s.slide = anim
		s.mu.Unlock()
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

// removeNow hides the snackbar and takes its popup off the canvas at once
// instead of letting it fade, so a snackbar replacing it shows alone.
func (s *Snackbar) removeNow() {
	s.Hide()
	s.mu.Lock()
	f := s.fading
	s.fading = nil
	s.mu.Unlock()
	if f != nil {
		f.removeNow()
	}
}

// Hide dismisses the snackbar. It is safe to call from any goroutine.
func (s *Snackbar) Hide() {
	s.mu.Lock()
	if s.canvas != nil {
		canvasSnackbars.CompareAndDelete(s.canvas, s)
	}
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.actionTimer != nil {
		s.actionTimer.Stop()
		s.actionTimer = nil
	}
	slide := s.slide
	s.slide = nil
	p := s.popUp
	s.popUp = nil
	if p != nil {
		s.fading = p
	}
	s.mu.Unlock()

	if slide != nil {
		slide.Stop()
	}
	if p != nil {
		p.Hide()
	}
}

// snackbarLayout lays out a snackbar's message and its optional action: the
// message wraps at textWidth, and the action sits at the end, centred. The
// message's RichText draws its text outset in from its edges, so the layout
// places it that far outside the text's area, putting the text where a
// single canvas.Text line used to be (#1106).
type snackbarLayout struct {
	text      *RichText
	action    core.CanvasObject
	textWidth float32
	outset    float32
	pad       float32 // between the message and the action
}

// textSize is the size of the message's text, wrapped at textWidth.
func (l *snackbarLayout) textSize() geo.Size {
	if w := l.textWidth + 2*l.outset; l.text.Size().Width != w {
		l.text.Resize(geo.NewSize(w, l.text.Size().Height))
	}
	return geo.NewSize(l.textWidth, l.text.MinSize().Height-2*l.outset)
}

func (l *snackbarLayout) MinSize([]core.CanvasObject) geo.Size {
	size := l.textSize()
	if l.action != nil {
		a := l.action.MinSize()
		size = geo.NewSize(size.Width+l.pad+a.Width, max(size.Height, a.Height))
	}
	return size
}

func (l *snackbarLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	text := l.textSize()
	l.text.Move(geo.NewPos(-l.outset, (size.Height-text.Height)/2-l.outset))
	l.text.Resize(text.AddWidthHeight(2*l.outset, 2*l.outset))
	if l.action != nil {
		a := l.action.MinSize()
		l.action.Move(geo.NewPos(size.Width-a.Width, (size.Height-a.Height)/2))
		l.action.Resize(a)
	}
	mirrorIfRTL(objects, size.Width) // the action leads in a right-to-left layout, as the box it replaced did
}

// ShowSnackbar is a convenience function to show a snackbar on the given canvas.
func ShowSnackbar(text, actionText string, action func(), c core.Canvas) {
	s := NewSnackbar(text, actionText, action)
	s.Show(c)
}
