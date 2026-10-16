package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget      = (*SearchField)(nil)
	_ core.Disableable = (*SearchField)(nil)
	_ core.Accessible  = (*SearchField)(nil)
)

// SearchField is an Entry with a leading search icon and a trailing clear button.
// OnChanged fires on every keystroke for live filtering.
// OnSearched fires when the user presses Enter/Return.
//
// Since: 1.0
type SearchField struct {
	Entry

	// OnSearched is called when the user presses Enter/Return.
	OnSearched func(string) `json:"-"`

	// OnChanged is called on every text change for live filtering.
	OnChanged func(string) `json:"-"`

	clearBtn *Button
}

// NewSearchField creates a search input with the given callback fired on each text change.
//
// Since: 1.0
func NewSearchField(changed func(string)) *SearchField {
	sf := &SearchField{}
	sf.OnChanged = changed
	sf.ExtendBaseWidget(sf)
	sf.PlaceHolder = "Search..."
	return sf
}

// AccessibleRole returns [core.RoleSearchField].
func (sf *SearchField) AccessibleRole() core.AccessibleRole { return core.RoleSearchField }

// AccessibleLabel returns the placeholder text.
func (sf *SearchField) AccessibleLabel() string { return sf.PlaceHolder }

// CreateRenderer returns a new renderer for this search field.
func (sf *SearchField) CreateRenderer() core.WidgetRenderer {
	sf.ExtendBaseWidget(sf)

	sf.Icon = sf.Theme().Icon(theme.IconNameSearch)
	sf.Validator = sf.validate

	if sf.ActionItem == nil {
		sf.clearBtn = sf.setupClearButton()
		sf.ActionItem = sf.clearBtn
	}

	sf.updateClearButtonVisibility()
	return sf.Entry.CreateRenderer()
}

// TypedKey intercepts Enter/Return to fire OnSearched, then delegates to Entry.
func (sf *SearchField) TypedKey(key *event.KeyEvent) {
	if key.Name == event.KeyReturn || key.Name == event.KeyEnter {
		if f := sf.OnSearched; f != nil {
			f(sf.Text)
		}
	}
	sf.Entry.TypedKey(key)
}

// SetText updates the text in the search field.
func (sf *SearchField) SetText(text string) {
	sf.Entry.SetText(text)
	sf.updateClearButtonVisibility()
}

// MinSize returns the minimal size of the search field.
func (sf *SearchField) MinSize() geo.Size {
	sf.ExtendBaseWidget(sf)
	return sf.Entry.MinSize()
}

func (sf *SearchField) setupClearButton() *Button {
	btn := NewButton("", func() {
		sf.Entry.SetText("")
		sf.updateClearButtonVisibility()
	})
	btn.Importance = LowImportance
	btn.SetIcon(sf.Theme().Icon(theme.IconNameCancel))
	return btn
}

func (sf *SearchField) validate(_ string) error {
	sf.updateClearButtonVisibility()
	if f := sf.OnChanged; f != nil {
		f(sf.Text)
	}
	return nil // search fields have no validation errors
}

func (sf *SearchField) updateClearButtonVisibility() {
	if sf.clearBtn == nil {
		return
	}
	if sf.Text == "" {
		sf.clearBtn.Hide()
	} else {
		sf.clearBtn.Show()
	}
}
