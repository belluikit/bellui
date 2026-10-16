package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	intTheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/widget"
)

// ThemeOverride is a container where the child widgets are themed by the specified theme.
// Containers will be traversed and all child widgets will reflect the theme in this container.
// This should be used sparingly to avoid a jarring user experience.
//
// Since: 1.0
type ThemeOverride struct {
	widget.BaseWidget

	Content core.CanvasObject
	Theme   bellui.Theme

	holder *bellui.Container

	mobile bool
}

// NewThemeOverride provides a container where the child widgets are themed by the specified theme.
// Containers will be traversed and all child widgets will reflect the theme in this container.
// This should be used sparingly to avoid a jarring user experience.
//
// If the content `obj` of this theme override is a container and items are later added to the container or any
// sub-containers ensure that you call `Refresh()` on this `ThemeOverride` to ensure the new items match the theme.
//
// Since: 1.0
func NewThemeOverride(obj core.CanvasObject, th bellui.Theme) *ThemeOverride {
	t := &ThemeOverride{Content: obj, Theme: th, holder: NewStack(obj)}
	t.ExtendBaseWidget(t)

	cache.OverrideTheme(obj, addFeatures(th, t))
	obj.Refresh() // required as the widgets passed in could have been initially rendered with default theme
	return t
}

func (t *ThemeOverride) CreateRenderer() core.WidgetRenderer {
	cache.OverrideTheme(t.Content, addFeatures(t.Theme, t))

	return &overrideRenderer{parent: t, objs: []core.CanvasObject{t.holder}}
}

func (t *ThemeOverride) Refresh() {
	if t.holder.Objects[0] != t.Content {
		t.holder.Objects[0] = t.Content
		t.holder.Refresh()
	}

	cache.OverrideTheme(t.Content, addFeatures(t.Theme, t))
	t.Content.Refresh()
	t.BaseWidget.Refresh()
}

// SetDeviceIsMobile allows a ThemeOverride container to shape the contained widgets as a mobile device.
// This will impact containers such as AppTabs and DocTabs, and more in the future, to display a layout
// that would automatically be used for a mobile device runtime.
//
// Since: 1.0
func (t *ThemeOverride) SetDeviceIsMobile(on bool) {
	t.mobile = on
	t.BaseWidget.Refresh()
}

type featureTheme struct {
	bellui.Theme

	over *ThemeOverride
}

func addFeatures(th bellui.Theme, o *ThemeOverride) bellui.Theme {
	return &featureTheme{Theme: th, over: o}
}

func (f *featureTheme) DeviceIsMobile() bool {
	return f.over.mobile
}

func (f *featureTheme) RenderStyle() intTheme.RenderStyle {
	if rs, ok := f.Theme.(intTheme.RenderStyleTheme); ok {
		return rs.RenderStyle()
	}
	return intTheme.RenderStyleClassic
}

type overrideRenderer struct {
	parent *ThemeOverride

	objs []core.CanvasObject
}

func (r *overrideRenderer) Destroy() {
}

func (r *overrideRenderer) Layout(s geo.Size) {
	intTheme.PushRenderingTheme(r.parent.Theme)
	defer intTheme.PopRenderingTheme()

	r.parent.holder.Resize(s)
}

func (r *overrideRenderer) MinSize() geo.Size {
	intTheme.PushRenderingTheme(r.parent.Theme)
	defer intTheme.PopRenderingTheme()

	return r.parent.Content.MinSize()
}

func (r *overrideRenderer) Objects() []core.CanvasObject {
	return r.objs
}

func (r *overrideRenderer) Refresh() {
}
