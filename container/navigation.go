package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// Navigation container is used to provide your application with a control bar and an area for content objects.
// Objects can be any CanvasObject, and only the most recent one will be visible.
//
// Since: 1.0
type Navigation struct {
	widget.BaseWidget

	Root      core.CanvasObject
	Title     string
	OnBack    func() `json:"-"`
	OnForward func() `json:"-"`

	level  int
	stack  bellui.Container
	titles []string
}

// NewNavigation creates a new navigation container with a given root object.
//
// Since: 1.0
func NewNavigation(root core.CanvasObject) *Navigation {
	return NewNavigationWithTitle(root, "")
}

// NewNavigationWithTitle creates a new navigation container with a given root object and a default title.
//
// Since: 1.0
func NewNavigationWithTitle(root core.CanvasObject, s string) *Navigation {
	var nav *Navigation
	nav = &Navigation{
		Root:      root,
		Title:     s,
		OnBack:    func() { _ = nav.Back() },
		OnForward: func() { _ = nav.Forward() },
	}
	nav.ExtendBaseWidget(nav)
	return nav
}

// Push puts the given object on top of the navigation stack and hides the object below.
//
// Since: 1.0
func (nav *Navigation) Push(obj core.CanvasObject) {
	nav.PushWithTitle(obj, nav.Title)
}

// PushWithTitle puts the given CanvasObject on top, hides the object below, and uses the given title as label text.
//
// Since: 1.0
func (nav *Navigation) PushWithTitle(obj core.CanvasObject, s string) {
	obj.Show()
	objs := nav.stack.Objects[:nav.level]
	if len(objs) > 0 {
		objs[len(objs)-1].Hide()
	}
	nav.stack.Objects = append(objs, obj)
	nav.titles = append(nav.titles[:nav.level], s)
	nav.level++
	nav.Refresh()
}

// Back returns the top level CanvasObject, adjusts the title accordingly, and disabled the back button
// when no more objects are left to go back to.
//
// Since: 1.0
func (nav *Navigation) Back() core.CanvasObject {
	if nav.level == 0 || nav.level == 1 && nav.Root != nil {
		return nil
	}

	objs := nav.stack.Objects
	objs[nav.level-1].Hide()
	if nav.level > 1 {
		objs[nav.level-2].Show()
	}

	nav.level--
	nav.Refresh()

	return objs[nav.level]
}

// Forward shows the next object in the stack again
// and returns the object that was visible before.
//
// Since: 1.0
func (nav *Navigation) Forward() core.CanvasObject {
	if nav.level >= len(nav.stack.Objects) {
		return nil
	}

	nav.stack.Objects[nav.level-1].Hide()
	nav.stack.Objects[nav.level].Show()
	nav.level++
	nav.Refresh()

	return nav.stack.Objects[nav.level-1]
}

// SetTitle changes the root navigation title shown by default.
//
// Since: 1.0
func (nav *Navigation) SetTitle(s string) {
	nav.Title = s

	if len(nav.titles) > 0 {
		nav.titles[0] = s
	}

	nav.Refresh()
}

// SetCurrentTitle changes the navigation title for the current level.
//
// Since: 1.0
func (nav *Navigation) SetCurrentTitle(s string) {
	if nav.level > 1 && nav.level-1 < len(nav.titles) {
		nav.titles[nav.level-1] = s
		nav.Refresh()
	}
}

func (nav *Navigation) setup() {
	objs := []core.CanvasObject{}
	titles := []string{}
	if nav.Root != nil {
		objs = append(objs, nav.Root)
		titles = append(titles, nav.Title)
	}
	nav.level = len(objs)
	nav.stack.Layout = layout.NewStackLayout()
	nav.stack.Objects = objs
	nav.titles = titles
	nav.ExtendBaseWidget(nav)
}

var _ core.WidgetRenderer = (*navigatorRenderer)(nil)

type navigatorRenderer struct {
	nav     *Navigation
	back    widget.Button
	forward widget.Button
	title   widget.Label
	object  core.CanvasObject
}

func (nav *Navigation) CreateRenderer() core.WidgetRenderer {
	r := &navigatorRenderer{
		nav: nav,
		title: widget.Label{
			Text:       nav.Title,
			Alignment:  bellui.TextAlignCenter,
			Truncation: bellui.TextTruncateEllipsis,
		},
		back: widget.Button{
			Icon:     theme.NavigateBackIcon(),
			OnTapped: nav.OnBack,
		},
		forward: widget.Button{
			Icon:     theme.NavigateNextIcon(),
			OnTapped: nav.OnForward,
		},
	}
	r.back.Disable()
	r.forward.Disable()

	nav.setup()

	pad := r.back.MinSize().Width
	r.object = NewBorder(
		NewStack(NewHBox(&r.back, layout.NewSpacer(), &r.forward),
			&bellui.Container{Layout: layout.NewCustomPaddedLayout(0, 0, pad, pad), Objects: []core.CanvasObject{&r.title}}),
		nil,
		nil,
		nil,
		&nav.stack,
	)

	return r
}

func (r *navigatorRenderer) Destroy() {
}

func (r *navigatorRenderer) Layout(s geo.Size) {
	r.object.Resize(s)
}

func (r *navigatorRenderer) MinSize() geo.Size {
	return r.object.MinSize()
}

func (r *navigatorRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.object}
}

func (r *navigatorRenderer) Refresh() {
	if r.nav.level < 1 || r.nav.level == 1 && r.nav.Root != nil {
		r.back.Disable()
	} else {
		r.back.Enable()
	}

	if r.nav.level == len(r.nav.stack.Objects) {
		r.forward.Disable()
	} else {
		r.forward.Enable()
	}

	if r.nav.level-1 >= 0 && r.nav.level-1 < len(r.nav.titles) {
		r.title.Text = r.nav.titles[r.nav.level-1]
	} else {
		r.title.Text = r.nav.Title
	}

	r.object.Refresh()
}
