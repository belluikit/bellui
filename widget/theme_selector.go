package widget

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalapp "github.com/belluikit/bellui/internal/app"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

// ThemeVariantSelection is the variant choice of a [ThemeSelection]: follow
// the system appearance, always light, or always dark.
//
// Since: 1.0
type ThemeVariantSelection int

const (
	// ThemeVariantSystem follows the operating system light/dark appearance,
	// including later changes to it (see [ThemeSelection.Apply]).
	ThemeVariantSystem ThemeVariantSelection = iota
	// ThemeVariantLight always uses the light theme variant.
	ThemeVariantLight
	// ThemeVariantDark always uses the dark theme variant.
	ThemeVariantDark
)

// String returns a stable textual form ("system", "light" or "dark"),
// also used when a selection is stored in Preferences.
func (v ThemeVariantSelection) String() string {
	switch v {
	case ThemeVariantLight:
		return "light"
	case ThemeVariantDark:
		return "dark"
	default:
		return "system"
	}
}

// ThemeSelection is the state of a [ThemeSelector]: a theme variant choice
// and a named primary color (one of [theme.PrimaryColorNames]).
//
// Since: 1.0
type ThemeSelection struct {
	Variant ThemeVariantSelection
	Primary string
}

// Apply re-themes the running app to this selection via its Settings.
// It only affects the current app — nothing is written to the system-wide
// Bellui settings. [ThemeVariantLight] and [ThemeVariantDark] pin the variant:
// later OS appearance changes leave it alone. [ThemeVariantSystem] withdraws
// any pin, so the app follows the operating system appearance live, when the
// app settings implement [bellui.ThemeVariantSettings] (the default app and
// the test app do). Other settings get the operating system appearance at
// call time instead, pinned like an explicit choice.
// The primary color is applied when the app settings implement
// [bellui.PrimaryColorSettings] (the default app and the test app do);
// an empty Primary leaves the current primary color untouched.
func (t ThemeSelection) Apply() {
	a := bellui.CurrentApp()
	if a == nil {
		return
	}
	s := a.Settings()

	if t.Primary != "" {
		if pc, ok := s.(bellui.PrimaryColorSettings); ok {
			pc.SetPrimaryColor(t.Primary)
		}
	}

	switch t.Variant {
	case ThemeVariantLight:
		s.SetThemeVariant(theme.VariantLight)
	case ThemeVariantDark:
		s.SetThemeVariant(theme.VariantDark)
	default:
		if tv, ok := s.(bellui.ThemeVariantSettings); ok {
			tv.ClearThemeVariant()
			return
		}
		// Settings that cannot withdraw a pin get a snapshot of the OS
		// appearance, the behaviour before live following existed.
		s.SetThemeVariant(internalapp.DefaultVariant())
	}
}

const (
	themeSelectionVariantKey = "bellui.themeselector.variant"
	themeSelectionPrimaryKey = "bellui.themeselector.primary"
)

// SaveThemeSelection stores sel in the given preferences, typically an app's
// own Preferences, so the appearance choice follows the app rather than the
// system-wide Bellui settings. Restore it with [LoadThemeSelection].
//
// Since: 1.0
func SaveThemeSelection(sel ThemeSelection, p bellui.Preferences) {
	bellui.SetPref(p, themeSelectionVariantKey, sel.Variant.String())
	bellui.SetPref(p, themeSelectionPrimaryKey, sel.Primary)
}

// LoadThemeSelection restores a selection stored by [SaveThemeSelection].
// The boolean reports whether a stored selection was found; call
// [ThemeSelection.Apply] on the result at startup to restore the appearance.
//
// Since: 1.0
func LoadThemeSelection(p bellui.Preferences) (ThemeSelection, bool) {
	stored := bellui.PrefOr(p, themeSelectionVariantKey, "")
	if stored == "" {
		return ThemeSelection{}, false
	}

	sel := ThemeSelection{Primary: bellui.PrefOr(p, themeSelectionPrimaryKey, "")}
	switch stored {
	case "light":
		sel.Variant = ThemeVariantLight
	case "dark":
		sel.Variant = ThemeVariantDark
	}
	return sel, true
}

// ThemeSelector is a compact appearance picker for app settings pages: a
// System/Light/Dark variant selector above the standard primary color
// swatches. It is callback-driven — interacting updates Selection and fires
// OnChanged, but nothing is applied or persisted by the widget itself.
// Use [ThemeSelection.Apply] and [SaveThemeSelection] for that.
//
// Since: 1.0
type ThemeSelector struct {
	BaseWidget

	// Selection is the current choice shown by the widget.
	Selection ThemeSelection
	// OnChanged fires whenever the user changes the selection.
	OnChanged func(ThemeSelection) `json:"-"`
}

// NewThemeSelector creates a theme selector. The initial selection is the
// system variant with the app's current primary color.
//
// Since: 1.0
func NewThemeSelector(changed func(ThemeSelection)) *ThemeSelector {
	t := &ThemeSelector{OnChanged: changed}
	if a := bellui.CurrentApp(); a != nil {
		t.Selection.Primary = a.Settings().PrimaryColor()
	}
	if t.Selection.Primary == "" {
		t.Selection.Primary = theme.ColorBlue
	}
	t.ExtendBaseWidget(t)
	return t
}

// SetSelection updates the shown selection, firing OnChanged if it changed.
func (t *ThemeSelector) SetSelection(sel ThemeSelection) {
	if sel == t.Selection {
		return
	}
	t.Selection = sel
	if t.OnChanged != nil {
		t.OnChanged(sel)
	}
	t.Refresh()
}

// MinSize returns the size that this widget should not shrink below.
func (t *ThemeSelector) MinSize() geo.Size {
	t.ExtendBaseWidget(t)
	return t.BaseWidget.MinSize()
}

// CreateRenderer returns a new renderer for the theme selector.
func (t *ThemeSelector) CreateRenderer() core.WidgetRenderer {
	t.ExtendBaseWidget(t)

	variants := NewSegmentedButton(
		[]string{lang.L("System"), lang.L("Light"), lang.L("Dark")},
		func(index int) { t.setVariant(ThemeVariantSelection(index)) },
	)
	variants.Selected = int(t.Selection.Variant)

	names := theme.PrimaryColorNames()
	swatches := make([]*themeSwatch, len(names))
	objects := make([]core.CanvasObject, len(names))
	for i, name := range names {
		s := &themeSwatch{name: name, owner: t}
		s.ExtendBaseWidget(s)
		swatches[i] = s
		objects[i] = s
	}
	row := &bellui.Container{Layout: layout.NewGridLayout(len(objects)), Objects: objects}
	content := &bellui.Container{
		Layout:  layout.NewVBoxLayout(),
		Objects: []core.CanvasObject{variants, row},
	}

	return &themeSelectorRenderer{selector: t, variants: variants, swatches: swatches, content: content}
}

func (t *ThemeSelector) setVariant(v ThemeVariantSelection) {
	if t.Selection.Variant == v {
		return
	}
	t.Selection.Variant = v
	if t.OnChanged != nil {
		t.OnChanged(t.Selection)
	}
}

func (t *ThemeSelector) setPrimary(name string) {
	if t.Selection.Primary == name {
		return
	}
	t.Selection.Primary = name
	if t.OnChanged != nil {
		t.OnChanged(t.Selection)
	}
	t.Refresh()
}

type themeSelectorRenderer struct {
	selector *ThemeSelector
	variants *SegmentedButton
	swatches []*themeSwatch
	content  *bellui.Container
}

func (r *themeSelectorRenderer) Layout(size geo.Size) {
	r.content.Resize(size)
}

func (r *themeSelectorRenderer) MinSize() geo.Size {
	return r.content.MinSize()
}

func (r *themeSelectorRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.content}
}

func (r *themeSelectorRenderer) Refresh() {
	if v := int(r.selector.Selection.Variant); v != r.variants.Selected {
		r.variants.Selected = v
		r.variants.Refresh()
	}
	for _, s := range r.swatches {
		s.Refresh()
	}
}

func (r *themeSelectorRenderer) Destroy() {
}

// themeSwatch is a single tappable primary color dot with a selected ring.
type themeSwatch struct {
	BaseWidget
	name  string
	owner *ThemeSelector
}

var (
	_ event.Tappable  = (*themeSwatch)(nil)
	_ core.Accessible = (*themeSwatch)(nil)
)

// AccessibleRole returns [core.RoleButton].
func (s *themeSwatch) AccessibleRole() core.AccessibleRole { return core.RoleButton }

// AccessibleLabel returns the primary color name this swatch selects.
func (s *themeSwatch) AccessibleLabel() string { return s.name }

// Tapped makes this swatch's color the selected primary color.
func (s *themeSwatch) Tapped(_ *event.PointEvent) {
	s.owner.setPrimary(s.name)
}

// CreateRenderer returns a new renderer for the swatch.
func (s *themeSwatch) CreateRenderer() core.WidgetRenderer {
	dot := &canvas.Circle{}
	ring := &canvas.Circle{FillColor: color.Transparent, StrokeWidth: 2}

	r := &themeSwatchRenderer{swatch: s, dot: dot, ring: ring}
	r.Refresh()
	return r
}

type themeSwatchRenderer struct {
	swatch    *themeSwatch
	dot, ring *canvas.Circle
}

func (r *themeSwatchRenderer) MinSize() geo.Size {
	d := r.swatch.Theme().Size(theme.SizeNameIconButtonSize) * 0.8
	return geo.NewSize(d, d)
}

func (r *themeSwatchRenderer) Layout(size geo.Size) {
	outer := size.Width
	if size.Height < outer {
		outer = size.Height
	}
	r.ring.Move(geo.NewPos((size.Width-outer)/2, (size.Height-outer)/2))
	r.ring.Resize(geo.NewSize(outer, outer))

	dot := outer - 12
	if dot < 8 {
		dot = 8
	}
	r.dot.Move(geo.NewPos((size.Width-dot)/2, (size.Height-dot)/2))
	r.dot.Resize(geo.NewSize(dot, dot))
}

func (r *themeSwatchRenderer) Objects() []core.CanvasObject {
	return []core.CanvasObject{r.ring, r.dot}
}

func (r *themeSwatchRenderer) Refresh() {
	c := internaltheme.PrimaryColorNamed(r.swatch.name)
	r.dot.FillColor = c
	if r.swatch.owner.Selection.Primary == r.swatch.name {
		r.ring.StrokeColor = c
	} else {
		r.ring.StrokeColor = color.Transparent
	}
	r.dot.Refresh()
	r.ring.Refresh()
}

func (r *themeSwatchRenderer) Destroy() {
}
