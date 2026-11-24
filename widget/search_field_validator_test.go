package widget

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	internalwidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// statusShown reports whether the entry's validation status (the ✓ or the
// error icon) is drawn.
func statusShown(e *Entry) bool {
	return e.validationStatus != nil && e.validationStatus.Visible()
}

// A search field with text draws no validation ✓: it validates nothing, and
// it must not claim the app's Validator to hear about changes (#1411).
func TestSearchField_NoValidationStatus(t *testing.T) {
	test.NewTempApp(t)
	sf := NewSearchField(nil)
	w := test.NewTempWindow(t, sf)
	w.Resize(geo.NewSize(320, 44))

	sf.SetText("query")
	test.Type(sf, "x")
	sf.Refresh()
	assert.Nil(t, sf.Validator, "the Validator stays the app's")
	assert.False(t, statusShown(&sf.Entry), "no ✓ beside the ✕")

	// No room is kept for a status icon between the text and the ✕: with a
	// validator the text ended a whole icon before it (24 dp at this size).
	var scroll *internalwidget.Scroll
	for _, o := range test.TempWidgetRenderer(t, sf).Objects() {
		if s, ok := o.(*internalwidget.Scroll); ok {
			scroll = s
		}
	}
	gap := sf.ActionItem.Position().X - (scroll.Position().X + scroll.Size().Width)
	assert.Less(t, gap, sf.Theme().Size(theme.SizeNameInlineIcon), "the text area reaches the ✕")
}

// An app's own Validator on a search field survives the renderer and works,
// and OnChanged still fires (#1411).
func TestSearchField_AppValidator(t *testing.T) {
	test.NewTempApp(t)
	var changed []string
	sf := NewSearchField(func(s string) { changed = append(changed, s) })
	bad := errors.New("no digits")
	sf.Validator = func(s string) error {
		for _, r := range s {
			if r >= '0' && r <= '9' {
				return bad
			}
		}
		return nil
	}
	w := test.NewTempWindow(t, sf)
	w.Resize(geo.NewSize(320, 44))

	sf.SetText("a1")
	assert.NotNil(t, sf.Validator, "CreateRenderer kept the app's Validator")
	assert.ErrorIs(t, sf.validationError, bad, "the app's Validator ran")
	assert.True(t, statusShown(&sf.Entry), "its error shows")
	assert.Equal(t, []string{"a1"}, changed, "OnChanged fires with an app Validator too")
}
