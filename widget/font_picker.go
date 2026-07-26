package widget

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget      = (*FontPicker)(nil)
	_ core.Disableable = (*FontPicker)(nil)
)

const (
	fontPickerPreviewText = "The quick brown fox jumps over the lazy dog"
	fontPickerMaxLines    = 3
)

// FontFamily represents a selectable font family.
type FontFamily struct {
	Name     string          // Display name
	Resource bellui.Resource // Font file bytes (nil = resolve from system at preview time)
}

// FontPickerConfig holds the current font selection state.
type FontPickerConfig struct {
	Family    string
	Size      float64
	Weight    bellui.TextWeight
	Bold      bool
	Italic    bool
	Underline bool
}

// TextStyle converts the config to a bellui.TextStyle.
func (c FontPickerConfig) TextStyle() bellui.TextStyle {
	s := bellui.TextStyle{
		Bold:      c.Bold,
		Italic:    c.Italic,
		Underline: c.Underline,
	}
	if c.Weight > 0 {
		s.Weight = bellui.Ptr(c.Weight)
	}
	return s
}

// weightNames maps display names to TextWeight values.
var weightNames = []struct {
	name   string
	weight bellui.TextWeight
}{
	{"Thin", bellui.WeightThin},
	{"ExtraLight", bellui.WeightExtraLight},
	{"Light", bellui.WeightLight},
	{"Normal", bellui.WeightNormal},
	{"Medium", bellui.WeightMedium},
	{"SemiBold", bellui.WeightSemiBold},
	{"Bold", bellui.WeightBold},
	{"ExtraBold", bellui.WeightExtraBold},
	{"Black", bellui.WeightBlack},
}

func weightOptions() []string {
	opts := make([]string, len(weightNames))
	for i, wn := range weightNames {
		opts[i] = wn.name
	}
	return opts
}

func weightFromName(name string) bellui.TextWeight {
	for _, wn := range weightNames {
		if wn.name == name {
			return wn.weight
		}
	}
	return bellui.WeightNormal
}

func nameFromWeight(w bellui.TextWeight) string {
	for _, wn := range weightNames {
		if wn.weight == w {
			return wn.name
		}
	}
	return "Normal"
}

// FontPicker is a composite widget for selecting font family, size, weight, and style.
type FontPicker struct {
	DisableableWidget

	Config    FontPickerConfig
	Families  []FontFamily           // nil = use system fonts
	OnChanged func(FontPickerConfig) `json:"-"`

	familySelect *SearchSelect
	sizeSpinBox  *SpinBox
	weightSelect *Select
	boldBtn      *Button
	italicBtn    *Button
	underlineBtn *Button

	// Preview: up to 3 canvas.Text lines in a fixed-height container.
	previewLines [fontPickerMaxLines]*canvas.Text
	previewBox   *bellui.Container
	content      *bellui.Container

	fontCacheMu sync.RWMutex
	fontCache   map[string]bellui.Resource

	// familyFiles maps family name -> filesystem paths per variant
	familyFiles           map[string]string
	familyBoldFiles       map[string]string
	familyItalicFiles     map[string]string
	familyBoldItalicFiles map[string]string

	// recentFonts tracks recently selected font families for the preferred section.
	recentFonts []string

	cancelled    atomic.Bool
	settingValue bool
}

// NewFontPicker creates a font picker with an explicit font family list.
func NewFontPicker(config FontPickerConfig, families []FontFamily, changed func(FontPickerConfig)) *FontPicker {
	fp := &FontPicker{
		Config:    config,
		Families:  families,
		OnChanged: changed,
		fontCache: make(map[string]bellui.Resource),
	}
	fp.ExtendBaseWidget(fp)
	fp.setupUI()
	return fp
}

// NewFontPickerWithSystemFonts creates a font picker that enumerates system fonts.
// The font list loads asynchronously.
func NewFontPickerWithSystemFonts(config FontPickerConfig, changed func(FontPickerConfig)) *FontPicker {
	fp := &FontPicker{
		Config:                config,
		OnChanged:             changed,
		fontCache:             make(map[string]bellui.Resource),
		familyFiles:           make(map[string]string),
		familyBoldFiles:       make(map[string]string),
		familyItalicFiles:     make(map[string]string),
		familyBoldItalicFiles: make(map[string]string),
	}
	fp.ExtendBaseWidget(fp)
	fp.setupUI()
	fp.loadSystemFontsAsync()
	return fp
}

// SetConfig updates the picker's configuration and refreshes all controls.
func (fp *FontPicker) SetConfig(config FontPickerConfig) {
	fp.Config = config
	fp.syncControls()
}

// SetFamilies updates the available font families.
func (fp *FontPicker) SetFamilies(families []FontFamily) {
	fp.Families = families
	opts := make([]string, len(families))
	for i, f := range families {
		opts[i] = f.Name
	}
	fp.familySelect.SetOptions(opts)
	fp.familySelect.SetPreferred(fp.recentFonts)
	if fp.Config.Family != "" {
		fp.familySelect.SetSelected(fp.Config.Family)
	}
}

// CreateRenderer returns the renderer for the font picker.
func (fp *FontPicker) CreateRenderer() core.WidgetRenderer {
	fp.ExtendBaseWidget(fp)
	if fp.content == nil {
		fp.setupUI()
	}
	return &fontPickerRenderer{content: fp.content, picker: fp}
}

type fontPickerRenderer struct {
	content core.CanvasObject
	picker  *FontPicker
}

func (r *fontPickerRenderer) Destroy()                     { r.picker.cancelled.Store(true) }
func (r *fontPickerRenderer) Layout(s geo.Size)            { r.content.Resize(s) }
func (r *fontPickerRenderer) MinSize() geo.Size            { return r.content.MinSize() }
func (r *fontPickerRenderer) Objects() []core.CanvasObject { return []core.CanvasObject{r.content} }
func (r *fontPickerRenderer) SetObjects(o []core.CanvasObject) {
	if len(o) > 0 {
		r.content = o[0]
	}
}
func (r *fontPickerRenderer) Refresh() { r.content.Refresh() }

// MinSize returns the minimum size of the font picker.
func (fp *FontPicker) MinSize() geo.Size {
	fp.ExtendBaseWidget(fp)
	return fp.BaseWidget.MinSize()
}

// Enable enables the font picker and all sub-widgets.
func (fp *FontPicker) Enable() {
	fp.DisableableWidget.Enable()
	if fp.familySelect != nil {
		fp.familySelect.Enable()
		fp.sizeSpinBox.Enable()
		fp.weightSelect.Enable()
		fp.boldBtn.Enable()
		fp.italicBtn.Enable()
		fp.underlineBtn.Enable()
	}
}

// Disable disables the font picker and all sub-widgets.
func (fp *FontPicker) Disable() {
	fp.DisableableWidget.Disable()
	if fp.familySelect != nil {
		fp.familySelect.Disable()
		fp.sizeSpinBox.Disable()
		fp.weightSelect.Disable()
		fp.boldBtn.Disable()
		fp.italicBtn.Disable()
		fp.underlineBtn.Disable()
	}
}

func (fp *FontPicker) setupUI() {
	// Create all widgets first before wiring callbacks that reference them.
	familyOpts := make([]string, len(fp.Families))
	for i, f := range fp.Families {
		familyOpts[i] = f.Name
	}
	fp.familySelect = NewSearchSelect(familyOpts, nil)
	fp.familySelect.PlaceHolder = "Font Family"

	fp.sizeSpinBox = NewSpinBox(1, 200, 1)

	fp.weightSelect = NewSelect(weightOptions(), nil)

	fp.boldBtn = NewButton("B", fp.toggleBold)
	fp.boldBtn.Style = fp.toggleStyle(fp.Config.Bold)

	fp.italicBtn = NewButton("I", fp.toggleItalic)
	fp.italicBtn.Style = fp.toggleStyle(fp.Config.Italic)

	fp.underlineBtn = NewButton("U", fp.toggleUnderline)
	fp.underlineBtn.Style = fp.toggleStyle(fp.Config.Underline)

	// Preview: 3 canvas.Text lines.
	fg := theme.Color(theme.ColorNameForeground)
	for i := range fp.previewLines {
		fp.previewLines[i] = canvas.NewText("", fg)
	}

	// Set initial values (settingValue guard prevents callback firing).
	fp.settingValue = true
	if fp.Config.Family != "" {
		fp.familySelect.SetSelected(fp.Config.Family)
	}
	if fp.Config.Size > 0 {
		fp.sizeSpinBox.SetValue(fp.Config.Size)
	} else {
		fp.Config.Size = 14
		fp.sizeSpinBox.SetValue(14)
	}
	fp.weightSelect.SetSelected(nameFromWeight(fp.Config.Weight))
	fp.settingValue = false

	// Wire callbacks after all widgets exist.
	fp.familySelect.OnChanged = func(selected string) {
		if fp.settingValue {
			return
		}
		fp.Config.Family = selected
		fp.addRecentFont(selected)
		fp.syncStyleButtons()
		fp.syncPreview()
		fp.fireChanged()
	}
	fp.sizeSpinBox.OnChanged = func(val float64) {
		if fp.settingValue {
			return
		}
		fp.Config.Size = val
		fp.syncPreview()
		fp.fireChanged()
	}
	fp.weightSelect.OnChanged = func(selected string) {
		if fp.settingValue {
			return
		}
		w := weightFromName(selected)
		fp.Config.Weight = w
		fp.settingValue = true
		fp.Config.Bold = w >= bellui.WeightSemiBold
		fp.updateToggleStyle(fp.boldBtn, fp.Config.Bold)
		fp.settingValue = false
		fp.syncPreview()
		fp.fireChanged()
	}

	// Layout: two rows + fixed-height preview
	row1 := &bellui.Container{
		Layout:  layout.NewBorderLayout(nil, nil, nil, fp.sizeSpinBox),
		Objects: []core.CanvasObject{fp.sizeSpinBox, fp.familySelect},
	}

	sep := NewSeparator()
	row2 := &bellui.Container{
		Layout:  layout.NewHBoxLayout(),
		Objects: []core.CanvasObject{fp.weightSelect, sep, fp.boldBtn, fp.italicBtn, fp.underlineBtn},
	}

	// Fixed-height preview area with clipping.
	previewLines := &bellui.Container{
		Layout: layout.NewVBoxLayout(),
		Objects: []core.CanvasObject{
			fp.previewLines[0], fp.previewLines[1], fp.previewLines[2],
		},
	}
	fp.previewBox = &bellui.Container{
		Layout:       &fixedHeightLayout{height: fp.previewHeight()},
		Objects:      []core.CanvasObject{previewLines},
		ClipToBounds: true,
	}
	previewCard := &Card{Content: fp.previewBox, Style: CardStyleOutlined}

	fp.content = &bellui.Container{
		Layout:  layout.NewVBoxLayout(),
		Objects: []core.CanvasObject{row1, row2, previewCard},
	}

	// Initial preview render.
	fp.syncPreview()
}

// previewHeight returns a fixed height for the preview area (3 lines at default text size).
func (fp *FontPicker) previewHeight() float32 {
	lineHeight := theme.Size(theme.SizeNameText) * 1.6
	return lineHeight*fontPickerMaxLines + theme.Padding()*2
}

func (fp *FontPicker) toggleBold() {
	fp.Config.Bold = !fp.Config.Bold
	fp.updateToggleStyle(fp.boldBtn, fp.Config.Bold)

	fp.settingValue = true
	if fp.Config.Bold {
		if fp.Config.Weight < bellui.WeightSemiBold {
			fp.Config.Weight = bellui.WeightBold
		}
	} else {
		if fp.Config.Weight >= bellui.WeightSemiBold {
			fp.Config.Weight = bellui.WeightNormal
		}
	}
	fp.weightSelect.SetSelected(nameFromWeight(fp.Config.Weight))
	fp.settingValue = false

	fp.syncPreview()
	fp.fireChanged()
}

func (fp *FontPicker) toggleItalic() {
	fp.Config.Italic = !fp.Config.Italic
	fp.updateToggleStyle(fp.italicBtn, fp.Config.Italic)
	fp.syncPreview()
	fp.fireChanged()
}

func (fp *FontPicker) toggleUnderline() {
	fp.Config.Underline = !fp.Config.Underline
	fp.updateToggleStyle(fp.underlineBtn, fp.Config.Underline)
	fp.syncPreview()
	fp.fireChanged()
}

// hasBold returns true if the selected font has a bold variant.
func (fp *FontPicker) hasBold() bool {
	return fp.familyBoldFiles[fp.Config.Family] != "" || fp.familyBoldItalicFiles[fp.Config.Family] != ""
}

// hasItalic returns true if the selected font has an italic variant.
func (fp *FontPicker) hasItalic() bool {
	return fp.familyItalicFiles[fp.Config.Family] != "" || fp.familyBoldItalicFiles[fp.Config.Family] != ""
}

// syncStyleButtons enables/disables B/I buttons based on available font variants.
// If a style is active but the variant is unavailable, the style is turned off.
// When no system font files are loaded (user-provided families or no selection),
// buttons are always enabled.
func (fp *FontPicker) syncStyleButtons() {
	family := fp.Config.Family
	if family == "" || len(fp.familyFiles) == 0 {
		fp.boldBtn.Enable()
		fp.italicBtn.Enable()
		return
	}

	// If the family is user-provided (not from system scan), keep buttons enabled.
	if _, fromSystem := fp.familyFiles[family]; !fromSystem {
		fp.boldBtn.Enable()
		fp.italicBtn.Enable()
		return
	}

	bold := fp.hasBold()
	italic := fp.hasItalic()

	if bold {
		fp.boldBtn.Enable()
	} else {
		if fp.Config.Bold {
			fp.Config.Bold = false
			fp.updateToggleStyle(fp.boldBtn, false)
		}
		fp.boldBtn.Disable()
	}

	if italic {
		fp.italicBtn.Enable()
	} else {
		if fp.Config.Italic {
			fp.Config.Italic = false
			fp.updateToggleStyle(fp.italicBtn, false)
		}
		fp.italicBtn.Disable()
	}
}

func (fp *FontPicker) toggleStyle(active bool) ButtonStyle {
	if active {
		return ButtonStyleFilledTonal
	}
	return ButtonStyleOutlined
}

func (fp *FontPicker) updateToggleStyle(btn *Button, active bool) {
	btn.Style = fp.toggleStyle(active)
	btn.Refresh()
}

// syncPreview updates the preview text lines with current font, size, and style.
func (fp *FontPicker) syncPreview() {
	if fp.previewLines[0] == nil {
		return
	}

	textSize := float32(fp.Config.Size)
	style := fp.Config.TextStyle()
	fg := theme.Color(theme.ColorNameForeground)

	// Word-wrap the preview text into up to 3 lines.
	availWidth := fp.availablePreviewWidth()
	lines := fp.wordWrap(fontPickerPreviewText, availWidth, textSize, style)

	for i := 0; i < fontPickerMaxLines; i++ {
		t := fp.previewLines[i]
		if i < len(lines) {
			text := lines[i]
			if i == fontPickerMaxLines-1 && len(lines) > fontPickerMaxLines {
				text += "..."
			}
			t.Text = text
		} else {
			t.Text = ""
		}
		t.TextSize = textSize
		t.TextStyle = style
		t.Color = fg
		t.FontSource = fp.currentFontResource()
		t.Refresh()
	}
}

// availablePreviewWidth estimates the width available for preview text.
func (fp *FontPicker) availablePreviewWidth() float32 {
	if fp.previewBox != nil {
		s := fp.previewBox.Size()
		if s.Width > 0 {
			return s.Width - theme.Padding()*2
		}
	}
	// Fallback: use a reasonable default.
	return 300
}

// wordWrap splits text into lines that fit within maxWidth at the given size and style.
func (fp *FontPicker) wordWrap(text string, maxWidth, textSize float32, style bellui.TextStyle) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		candidate := currentLine + " " + word
		w := fp.measureText(candidate, textSize, style)
		if w <= maxWidth {
			currentLine = candidate
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)
	return lines
}

// currentFontResource returns the font resource for the current family and style.
// It picks the bold or italic variant file when available.
func (fp *FontPicker) currentFontResource() bellui.Resource {
	family := fp.Config.Family
	if family == "" {
		return nil
	}

	// Check user-provided families (no variant support).
	for _, f := range fp.Families {
		if f.Name == family && f.Resource != nil {
			return f.Resource
		}
	}

	// Pick the right variant file.
	filePath := fp.familyFiles[family]
	bold := fp.Config.Bold
	italic := fp.Config.Italic
	switch {
	case bold && italic:
		if f := fp.familyBoldItalicFiles[family]; f != "" {
			filePath = f
		}
	case bold:
		if f := fp.familyBoldFiles[family]; f != "" {
			filePath = f
		}
	case italic:
		if f := fp.familyItalicFiles[family]; f != "" {
			filePath = f
		}
	}
	if filePath == "" {
		return nil
	}

	return fp.loadFontFile(filePath)
}

// measureText returns the width of text at the given size and style.
func (fp *FontPicker) measureText(text string, textSize float32, style bellui.TextStyle) float32 {
	s, _ := bellui.CurrentApp().Driver().RenderedTextSize(text, textSize, style, fp.currentFontResource())
	return s.Width
}

// loadFontFile loads a font file from disk, caching the result.
func (fp *FontPicker) loadFontFile(filePath string) bellui.Resource {
	fp.fontCacheMu.RLock()
	res, ok := fp.fontCache[filePath]
	fp.fontCacheMu.RUnlock()
	if ok {
		return res
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	res = bellui.NewStaticResource(filePath, data)
	fp.fontCacheMu.Lock()
	fp.fontCache[filePath] = res
	fp.fontCacheMu.Unlock()
	return res
}

func (fp *FontPicker) syncControls() {
	fp.settingValue = true
	defer func() { fp.settingValue = false }()

	fp.familySelect.SetSelected(fp.Config.Family)
	fp.sizeSpinBox.SetValue(fp.Config.Size)
	fp.weightSelect.SetSelected(nameFromWeight(fp.Config.Weight))
	fp.updateToggleStyle(fp.boldBtn, fp.Config.Bold)
	fp.updateToggleStyle(fp.italicBtn, fp.Config.Italic)
	fp.updateToggleStyle(fp.underlineBtn, fp.Config.Underline)
	fp.syncStyleButtons()
	fp.syncPreview()
}

const maxRecentFonts = 5

// addRecentFont adds a font to the recent list and updates the preferred section.
func (fp *FontPicker) addRecentFont(family string) {
	if family == "" {
		return
	}
	// Remove if already present.
	for i, f := range fp.recentFonts {
		if f == family {
			fp.recentFonts = append(fp.recentFonts[:i], fp.recentFonts[i+1:]...)
			break
		}
	}
	// Prepend.
	fp.recentFonts = append([]string{family}, fp.recentFonts...)
	if len(fp.recentFonts) > maxRecentFonts {
		fp.recentFonts = fp.recentFonts[:maxRecentFonts]
	}
	fp.familySelect.SetPreferred(fp.recentFonts)
}

func (fp *FontPicker) fireChanged() {
	if f := fp.OnChanged; f != nil {
		f(fp.Config)
	}
}

func (fp *FontPicker) loadSystemFontsAsync() {
	fp.familySelect.PlaceHolder = "Loading fonts..."
	fp.familySelect.Refresh()

	go func() {
		families := painter.SystemFontFamilies()

		if fp.cancelled.Load() {
			return
		}

		names := make([]string, len(families))
		fileMap := make(map[string]string, len(families))
		boldMap := make(map[string]string, len(families))
		italicMap := make(map[string]string, len(families))
		boldItalicMap := make(map[string]string, len(families))
		for i, fi := range families {
			names[i] = fi.Name
			fileMap[fi.Name] = fi.File
			if fi.BoldFile != "" {
				boldMap[fi.Name] = fi.BoldFile
			}
			if fi.ItalicFile != "" {
				italicMap[fi.Name] = fi.ItalicFile
			}
			if fi.BoldItalicFile != "" {
				boldItalicMap[fi.Name] = fi.BoldItalicFile
			}
		}

		bellui.Do(func() {
			if fp.cancelled.Load() {
				return
			}
			fp.familyFiles = fileMap
			fp.familyBoldFiles = boldMap
			fp.familyItalicFiles = italicMap
			fp.familyBoldItalicFiles = boldItalicMap
			fp.familySelect.SetOptions(names)
			fp.familySelect.SetPreferred(fp.recentFonts)
			fp.familySelect.PlaceHolder = "Font Family"
			if fp.Config.Family != "" {
				fp.familySelect.SetSelected(fp.Config.Family)
			}
			fp.familySelect.Refresh()
			fp.syncPreview()
		})
	}()
}

// ShowFontPickerPopup shows the font picker in a modal popup with OK/Cancel buttons.
func ShowFontPickerPopup(config FontPickerConfig, families []FontFamily, changed func(FontPickerConfig), c core.Canvas) {
	var picker *FontPicker
	if families != nil {
		picker = NewFontPicker(config, families, nil)
	} else {
		picker = NewFontPickerWithSystemFonts(config, nil)
	}

	var popUp *PopUp
	okBtn := NewButton("OK", func() {
		if changed != nil {
			changed(picker.Config)
		}
		if popUp != nil {
			popUp.Hide()
		}
	})
	okBtn.Style = ButtonStyleFilled

	cancelBtn := NewButton("Cancel", func() {
		if popUp != nil {
			popUp.Hide()
		}
	})
	cancelBtn.Style = ButtonStyleText

	buttons := &bellui.Container{
		Layout:  layout.NewHBoxLayout(),
		Objects: []core.CanvasObject{layout.NewSpacer(), cancelBtn, okBtn},
	}

	content := &bellui.Container{
		Layout:  layout.NewVBoxLayout(),
		Objects: []core.CanvasObject{picker, buttons},
	}

	th := theme.Current()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bgColor := th.Color(theme.ColorNameSurfaceContainerHigh, v)

	popUp = NewModalPopUp(content, c)
	popUp.bgColor = bgColor
	popUp.Show()
}

// fixedHeightLayout constrains its children to a fixed height while using full width.
type fixedHeightLayout struct {
	height float32
}

func (l *fixedHeightLayout) Layout(objects []core.CanvasObject, size geo.Size) {
	for _, o := range objects {
		o.Resize(geo.NewSize(size.Width, l.height))
		o.Move(geo.NewPos(0, 0))
	}
}

func (l *fixedHeightLayout) MinSize(_ []core.CanvasObject) geo.Size {
	return geo.NewSize(0, l.height)
}

var _ core.Layout = (*fixedHeightLayout)(nil)
