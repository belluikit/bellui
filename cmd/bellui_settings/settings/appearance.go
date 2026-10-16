package settings

import (
	"image/color"
	"os"
	"path/filepath"
	"runtime"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/app"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	internalapp "github.com/belluikit/bellui/internal/app"
	internaltheme "github.com/belluikit/bellui/internal/theme"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

const (
	themeNameDark        = "dark"
	themeNameLight       = "light"
	themeNameSystem      = ""
	themeNameSystemLabel = "system default"
)

// Settings gives access to user interfaces to control Bellui settings
type Settings struct {
	belluiSettings app.SettingsSchema

	preview *bellui.Container
	colors  []core.CanvasObject

	userTheme bellui.Theme
}

// NewSettings returns a new settings instance with the current configuration loaded
func NewSettings() *Settings {
	s := &Settings{}
	s.load()
	if s.belluiSettings.Scale == 0 {
		s.belluiSettings.Scale = 1
	}
	return s
}

// AppearanceIcon returns the icon for appearance settings
func (s *Settings) AppearanceIcon() bellui.Resource {
	return theme.NewThemedResource(resourceAppearanceSvg)
}

// LoadAppearanceScreen creates a new settings screen to handle appearance configuration
func (s *Settings) LoadAppearanceScreen(w bellui.Window) core.CanvasObject {
	fallback := bellui.CurrentApp().Settings().Theme()
	if fallback == nil {
		fallback = theme.DefaultTheme()
	}
	s.userTheme = &previewTheme{s: s, t: fallback}
	s.preview = s.createPreview()

	def := s.belluiSettings.ThemeName
	themeNames := []string{themeNameDark, themeNameLight}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		themeNames = append(themeNames, themeNameSystemLabel)
		if s.belluiSettings.ThemeName == themeNameSystem {
			def = themeNameSystemLabel
		}
	}
	themes := widget.NewSelect(themeNames, s.chooseTheme)
	themes.SetSelected(def)

	scale := s.makeScaleGroup(w.Canvas().Scale())
	box := container.NewVBox(scale)

	animations := widget.NewCheck("Animate widgets", func(on bool) {
		s.belluiSettings.DisableAnimations = !on
	})
	animations.Checked = !s.belluiSettings.DisableAnimations
	for _, c := range theme.PrimaryColorNames() {
		b := newPrimaryColorButton(c, s)
		s.colors = append(s.colors, b)
	}
	swatch := container.NewGridWithColumns(len(s.colors), s.colors...)
	appearance := widget.NewForm(
		widget.NewFormItem("Animations", animations),
		widget.NewFormItem("Main Color", swatch),
		widget.NewFormItem("Theme", themes),
	)

	box.Add(widget.NewCard("Appearance", "", appearance))
	bottom := container.NewHBox(layout.NewSpacer(),
		&widget.Button{Text: "Apply", Importance: widget.HighImportance, OnTapped: func() {
			if s.belluiSettings.Scale == 0.0 {
				s.chooseScale(1.0)
			}
			err := s.save()
			if err != nil {
				bellui.LogError("Failed on saving", err)
			}

			s.appliedScale(s.belluiSettings.Scale)
		}})

	return container.NewBorder(box, bottom, nil, nil,
		container.NewCenter(s.preview))
}

func (s *Settings) chooseTheme(name string) {
	if name == themeNameSystemLabel {
		name = themeNameSystem
	}
	s.belluiSettings.ThemeName = name

	s.refreshPreview()
}

func (s *Settings) createPreview() *bellui.Container {
	preview := createPreviewWidget()

	content := container.NewThemeOverride(preview, s.userTheme)
	bg := canvas.NewRectangle(s.userTheme.Color(theme.ColorNameBackground, theme.VariantDark))
	content.Refresh()

	return container.NewStack(bg, content)
}

func (s *Settings) load() {
	err := s.loadFromFile(s.belluiSettings.StoragePath())
	if err != nil {
		bellui.LogError("Settings load error:", err)
	}
}

func (s *Settings) loadFromFile(path string) error {
	data, err := os.ReadFile(path) // #nosec
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(filepath.Dir(path), 0o700)
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return s.belluiSettings.UnmarshalPXF(data)
}

func (s *Settings) refreshPreview() {
	s.preview.Objects[0].(*canvas.Rectangle).FillColor = s.userTheme.Color(theme.ColorNameBackground, theme.VariantDark)
	s.preview.Refresh()
}

func (s *Settings) save() error {
	return s.saveToFile(s.belluiSettings.StoragePath())
}

func (s *Settings) saveToFile(path string) error {
	err := os.MkdirAll(filepath.Dir(path), 0o700)
	if err != nil { // this is not an exists error according to docs
		return err
	}

	data, err := s.belluiSettings.MarshalPXF()
	if err != nil {
		return err
	}

	// Write atomically (temp + rename) so a crash mid-write never truncates the
	// settings store, matching the preferences PXF backend.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	// Drop the superseded legacy settings.json once the PXF store is written
	// (pre-rebrand tracker #872 dropped the JSON path outright; this clears any stale file it left).
	os.Remove(filepath.Join(filepath.Dir(path), "settings.json"))
	return nil
}

type primaryColorButton struct {
	widget.BaseWidget
	name string

	s *Settings
}

func newPrimaryColorButton(name string, s *Settings) *primaryColorButton {
	b := &primaryColorButton{name: name, s: s}
	b.ExtendBaseWidget(b)
	return b
}

func (c *primaryColorButton) CreateRenderer() core.WidgetRenderer {
	r := canvas.NewRectangle(internaltheme.PrimaryColorNamed(c.name))
	r.CornerRadius = theme.SelectionRadiusSize()
	r.StrokeWidth = 5

	if c.name == c.s.belluiSettings.PrimaryColor {
		r.StrokeColor = theme.Color(theme.ColorNamePrimary)
	}

	return &primaryColorButtonRenderer{c: c, rect: r, objs: []core.CanvasObject{r}}
}

func (c *primaryColorButton) Tapped(_ *event.PointEvent) {
	c.s.belluiSettings.PrimaryColor = c.name
	for _, child := range c.s.colors {
		child.Refresh()
	}

	c.s.refreshPreview()
}

type primaryColorButtonRenderer struct {
	c    *primaryColorButton
	rect *canvas.Rectangle
	objs []core.CanvasObject
}

func (c *primaryColorButtonRenderer) Layout(s geo.Size) {
	c.rect.Resize(s)
}

func (c *primaryColorButtonRenderer) MinSize() geo.Size {
	return geo.NewSize(20, 32)
}

func (c *primaryColorButtonRenderer) Refresh() {
	if c.c.name == c.c.s.belluiSettings.PrimaryColor {
		c.rect.StrokeColor = theme.Color(theme.ColorNamePrimary)
	} else {
		c.rect.StrokeColor = color.Transparent
	}
	c.rect.FillColor = internaltheme.PrimaryColorNamed(c.c.name)
	c.rect.CornerRadius = theme.SelectionRadiusSize()

	c.rect.Refresh()
}

func (c *primaryColorButtonRenderer) Objects() []core.CanvasObject {
	return c.objs
}

func (c *primaryColorButtonRenderer) Destroy() {
}

type previewTheme struct {
	s *Settings
	t bellui.Theme
}

func (p *previewTheme) Color(n bellui.ThemeColorName, _ bellui.ThemeVariant) color.Color {
	variant := theme.VariantDark
	switch p.s.belluiSettings.ThemeName {
	case themeNameLight:
		variant = theme.VariantLight
	case themeNameSystem:
		variant = internalapp.DefaultVariant()
	}

	switch n {
	case theme.ColorNamePrimary:
		return internaltheme.PrimaryColorNamed(p.s.belluiSettings.PrimaryColor)
	case theme.ColorNameForegroundOnPrimary:
		return internaltheme.ForegroundOnPrimaryColorNamed(p.s.belluiSettings.PrimaryColor)
	}

	return p.t.Color(n, variant)
}

func (p *previewTheme) Font(s bellui.TextStyle) bellui.Resource {
	return p.t.Font(s)
}

func (p *previewTheme) Icon(n bellui.ThemeIconName) bellui.Resource {
	return p.t.Icon(n)
}

func (p *previewTheme) Size(n bellui.ThemeSizeName) float32 {
	return p.t.Size(n)
}

func createPreviewWidget() core.CanvasObject {
	username := widget.NewEntry()
	password := widget.NewPasswordEntry()
	form := widget.NewForm(widget.NewFormItem("Username", username),
		widget.NewFormItem("Password", password))
	form.OnCancel = func() {}
	form.OnSubmit = func() {}
	content := container.NewVBox(
		widget.NewLabelWithStyle("Theme Preview", bellui.TextAlignCenter, bellui.TextStyle{Bold: true}), form,
	)

	over := container.NewStack(intWidget.NewShadow(intWidget.ShadowAround, intWidget.DialogLevel),
		container.NewPadded(content))

	return over
}
