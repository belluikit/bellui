package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/theme"
)

// TopAppBarAction is an action icon in the top app bar.
type TopAppBarAction struct {
	Icon     bellui.Resource
	OnTapped func()
}

var _ core.Accessible = (*TopAppBar)(nil)

// AccessibleRole returns [core.RoleToolbar].
func (t *TopAppBar) AccessibleRole() core.AccessibleRole { return core.RoleToolbar }

// AccessibleLabel returns the app bar title.
func (t *TopAppBar) AccessibleLabel() string { return t.Title }

// TopAppBar is an MD3 top application bar with title, navigation icon, and action icons.
//
// A disabled bar draws its navigation and action icons in the disabled
// colours, and those buttons refuse every press. They are the bar's own, built
// from NavigationIcon and Actions. The title keeps its colour: it takes no
// input, so only the controls look disabled (#1251).
type TopAppBar struct {
	DisableableWidget
	Title          string
	NavigationIcon bellui.Resource
	Actions        []TopAppBarAction
	OnNavigation   func() `json:"-"`
}

// NewTopAppBar creates a top app bar.
func NewTopAppBar(title string, navIcon bellui.Resource, navAction func()) *TopAppBar {
	t := &TopAppBar{Title: title, NavigationIcon: navIcon, OnNavigation: navAction}
	t.ExtendBaseWidget(t)
	return t
}

func (t *TopAppBar) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

func (t *TopAppBar) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)
	th := t.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bg := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerLow, v))
	divider := canvas.NewRectangle(th.Color(theme.ColorNameOutlineVariant, v))
	title := canvas.NewText(t.Title, th.Color(theme.ColorNameOnSurface, v))
	title.TextSize = th.Size(theme.SizeNameTitleLarge)
	title.TextStyle.Bold = true

	objects := []core.CanvasObject{bg, divider, title}

	var navBtn *IconButton
	if t.NavigationIcon != nil {
		navBtn = NewIconButton(t.NavigationIcon, t.OnNavigation)
		objects = append(objects, navBtn)
	}

	var actionBtns []*IconButton
	for _, a := range t.Actions {
		btn := NewIconButton(a.Icon, a.OnTapped)
		actionBtns = append(actionBtns, btn)
		objects = append(objects, btn)
	}

	r := &topAppBarRenderer{
		BaseRenderer: internalWidget.NewBaseRenderer(objects),
		bg:           bg,
		divider:      divider,
		title:        title,
		navBtn:       navBtn,
		actionBtns:   actionBtns,
		bar:          t,
	}
	r.Refresh()
	return r
}

type topAppBarRenderer struct {
	internalWidget.BaseRenderer
	bg         *canvas.Rectangle
	divider    *canvas.Rectangle
	title      *canvas.Text
	navBtn     *IconButton
	actionBtns []*IconButton
	bar        *TopAppBar
}

func (r *topAppBarRenderer) MinSize() geo.Size {
	th := r.bar.Theme()
	return geo.NewSize(200, th.Size(theme.SizeNameTopBarHeight))
}

func (r *topAppBarRenderer) Layout(size geo.Size) {
	r.bg.Resize(size)
	r.divider.Move(geo.NewPos(0, size.Height-1))
	r.divider.Resize(geo.NewSize(size.Width, 1))
	th := r.bar.Theme()
	btnSize := th.Size(theme.SizeNameIconButtonSize)
	padding := btnSize * 0.4 // 16 at default 40
	stride := btnSize + padding*0.5
	x := padding

	if r.navBtn != nil {
		r.navBtn.Move(geo.NewPos(x, (size.Height-btnSize)/2))
		r.navBtn.Resize(geo.NewSize(btnSize, btnSize))
		x += stride
	}

	// Actions from right
	ax := size.Width - padding*0.5
	for i := len(r.actionBtns) - 1; i >= 0; i-- {
		ax -= btnSize
		r.actionBtns[i].Move(geo.NewPos(ax, (size.Height-btnSize)/2))
		r.actionBtns[i].Resize(geo.NewSize(btnSize, btnSize))
		ax -= padding * 0.25
	}

	// Title, in the room between the navigation button and the first action
	// (the bar's end padding when there are none), keeping the same gap to an
	// action as the navigation button keeps to the title. A title that does
	// not fit ends in an ellipsis, so it never runs under an action (#1454).
	end := size.Width - padding
	if len(r.actionBtns) > 0 {
		end = r.actionBtns[0].Position().X - padding*0.5
	}
	r.title.TextSize = th.Size(theme.SizeNameTitleLarge)
	r.layoutTitle(x, end-x, size.Height)

	// Under a right-to-left layout, reflect every child horizontally so the
	// navigation icon and title sit at the trailing (right) edge and the action
	// icons at the leading (left) edge. The full-width bg/divider map onto
	// themselves, so a blanket pass is safe.
	mirrorIfRTL(r.Objects(), size.Width)
}

// layoutTitle places the title at x in a room wide, truncating it with an
// ellipsis when the whole title is wider, and drawing nothing when not even
// the ellipsis fits. The text box is resized to the drawn text's measured
// size, so the box width matches the glyphs: this keeps the painter's
// leading→trailing alignment flip a no-op and gives the RTL mirror pass the
// correct width to reflect.
func (r *topAppBarRenderer) layoutTitle(x, room, height float32) {
	shown := r.title.Text
	r.title.Text = r.bar.Title
	if r.title.MinSize().Width > room {
		runes := []rune(r.bar.Title)
		keep := 0
		if room > 0 {
			keep, _ = truncateLimit(r.bar.Title, r.title, room, []rune{'…'})
			keep = min(keep, len(runes))
		}
		// The shaper's measure and MinSize can disagree by a fraction of a
		// pixel, so trust MinSize, and shorten until it fits.
		for ; keep >= 0; keep-- {
			r.title.Text = string(runes[:keep]) + "…"
			if r.title.MinSize().Width <= room {
				break
			}
		}
		if keep < 0 {
			r.title.Text = "" // not even the ellipsis fits
		}
	}
	if r.title.Text != shown {
		r.title.Refresh()
	}
	titleSize := r.title.MinSize()
	r.title.Move(geo.NewPos(x, (height-titleSize.Height)/2))
	r.title.Resize(titleSize)
}

func (r *topAppBarRenderer) Refresh() {
	th := r.bar.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerLow, v)
	r.bg.Refresh()

	r.divider.FillColor = th.Color(theme.ColorNameOutlineVariant, v)
	r.divider.Refresh()

	// Layout below sets the title's text: the whole title, or as much of it as
	// fits (#1454).
	r.title.Color = th.Color(theme.ColorNameOnSurface, v) // also while disabled (#1251)
	r.title.Refresh()

	// Dynamically add/remove the navigation button.
	changed := false
	if r.bar.NavigationIcon != nil && r.navBtn == nil {
		r.navBtn = NewIconButton(r.bar.NavigationIcon, r.bar.OnNavigation)
		r.navBtn.disabled = r.bar.Disabled()
		changed = true
	} else if r.bar.NavigationIcon == nil && r.navBtn != nil {
		r.navBtn = nil
		changed = true
	} else if r.navBtn != nil {
		r.navBtn.Icon = r.bar.NavigationIcon
		r.navBtn.OnTapped = r.bar.OnNavigation
		r.navBtn.disabled = r.bar.Disabled()
		r.navBtn.Refresh()
	}

	// Rebuild action buttons only if the count changed.
	if len(r.bar.Actions) != len(r.actionBtns) {
		r.actionBtns = nil
		for _, a := range r.bar.Actions {
			btn := NewIconButton(a.Icon, a.OnTapped)
			btn.disabled = r.bar.Disabled()
			r.actionBtns = append(r.actionBtns, btn)
		}
		changed = true
	} else {
		for i, a := range r.bar.Actions {
			r.actionBtns[i].Icon = a.Icon
			r.actionBtns[i].OnTapped = a.OnTapped
			r.actionBtns[i].disabled = r.bar.Disabled()
			r.actionBtns[i].Refresh()
		}
	}

	if changed {
		r.SetObjects(r.collectObjects())
	}
	r.Layout(r.bar.Size())
	canvas.Refresh(r.bar.super())
}

func (r *topAppBarRenderer) collectObjects() []core.CanvasObject {
	objects := []core.CanvasObject{r.bg, r.divider, r.title}
	if r.navBtn != nil {
		objects = append(objects, r.navBtn)
	}
	for _, btn := range r.actionBtns {
		objects = append(objects, btn)
	}
	return objects
}
