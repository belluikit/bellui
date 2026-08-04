package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget      = (*SelectEntry)(nil)
	_ core.Disableable = (*SelectEntry)(nil)
)

// SelectEntry is an input field which supports selecting from a fixed set of options.
type SelectEntry struct {
	Entry

	// OnOptionSelected is called when the user picks an option from the
	// drop-down menu, after the entry's text has been set to it (so it fires
	// after OnChanged). It lets a consumer treat a pick as a complete edit —
	// e.g. an inline table cell editor committing on pick — which a plain
	// OnChanged cannot distinguish from free-text typing. Typing never
	// triggers it, even when the typed text equals an option (#1047).
	//
	// Since: 1.0
	OnOptionSelected func(string)

	dropDown *bellui.Menu
	popUp    *PopUpMenu
	options  []string
}

// NewSelectEntry creates a SelectEntry.
func NewSelectEntry(options []string) *SelectEntry {
	e := &SelectEntry{options: options}
	e.ExtendBaseWidget(e)
	e.Wrapping = bellui.TextWrap(bellui.TextTruncateClip)
	return e
}

// CreateRenderer returns a new renderer for this select entry.
func (e *SelectEntry) CreateRenderer() core.WidgetRenderer {
	e.ExtendBaseWidget(e)
	e.SetOptions(e.options)
	return e.Entry.CreateRenderer()
}

// Enable this widget, updating any style or features appropriately.
func (e *SelectEntry) Enable() {
	if e.ActionItem != nil {
		e.ActionItem.(core.Disableable).Enable()
	}
	e.Entry.Enable()
}

// Disable this widget so that it cannot be interacted with, updating any style appropriately.
func (e *SelectEntry) Disable() {
	if e.ActionItem != nil {
		e.ActionItem.(core.Disableable).Disable()
	}
	e.Entry.Disable()
}

// MinSize returns the minimal size of the select entry.
func (e *SelectEntry) MinSize() geo.Size {
	e.ExtendBaseWidget(e)
	return e.Entry.MinSize()
}

// Move changes the relative position of the select entry.
func (e *SelectEntry) Move(pos geo.Position) {
	e.Entry.Move(pos)
	if e.popUp != nil {
		e.popUp.Move(e.popUpPos())
	}
}

// Resize changes the size of the select entry.
func (e *SelectEntry) Resize(size geo.Size) {
	e.Entry.Resize(size)
	if e.popUp != nil {
		e.popUp.Resize(geo.NewSize(size.Width, e.popUp.Size().Height))
	}
}

// SetOptions sets the options the user might select from.
func (e *SelectEntry) SetOptions(options []string) {
	e.options = options
	items := make([]*bellui.MenuItem, len(options))
	for i, option := range options {
		option := option // capture
		items[i] = bellui.NewMenuItem(option, func() {
			e.SetText(option)
			if f := e.OnOptionSelected; f != nil {
				f(option)
			}
		})
	}
	e.dropDown = bellui.NewMenu("", items...)

	if e.ActionItem == nil {
		e.ActionItem = e.setupDropDown()
		if e.Disabled() {
			e.ActionItem.(core.Disableable).Disable()
		}
	}
}

func (e *SelectEntry) popUpPos() geo.Position {
	entryPos := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super())
	return entryPos.Add(geo.NewPos(0, e.Size().Height-e.Theme().Size(theme.SizeNameInputBorder)))
}

func (e *SelectEntry) setupDropDown() *Button {
	dropDownButton := NewButton("", func() {
		c := bellui.CurrentApp().Driver().CanvasForObject(e.super())
		if c == nil {
			return
		}
		e.popUp = NewPopUpMenu(e.dropDown, c)
		e.popUp.ShowAtPosition(e.popUpPos())
		e.popUp.Resize(geo.NewSize(e.Size().Width, e.popUp.MinSize().Height))
	})
	dropDownButton.Importance = LowImportance
	dropDownButton.SetIcon(e.Theme().Icon(theme.IconNameArrowDropDown))
	return dropDownButton
}
