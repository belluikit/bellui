package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func TestSearchField_Creation(t *testing.T) {
	sf := NewSearchField(nil)
	assert.NotNil(t, sf)
	assert.Equal(t, "Search...", sf.PlaceHolder)
}

func TestSearchField_OnChangedFiresOnTyping(t *testing.T) {
	var result string
	sf := NewSearchField(func(text string) {
		result = text
	})
	w := test.NewTempWindow(t, sf)
	_ = w

	sf.TypedRune('a')
	assert.Equal(t, "a", result)

	sf.TypedRune('b')
	assert.Equal(t, "ab", result)
}

func TestSearchField_OnSearchedFiresOnEnter(t *testing.T) {
	var searched string
	sf := NewSearchField(nil)
	sf.OnSearched = func(text string) {
		searched = text
	}
	w := test.NewTempWindow(t, sf)
	_ = w

	sf.SetText("query")
	sf.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, "query", searched)
}

func TestSearchField_ClearButton(t *testing.T) {
	var result string
	sf := NewSearchField(func(text string) {
		result = text
	})
	w := test.NewTempWindow(t, sf)
	_ = w

	// Initially hidden
	assert.True(t, sf.clearBtn.Hidden, "Clear button should be hidden when text is empty")

	// Type something
	sf.SetText("hello")
	assert.False(t, sf.clearBtn.Hidden, "Clear button should be visible when text is non-empty")

	// Clear via button
	test.Tap(sf.clearBtn)
	assert.Equal(t, "", sf.Text)
	assert.Equal(t, "", result, "OnChanged should fire with empty string on clear")
	assert.True(t, sf.clearBtn.Hidden, "Clear button should be hidden after clear")
}

func TestSearchField_ClearButtonVisibilityOnTyping(t *testing.T) {
	sf := NewSearchField(nil)
	w := test.NewTempWindow(t, sf)
	_ = w

	assert.True(t, sf.clearBtn.Hidden)

	sf.TypedRune('a')
	assert.False(t, sf.clearBtn.Hidden)

	sf.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.True(t, sf.clearBtn.Hidden)
}

func TestSearchField_SearchIcon(t *testing.T) {
	sf := NewSearchField(nil)
	w := test.NewTempWindow(t, sf)
	_ = w

	assert.Equal(t, sf.Theme().Icon(theme.IconNameSearch), sf.Icon)
}

func TestSearchField_MinSize(t *testing.T) {
	sf := NewSearchField(nil)
	w := test.NewTempWindow(t, sf)
	_ = w

	min := sf.MinSize()
	assert.Greater(t, min.Width, float32(0))
	assert.Greater(t, min.Height, float32(0))
}

func TestSearchField_OnChangedFiresOnClear(t *testing.T) {
	calls := 0
	sf := NewSearchField(func(text string) {
		calls++
	})
	w := test.NewTempWindow(t, sf)
	_ = w

	sf.SetText("test")
	calls = 0 // reset after SetText triggers OnChanged
	test.Tap(sf.clearBtn)
	assert.GreaterOrEqual(t, calls, 1, "OnChanged should fire on clear")
}

func TestSearchField_SetTextTriggersOnChanged(t *testing.T) {
	var result string
	sf := NewSearchField(func(text string) {
		result = text
	})
	w := test.NewTempWindow(t, sf)
	_ = w

	sf.SetText("hello")
	assert.Equal(t, "hello", result)
}

func TestSearchField_Disable(t *testing.T) {
	sf := NewSearchField(nil)
	w := test.NewTempWindow(t, sf)
	_ = w

	sf.Disable()
	assert.True(t, sf.Disabled())

	sf.Enable()
	assert.False(t, sf.Disabled())
}

func TestSearchField_OnChangedAndOnSearchedSeparate(t *testing.T) {
	var changed, searched string
	sf := NewSearchField(func(text string) {
		changed = text
	})
	sf.OnSearched = func(text string) {
		searched = text
	}
	w := test.NewTempWindow(t, sf)
	_ = w

	sf.TypedRune('a')
	assert.Equal(t, "a", changed, "OnChanged should fire on typing")
	assert.Equal(t, "", searched, "OnSearched should not fire on typing")

	sf.TypedKey(&event.KeyEvent{Name: event.KeyReturn})
	assert.Equal(t, "a", searched, "OnSearched should fire on Enter")
}
