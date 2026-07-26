package widget

import (
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/theme"
)

const defaultToastDuration = 3 * time.Second

// ShowToast shows a transient notification at the bottom of the canvas.
// The toast auto-dismisses after 3 seconds.
//
// Since: 2.7
func ShowToast(text string, c core.Canvas) *Toast {
	return ShowToastWithImportance(text, MediumImportance, c)
}

// ShowToastWithImportance shows a styled transient notification at the bottom of the canvas.
// The returned Toast can be used to dismiss the notification early; its
// auto-dismiss timer keeps running otherwise, so callers that tear the
// canvas down before Duration elapses (e.g. tests) should call Dismiss.
//
// Since: 2.7
func ShowToastWithImportance(text string, importance Importance, c core.Canvas) *Toast {
	t := newToast(text, importance, c)
	t.Show()
	return t
}

var _ core.Accessible = (*Toast)(nil)

// AccessibleRole returns [core.RoleStatus].
func (t *Toast) AccessibleRole() core.AccessibleRole { return core.RoleStatus }

// AccessibleLabel returns the toast text.
func (t *Toast) AccessibleLabel() string { return t.Text }

// Toast is a transient notification that appears briefly and auto-dismisses.
//
// Since: 2.7
type Toast struct {
	BaseWidget

	Text       string
	Importance Importance
	Duration   time.Duration

	canvas     core.Canvas
	popUp      *PopUp
	timer      *time.Timer
	mu         sync.Mutex
	dismissing bool
}

func newToast(text string, importance Importance, c core.Canvas) *Toast {
	t := &Toast{
		Text:       text,
		Importance: importance,
		Duration:   defaultToastDuration,
		canvas:     c,
	}
	t.ExtendBaseWidget(t)
	return t
}

// Show displays the toast on the canvas and starts the auto-dismiss timer.
func (t *Toast) Show() {
	if t.canvas == nil {
		return
	}

	th := t.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	fgName, bgName := t.colors()

	bgRect := canvas.NewRectangle(th.Color(bgName, v))
	bgRect.CornerRadius = th.Size(theme.SizeNameInputRadius)

	label := canvas.NewText(t.Text, th.Color(fgName, v))
	label.TextSize = th.Size(theme.SizeNameText)
	label.Alignment = bellui.TextAlignCenter

	pad := th.Size(theme.SizeNameInnerPadding)
	content := &bellui.Container{
		Layout:  &toastContentLayout{padding: pad},
		Objects: []core.CanvasObject{bgRect, label},
	}

	t.mu.Lock()
	t.popUp = NewPopUp(content, t.canvas)
	// A toast is a transient notification, not a dialog: it must not block
	// taps, scrolling or typing aimed at the UI beneath it (#569).
	t.popUp.passThrough = true
	t.mu.Unlock()

	pos := t.toastPos(content.MinSize())
	t.popUp.ShowAtPosition(pos)

	// A toast auto-dismisses before a screen-reader user would discover it in
	// the tree, so push the text as a live-region announcement (#525). Danger
	// and warning toasts interrupt current speech; the rest queue politely.
	assertive := t.Importance == DangerImportance || t.Importance == WarningImportance
	accessibility.Announce(t.Text, assertive)

	dur := t.Duration
	if dur == 0 {
		dur = defaultToastDuration
	}
	t.mu.Lock()
	t.timer = time.AfterFunc(dur, func() {
		bellui.Do(func() {
			t.Dismiss()
		})
	})
	t.mu.Unlock()
}

// Dismiss hides the toast immediately and cancels the auto-dismiss timer.
func (t *Toast) Dismiss() {
	t.mu.Lock()
	if t.dismissing {
		t.mu.Unlock()
		return
	}
	t.dismissing = true
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	p := t.popUp
	t.popUp = nil
	t.mu.Unlock()

	if p != nil {
		p.Hide()
	}
}

// CreateRenderer returns a minimal renderer (Toast is shown via PopUp, not inline).
func (t *Toast) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	return &toastRenderer{toast: t, bg: canvas.NewRectangle(nil)}
}

type toastRenderer struct {
	toast *Toast
	bg    *canvas.Rectangle
}

func (r *toastRenderer) Destroy()                         { r.toast.Dismiss() }
func (r *toastRenderer) Layout(_ geo.Size)                {}
func (r *toastRenderer) MinSize() geo.Size                { return geo.Size{} }
func (r *toastRenderer) Objects() []core.CanvasObject     { return []core.CanvasObject{r.bg} }
func (r *toastRenderer) SetObjects(_ []core.CanvasObject) {}
func (r *toastRenderer) Refresh()                         {}

func (t *Toast) colors() (bellui.ThemeColorName, bellui.ThemeColorName) {
	switch t.Importance {
	case DangerImportance:
		return theme.ColorNameForegroundOnError, theme.ColorNameError
	case SuccessImportance:
		return theme.ColorNameForegroundOnSuccess, theme.ColorNameSuccess
	case WarningImportance:
		return theme.ColorNameForegroundOnWarning, theme.ColorNameWarning
	default:
		return theme.ColorNameForeground, theme.ColorNameOverlayBackground
	}
}

func (t *Toast) toastPos(contentSize geo.Size) geo.Position {
	canvasSize := t.canvas.Size()
	padding := theme.Padding() * 4
	x := (canvasSize.Width - contentSize.Width) / 2
	y := canvasSize.Height - contentSize.Height - padding
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return geo.NewPos(x, y)
}

// --- toastContentLayout: background fills area, text is padded ---

type toastContentLayout struct {
	padding float32
}

func (l *toastContentLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	if len(objects) < 2 {
		return
	}
	objects[0].Move(geo.NewPos(0, 0))
	objects[0].Resize(size)
	objects[1].Move(geo.NewPos(l.padding, l.padding))
	objects[1].Resize(geo.NewSize(size.Width-l.padding*2, size.Height-l.padding*2))
}

func (l *toastContentLayout) MinSize(objects []core.CanvasObject) geo.Size {
	if len(objects) < 2 {
		return geo.NewSize(0, 0)
	}
	textMin := objects[1].MinSize()
	return geo.NewSize(textMin.Width+l.padding*2, textMin.Height+l.padding*2)
}
