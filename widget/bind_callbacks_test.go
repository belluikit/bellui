package widget

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/test"
)

// Bind must not claim the public callback fields an app owns. Before #49 (and
// #47 for Slider) the data write-back was implemented by assigning OnChanged --
// or, for Entry, Validator -- so binding and an app handler silently destroyed
// each other in both directions, and Unbind cleared a field it never installed.

// --- Check ---

func TestCheck_Bind_KeepsOnChangedAssignedBefore(t *testing.T) {
	test.NewTempApp(t)

	c := NewCheck("go", nil)
	changed := 0
	c.OnChanged = func(bool) { changed++ }

	data := binding.NewBool()
	c.Bind(data)
	waitForBinding()

	c.SetChecked(true)
	assert.Equal(t, 1, changed, "app handler assigned before Bind must survive")

	v, err := data.Get()
	assert.NoError(t, err)
	assert.True(t, v, "the write-back must still reach the bound source")
}

func TestCheck_Bind_KeepsOnChangedAssignedAfter(t *testing.T) {
	test.NewTempApp(t)

	data := binding.NewBool()
	c := NewCheckWithData("go", data) // calls Bind for you
	waitForBinding()

	changed := 0
	c.OnChanged = func(bool) { changed++ }

	c.SetChecked(true)
	assert.Equal(t, 1, changed)

	v, err := data.Get()
	assert.NoError(t, err)
	assert.True(t, v, "assigning OnChanged after Bind must not stop the data tracking")
}

// The write-back runs first so a handler reading the bound source does not see
// a stale value.
func TestCheck_Bind_WriteBackRunsBeforeOnChanged(t *testing.T) {
	test.NewTempApp(t)

	data := binding.NewBool()
	c := NewCheck("go", nil)
	c.Bind(data)
	waitForBinding()

	var seen bool
	c.OnChanged = func(bool) {
		seen, _ = data.Get()
	}

	c.SetChecked(true)
	assert.True(t, seen, "OnChanged must observe the already-written bound value")
}

func TestCheck_Unbind_KeepsOnChanged(t *testing.T) {
	test.NewTempApp(t)

	c := NewCheck("go", nil)
	changed := 0
	c.OnChanged = func(bool) { changed++ }

	data := binding.NewBool()
	c.Bind(data)
	waitForBinding()
	c.Unbind()
	waitForBinding()

	c.SetChecked(true)
	assert.Equal(t, 1, changed, "Unbind must not clear the app's OnChanged")

	v, err := data.Get()
	assert.NoError(t, err)
	assert.False(t, v, "Unbind must stop the write-back")
}

// --- Select ---

func TestSelect_Bind_KeepsOnChangedAssignedBefore(t *testing.T) {
	test.NewTempApp(t)

	s := NewSelect([]string{"a", "b"}, nil)
	changed := 0
	s.OnChanged = func(string) { changed++ }

	data := binding.NewString()
	s.Bind(data)
	waitForBinding()

	s.SetSelected("b")
	assert.Equal(t, 1, changed, "app handler assigned before Bind must survive")

	v, err := data.Get()
	assert.NoError(t, err)
	assert.Equal(t, "b", v, "the write-back must still reach the bound source")
}

func TestSelect_Bind_KeepsOnChangedAssignedAfter(t *testing.T) {
	test.NewTempApp(t)

	data := binding.NewString()
	s := NewSelect([]string{"a", "b"}, nil)
	s.Bind(data)
	waitForBinding()

	changed := 0
	s.OnChanged = func(string) { changed++ }

	s.SetSelected("b")
	assert.Equal(t, 1, changed)

	v, err := data.Get()
	assert.NoError(t, err)
	assert.Equal(t, "b", v, "assigning OnChanged after Bind must not stop the data tracking")
}

// One change must be reported once. Select.SetSelected has no equality guard of
// its own, so the write-back's own binding notification used to re-enter and
// report the change a second time -- invisible while Bind owned OnChanged.
func TestSelect_Bind_ReportsOneChangeOnce(t *testing.T) {
	test.NewTempApp(t)

	data := binding.NewString()
	s := NewSelect([]string{"a", "b"}, nil)
	s.Bind(data)
	waitForBinding()

	var seen []string
	s.OnChanged = func(v string) { seen = append(seen, v) }

	s.SetSelected("b")
	waitForBinding()
	assert.Equal(t, []string{"b"}, seen)

	// An external write to the source is still reported.
	assert.NoError(t, data.Set("a"))
	waitForBinding()
	assert.Equal(t, []string{"b", "a"}, seen)
}

func TestSelect_Unbind_KeepsOnChanged(t *testing.T) {
	test.NewTempApp(t)

	s := NewSelect([]string{"a", "b"}, nil)
	changed := 0
	s.OnChanged = func(string) { changed++ }

	data := binding.NewString()
	s.Bind(data)
	waitForBinding()
	s.Unbind()
	waitForBinding()

	s.SetSelected("b")
	assert.Equal(t, 1, changed, "Unbind must not clear the app's OnChanged")

	v, err := data.Get()
	assert.NoError(t, err)
	assert.Empty(t, v, "Unbind must stop the write-back")
}

// --- Entry ---

func TestEntry_Bind_KeepsValidatorAssignedBefore(t *testing.T) {
	test.NewTempApp(t)

	errNotB := errors.New("must be b")
	e := NewEntry()
	e.Validator = func(s string) error {
		if s != "b" {
			return errNotB
		}
		return nil
	}

	data := binding.NewString()
	e.Bind(data)
	waitForBinding()

	e.SetText("a")
	assert.Equal(t, errNotB, e.Validate(), "app validator assigned before Bind must survive")

	e.SetText("b")
	assert.NoError(t, e.Validate())
}

func TestEntry_Bind_KeepsValidatorAssignedAfter(t *testing.T) {
	test.NewTempApp(t)

	data := binding.IntToString(binding.NewInt())
	e := NewEntryWithData(data) // calls Bind for you
	waitForBinding()

	errOdd := errors.New("must be even")
	e.Validator = func(s string) error {
		if s == "43" {
			return errOdd
		}
		return nil
	}

	e.SetText("43")
	assert.Equal(t, errOdd, e.Validate(), "assigning Validator after Bind must take effect")

	e.SetText("42")
	assert.NoError(t, e.Validate())

	// Assigning Validator must not have displaced the conversion check Bind
	// installed -- both rules apply.
	e.SetText("not a number")
	assert.Error(t, e.Validate(), "the conversion check must survive a later Validator assignment")
}

func TestEntry_Unbind_KeepsValidator(t *testing.T) {
	test.NewTempApp(t)

	errNotB := errors.New("must be b")
	e := NewEntry()
	e.Validator = func(s string) error {
		if s != "b" {
			return errNotB
		}
		return nil
	}

	data := binding.NewString()
	e.Bind(data)
	waitForBinding()
	e.Unbind()
	waitForBinding()

	e.SetText("a")
	assert.Equal(t, errNotB, e.Validate(), "Unbind must not leave the widget with no validator")
}

// A conversion error wins over the app's rule: text that cannot be written to
// the bound source is invalid whatever the app's rule says about it.
func TestEntry_Bind_ConversionErrorWinsOverValidator(t *testing.T) {
	test.NewTempApp(t)

	e := NewEntry()
	e.Validator = func(string) error { return nil } // accepts everything

	data := binding.IntToString(binding.NewInt())
	e.Bind(data)
	waitForBinding()

	e.SetText("not a number")
	assert.Error(t, e.Validate(), "an unwritable value must be invalid even when the app's rule accepts it")

	e.SetText("42")
	assert.NoError(t, e.Validate())
}

// Binding alone still validates, as it did before -- the conversion check is
// composed with the app's rule, not conditional on one existing.
func TestEntry_Bind_ConversionErrorWithoutValidator(t *testing.T) {
	test.NewTempApp(t)

	e := NewEntry()
	data := binding.IntToString(binding.NewInt())
	e.Bind(data)
	waitForBinding()

	e.SetText("not a number")
	assert.Error(t, e.Validate())

	e.SetText("42")
	assert.NoError(t, e.Validate())
}

// The widgets that install their own Validator in their constructor
// (NumericEntry, SearchField, DateEntry, DateTimeEntry) used to lose it the
// moment they were bound.
func TestNumericEntry_Bind_KeepsOwnValidator(t *testing.T) {
	test.NewTempApp(t)

	e := NewNumericEntry()
	data := binding.NewString()
	e.Bind(data)
	waitForBinding()

	e.SetText("not a number")
	assert.Error(t, e.Validate(), "NumericEntry must keep its own validation once bound")

	e.SetText("42")
	assert.NoError(t, e.Validate())
}
