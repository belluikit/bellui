package widget

import (
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget      = (*SearchSelect)(nil)
	_ core.Disableable = (*SearchSelect)(nil)
)

// SearchSelect is a dropdown widget with text search filtering and support for
// preferred options shown at the top with a divider.
type SearchSelect struct {
	DisableableWidget

	Options     []string     // All available options
	Preferred   []string     // Options shown first, above a divider
	Selected    string       // Currently selected value
	PlaceHolder string       // Text shown when nothing is selected
	OnChanged   func(string) `json:"-"`

	entry  *Entry
	list   *List
	popUp  *PopUp
	filter string
}

// NewSearchSelect creates a new SearchSelect with the given options and change handler.
func NewSearchSelect(options []string, changed func(string)) *SearchSelect {
	s := &SearchSelect{
		Options:     options,
		OnChanged:   changed,
		PlaceHolder: "(Select one)",
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetSelected sets the current option and updates the displayed label.
func (s *SearchSelect) SetSelected(text string) {
	s.updateSelected(text)
}

// SetOptions updates the list of available options.
func (s *SearchSelect) SetOptions(options []string) {
	s.Options = options
	s.Refresh()
}

// SetPreferred updates the preferred options shown at the top.
func (s *SearchSelect) SetPreferred(preferred []string) {
	s.Preferred = preferred
}

// ClearSelected resets the selection to empty.
func (s *SearchSelect) ClearSelected() {
	s.Selected = ""
	s.Refresh()
}

// Tapped opens the search popup.
func (s *SearchSelect) Tapped(*event.PointEvent) {
	if s.Disabled() {
		return
	}
	s.showPopUp()
}

// CreateRenderer returns the renderer for SearchSelect.
// It reuses Select's visual appearance: background, label, dropdown icon.
func (s *SearchSelect) CreateRenderer() core.WidgetRenderer {
	s.ExtendBaseWidget(s)

	label := NewRichTextWithText("")
	label.inset = geo.NewSquareSize(0)
	label.Truncation = bellui.TextTruncateEllipsis
	icon := NewIcon(s.Theme().Icon(theme.IconNameArrowDropDown))
	bg := newSelectBackground(s)

	objects := []core.CanvasObject{bg, label, icon}
	r := &searchSelectRenderer{
		label:   label,
		icon:    icon,
		bg:      bg,
		objects: objects,
		combo:   s,
	}
	r.updateLabel()
	return r
}

// MinSize returns the minimum size.
func (s *SearchSelect) MinSize() geo.Size {
	s.ExtendBaseWidget(s)
	return s.BaseWidget.MinSize()
}

func (s *SearchSelect) updateSelected(text string) {
	s.Selected = text
	if s.OnChanged != nil {
		s.OnChanged(text)
	}
	s.Refresh()
}

type displayItem struct {
	text    string
	divider bool
}

// displayItems returns the display list: preferred (filtered) + divider label + all options (filtered).
func (s *SearchSelect) displayItems() []displayItem {
	filter := strings.ToLower(s.filter)

	var preferred []displayItem
	prefSet := make(map[string]bool, len(s.Preferred))
	for _, p := range s.Preferred {
		prefSet[p] = true
		if filter == "" || strings.Contains(strings.ToLower(p), filter) {
			preferred = append(preferred, displayItem{text: p})
		}
	}

	var others []displayItem
	for _, opt := range s.Options {
		if prefSet[opt] {
			continue
		}
		if filter == "" || strings.Contains(strings.ToLower(opt), filter) {
			others = append(others, displayItem{text: opt})
		}
	}

	if len(preferred) == 0 {
		return others
	}
	if len(others) == 0 {
		return preferred
	}
	result := make([]displayItem, 0, len(preferred)+1+len(others))
	result = append(result, preferred...)
	result = append(result, displayItem{text: "────────", divider: true})
	result = append(result, others...)
	return result
}

// filteredItems returns just the text values for external use (tests, item count).
func (s *SearchSelect) filteredItems() []string {
	items := s.displayItems()
	result := make([]string, 0, len(items))
	for _, di := range items {
		if di.divider {
			result = append(result, "")
		} else {
			result = append(result, di.text)
		}
	}
	return result
}

func (s *SearchSelect) showPopUp() {
	c := bellui.CurrentApp().Driver().CanvasForObject(s.super())
	if c == nil {
		return
	}

	s.filter = ""
	items := s.filteredItems()

	// Search entry at top.
	s.entry = NewEntry()
	s.entry.PlaceHolder = "Search..."
	s.entry.OnChanged = func(text string) {
		s.filter = text
		s.list.Refresh()
	}

	// Build display items.
	popWidth := geo.Max(s.Size().Width, float32(250))

	// Find the longest option name to size the template appropriately.
	longestOpt := ""
	for _, opt := range s.Options {
		if len(opt) > len(longestOpt) {
			longestOpt = opt
		}
	}
	// Cap at a reasonable length to avoid overly wide popups.
	if len(longestOpt) > 40 {
		longestOpt = longestOpt[:40]
	}

	s.list = NewList(
		func() int {
			return len(s.displayItems())
		},
		func() core.CanvasObject {
			l := NewLabel(longestOpt)
			l.Truncation = bellui.TextTruncateEllipsis
			return l
		},
		func(id ListItemID, obj core.CanvasObject) {
			items := s.displayItems()
			if id >= len(items) {
				return
			}
			obj.(*Label).SetText(items[id].text)
		},
	)
	s.list.HideSeparators = true
	s.list.OnSelected = func(id ListItemID) {
		items := s.displayItems()
		if id >= len(items) {
			return
		}
		di := items[id]
		if di.divider {
			s.list.UnselectAll()
			return
		}
		s.updateSelected(di.text)
		if s.popUp != nil {
			s.popUp.Hide()
			s.popUp = nil
		}
	}

	_ = items // initial build

	// Ensure popup is wide enough for the longest item.
	listMinW := s.list.MinSize().Width + theme.Padding()*4
	popWidth = geo.Max(popWidth, listMinW)

	// Popup content: search entry + list.
	listHeight := geo.Min(float32(300), float32(len(s.displayItems()))*s.Theme().Size(theme.SizeNameText)*2.5)
	if listHeight < 100 {
		listHeight = 100
	}
	listBox := &bellui.Container{
		Layout:  &fixedSizeLayout{width: popWidth, height: listHeight},
		Objects: []core.CanvasObject{s.list},
	}

	content := &bellui.Container{
		Layout:  layout.NewVBoxLayout(),
		Objects: []core.CanvasObject{s.entry, listBox},
	}

	th := s.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bgColor := th.Color(theme.ColorNameSurfaceContainerHigh, v)

	s.popUp = NewModalPopUp(content, c)
	s.popUp.bgColor = bgColor
	s.popUp.Show()

	// Focus the search entry.
	c.Focus(s.entry)
}

// --- renderer ---

type searchSelectRenderer struct {
	label   *RichText
	icon    *Icon
	bg      core.CanvasObject
	objects []core.CanvasObject
	combo   *SearchSelect
}

func (r *searchSelectRenderer) Objects() []core.CanvasObject { return r.objects }

// Destroy has nothing to release: the renderer's objects are torn down by the
// framework's destroy walk, and the dropdown popup is owned by the SearchSelect
// widget and dismissed when a choice is made or the popup loses focus.
func (r *searchSelectRenderer) Destroy() {}

func (r *searchSelectRenderer) Layout(size geo.Size) {
	th := r.combo.Theme()
	pad := th.Size(theme.SizeNamePadding)
	iconSize := th.Size(theme.SizeNameInlineIcon)
	innerPad := th.Size(theme.SizeNameInnerPadding)

	r.bg.Resize(size)
	r.bg.Move(geo.NewPos(0, 0))

	r.label.inset = geo.NewSquareSize(pad)
	iconPos := geo.NewPos(size.Width-iconSize-innerPad, (size.Height-iconSize)/2)
	labelSize := geo.NewSize(iconPos.X-pad, r.label.MinSize().Height)
	r.label.Resize(labelSize)
	r.label.Move(geo.NewPos(pad, (size.Height-labelSize.Height)/2))

	r.icon.Resize(geo.NewSquareSize(iconSize))
	r.icon.Move(iconPos)
}

func (r *searchSelectRenderer) MinSize() geo.Size {
	th := r.combo.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)

	placeholder := r.combo.PlaceHolder
	if placeholder == "" {
		placeholder = "(Select one)"
	}
	minW := bellui.MeasureText(placeholder, th.Size(theme.SizeNameText), bellui.TextStyle{}).Width
	min := r.label.MinSize()
	min.Width = minW
	min = min.Add(geo.NewSize(innerPad*3, innerPad))
	return min.Add(geo.NewSize(th.Size(theme.SizeNameInlineIcon)+innerPad, 0))
}

func (r *searchSelectRenderer) Refresh() {
	r.updateLabel()
	r.updateIcon()
	r.bg.Refresh()
	r.Layout(r.combo.Size())
}

func (r *searchSelectRenderer) updateLabel() {
	seg := r.label.Segments[0].(*TextSegment)
	if r.combo.Disabled() {
		seg.Style.ColorName = theme.ColorNameDisabled
	} else {
		seg.Style.ColorName = theme.ColorNameForeground
	}
	if r.combo.Selected == "" {
		seg.Text = r.combo.PlaceHolder
		if seg.Text == "" {
			seg.Text = "(Select one)"
		}
	} else {
		seg.Text = r.combo.Selected
	}
	r.label.Refresh()
}

func (r *searchSelectRenderer) updateIcon() {
	th := r.combo.Theme()
	icon := th.Icon(theme.IconNameArrowDropDown)
	if r.combo.Disabled() {
		r.icon.Resource = theme.NewDisabledResource(icon)
	} else {
		r.icon.Resource = icon
	}
	r.icon.Refresh()
}

// newSelectBackground creates a themed background rectangle matching Select's appearance.
func newSelectBackground(s *SearchSelect) *canvas.Rectangle {
	th := s.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bg := canvas.NewRectangle(th.Color(theme.ColorNameInputBackground, v))
	bg.CornerRadius = buttonRadius(th)
	return bg
}

// fixedSizeLayout constrains children to a fixed width and height.
type fixedSizeLayout struct {
	width, height float32
}

func (l *fixedSizeLayout) Layout(objects []core.CanvasObject, _ geo.Size) {
	for _, o := range objects {
		o.Resize(geo.NewSize(l.width, l.height))
		o.Move(geo.NewPos(0, 0))
	}
}

func (l *fixedSizeLayout) MinSize(_ []core.CanvasObject) geo.Size {
	return geo.NewSize(l.width, l.height)
}

var _ core.Layout = (*fixedSizeLayout)(nil)
