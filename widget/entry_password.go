package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var (
	_ desktop.Cursorable = (*passwordRevealer)(nil)
	_ event.Tappable     = (*passwordRevealer)(nil)
	_ core.Widget        = (*passwordRevealer)(nil)
)

type passwordRevealer struct {
	BaseWidget
	pressTarget

	icon  *canvas.Image
	entry *Entry
}

func newPasswordRevealer(e *Entry) *passwordRevealer {
	th := e.Theme()
	pr := &passwordRevealer{
		icon:  canvas.NewImageFromResource(th.Icon(theme.IconNameVisibilityOff)),
		entry: e,
	}
	pr.ExtendBaseWidget(pr)
	return pr
}

func (r *passwordRevealer) CreateRenderer() core.WidgetRenderer {
	ripple := internalwidget.AttachPress(&r.pressTarget, r.pressShape)
	return &passwordRevealerRenderer{
		BaseRenderer: internalwidget.NewBaseRenderer([]core.CanvasObject{ripple, r.icon}),
		icon:         r.icon,
		entry:        r.entry,
	}
}

func (r *passwordRevealer) Cursor() desktop.Cursor {
	return desktop.DefaultCursor
}

// AccessibleRole returns [core.RoleButton].
func (r *passwordRevealer) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns a localized "Show password" / "Hide password"
// depending on which action a press would perform.
func (r *passwordRevealer) AccessibleLabel() string {
	if r.entry.Password {
		return lang.L("Show password")
	}
	return lang.L("Hide password")
}

// AccessibleState reports the disabled state of the owning entry.
func (r *passwordRevealer) AccessibleState() core.AccessibleState {
	if r.entry.Disabled() {
		return core.StateDisabled
	}
	return 0
}

// AccessibleActions returns the "press" action.
func (r *passwordRevealer) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Toggle password visibility"}}
}

// PerformAccessibleAction toggles password visibility like a tap would.
func (r *passwordRevealer) PerformAccessibleAction(name string) {
	if name == core.ActionPress {
		r.Tapped(nil)
	}
}

func (r *passwordRevealer) Tapped(ev *event.PointEvent) {
	if r.entry.Disabled() {
		return
	}
	internalwidget.TapPress(&r.pressTarget, ev)

	r.entry.setFieldsAndRefresh(func() {
		r.entry.Password = !r.entry.Password
	})
	if c := bellui.CurrentApp().Driver().CanvasForObject(r); c != nil {
		c.Focus(r.entry.super().(event.Focusable))
	}
}

// HoverIn does nothing: the button is Hoverable for HoverOut.
func (r *passwordRevealer) HoverIn(*event.HoverEvent) {}

// HoverMoved does nothing.
func (r *passwordRevealer) HoverMoved(*event.HoverEvent) {}

// HoverOut ends a press the pointer took off the button: it gets no MouseUp
// for a release elsewhere.
func (r *passwordRevealer) HoverOut() {
	internalwidget.CancelPress(&r.pressTarget)
}

// pressShape is a circle around the eye icon, in its colour, as for an icon
// button (#1091).
func (r *passwordRevealer) pressShape() (internalwidget.PressShape, bool) {
	if r.entry.Disabled() {
		return internalwidget.PressShape{}, false
	}
	size := r.Size()
	d := geo.Min(size.Width, size.Height)
	th := r.entry.Theme()
	return internalwidget.PressShape{
		Pos:          geo.NewPos((size.Width-d)/2, (size.Height-d)/2),
		Size:         geo.NewSquareSize(d),
		CornerRadius: d / 2,
		Content:      th.Color(theme.ColorNameOnSurfaceVariant, bellui.CurrentApp().Settings().ThemeVariant()),
		FromCenter:   true,
	}, true
}

var _ core.WidgetRenderer = (*passwordRevealerRenderer)(nil)

type passwordRevealerRenderer struct {
	internalwidget.BaseRenderer
	entry *Entry
	icon  *canvas.Image
}

func (r *passwordRevealerRenderer) Layout(size geo.Size) {
	iconSize := r.entry.Theme().Size(theme.SizeNameInlineIcon)
	r.icon.Resize(geo.NewSquareSize(iconSize))
	r.icon.Move(geo.NewPos((size.Width-iconSize)/2, (size.Height-iconSize)/2))
}

func (r *passwordRevealerRenderer) MinSize() geo.Size {
	iconSize := r.entry.Theme().Size(theme.SizeNameInlineIcon)
	return geo.NewSquareSize(iconSize + r.entry.Theme().Size(theme.SizeNameInnerPadding)*2)
}

func (r *passwordRevealerRenderer) Refresh() {
	th := r.entry.Theme()
	if !r.entry.Password {
		r.icon.Resource = th.Icon(theme.IconNameVisibility)
	} else {
		r.icon.Resource = th.Icon(theme.IconNameVisibilityOff)
	}

	if r.entry.Disabled() {
		r.icon.Resource = theme.NewDisabledResource(r.icon.Resource)
	}
	r.icon.Refresh()
}
