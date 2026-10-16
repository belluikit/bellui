package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestNumericEntry_RejectsLetters(t *testing.T) {
	e := widget.NewNumericEntry()
	w := test.NewTempWindow(t, e)
	_ = w

	test.Type(e, "abc123def")
	assert.Equal(t, "123", e.Text)
}

func TestNumericEntry_AllowsDecimal(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	test.Type(e, "3.14")
	assert.Equal(t, "3.14", e.Text)
}

func TestNumericEntry_SingleDecimalPoint(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	test.Type(e, "3..14")
	assert.Equal(t, "3.14", e.Text)
}

func TestNumericEntry_AllowsNegative(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	test.Type(e, "-5")
	assert.Equal(t, "-5", e.Text)
}

func TestNumericEntry_RejectsMinusInMiddle(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	test.Type(e, "5-3")
	assert.Equal(t, "53", e.Text)
}

func TestIntegerEntry_RejectsDecimal(t *testing.T) {
	e := widget.NewIntegerEntry()
	test.NewTempWindow(t, e)

	test.Type(e, "3.14")
	assert.Equal(t, "314", e.Text)
}

func TestNumericEntry_RightAligned(t *testing.T) {
	e := widget.NewNumericEntry()
	assert.Equal(t, bellui.TextAlignTrailing, e.Alignment)
}

func TestNumericEntry_Value(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	test.Type(e, "42.5")
	v, err := e.Value()
	assert.NoError(t, err)
	assert.Equal(t, 42.5, v)
}

func TestNumericEntry_SetValue(t *testing.T) {
	e := widget.NewNumericEntry()
	e.SetValue(99.9)
	assert.Equal(t, "99.9", e.Text)
}

func TestIntegerEntry_SetValue(t *testing.T) {
	e := widget.NewIntegerEntry()
	e.SetValue(42)
	assert.Equal(t, "42", e.Text)
}

func TestNumericEntry_Bounds(t *testing.T) {
	e := widget.NewNumericEntry()
	min, max := 0.0, 100.0
	e.Min = &min
	e.Max = &max

	e.SetText("50")
	assert.NoError(t, e.Validate())

	e.SetText("150")
	assert.Error(t, e.Validate())

	e.SetText("-5")
	assert.Error(t, e.Validate())
}

func TestNumericEntry_OnValueChanged(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	var got float64
	e.OnValueChanged = func(v float64) { got = v }

	test.Type(e, "7")
	assert.Equal(t, 7.0, got)
}

func TestNumericEntry_OnValueChanged_NotFiredFromValidate(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	var count int
	e.OnValueChanged = func(float64) { count++ }

	test.Type(e, "42")
	base := count
	assert.Positive(t, base, "OnValueChanged should fire on edit")

	// Validation may be called speculatively/repeatedly; it must not re-fire
	// the user callback (regression for pre-rebrand tracker #290).
	e.Validate()
	e.SetBounds(0, 100) // calls Validate internally
	assert.Equal(t, base, count, "OnValueChanged must not fire from validation")
}

func TestNumericEntry_PasteDoesNotClobberClipboard(t *testing.T) {
	e := widget.NewNumericEntry()
	test.NewTempWindow(t, e)

	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("abc123def")
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})

	assert.Equal(t, "123", e.Text)
	assert.Equal(t, "abc123def", cb.Content(), "paste must not mutate the system clipboard (#290)")
}

func TestNumericEntry_DisallowNegative(t *testing.T) {
	e := widget.NewNumericEntry()
	e.AllowNegative = false
	test.NewTempWindow(t, e)

	test.Type(e, "-5")
	assert.Equal(t, "5", e.Text)
}

func TestNumericEntry_PasteFiltering(t *testing.T) {
	e := widget.NewNumericEntry()
	w := test.NewTempWindow(t, e)
	_ = w

	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("abc123def")
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})
	assert.Equal(t, "123", e.Text)
}

func TestNumericEntry_PasteMultipleDecimals(t *testing.T) {
	e := widget.NewNumericEntry()
	w := test.NewTempWindow(t, e)
	_ = w

	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("3.5.2")
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})
	assert.Equal(t, "3.52", e.Text)
}

// TestNumericEntry_PasteViaMemoryClipboard confirms the filtered-entry paste
// path stays intact after the pre-rebrand tracker #581 flavor negotiation: NumericEntry re-dispatches
// through an internal memoryClipboard that implements NEITHER bellui.HTMLClipboard
// NOR bellui.ImageClipboard, so Entry's flavor assertions must fall through
// (ok == false) and paste the plain, filtered text.
func TestNumericEntry_PasteViaMemoryClipboard(t *testing.T) {
	e := widget.NewNumericEntry()
	w := test.NewTempWindow(t, e)
	_ = w

	cb := bellui.CurrentApp().Clipboard() // no flavor interfaces on the memory shim.
	cb.SetContent("x9y8z")
	e.TypedShortcut(&bellui.ShortcutPaste{Clipboard: cb})
	assert.Equal(t, "98", e.Text)
}

func TestNumericEntry_Disabled(t *testing.T) {
	e := widget.NewNumericEntry()
	e.SetValue(10)
	e.Disable()

	assert.True(t, e.Disabled())
	assert.Equal(t, "10", e.Text)
}

func TestNumericEntry_ReadOnly(t *testing.T) {
	e := widget.NewNumericEntry()
	e.SetValue(42.5)
	e.ReadOnly = true

	assert.Equal(t, "42.5", e.Text)
	assert.True(t, e.ReadOnly)
}

func TestNumericEntry_SetBounds(t *testing.T) {
	e := widget.NewNumericEntry()
	e.SetBounds(10, 50)

	assert.NotNil(t, e.Min)
	assert.NotNil(t, e.Max)
	assert.Equal(t, 10.0, *e.Min)
	assert.Equal(t, 50.0, *e.Max)

	e.SetText("5")
	assert.Error(t, e.Validate())

	e.SetText("60")
	assert.Error(t, e.Validate())

	e.SetText("25")
	assert.NoError(t, e.Validate())
}

func TestNumericEntry_ValidateWithBorder(t *testing.T) {
	e := widget.NewNumericEntry()
	assert.True(t, e.ValidateWithBorder, "NumericEntry should use border validation")
}

func TestNumericEntry_EmptyIsValid(t *testing.T) {
	e := widget.NewNumericEntry()
	e.SetText("")
	assert.NoError(t, e.Validate())
}

func TestNumericEntry_LocaleFormat_SetValue(t *testing.T) {
	e := widget.NewNumericEntry()
	e.LocaleFormat = true
	e.SetValue(1234.5)

	// The text should use locale separators.
	v, err := e.Value()
	assert.NoError(t, err)
	assert.InDelta(t, 1234.5, v, 0.001)
}

func TestNumericEntry_LocaleFormat_DecimalSeparator(t *testing.T) {
	e := widget.NewNumericEntry()
	e.LocaleFormat = true
	test.NewTempWindow(t, e)

	dec := lang.DecimalSeparator()
	// Type "3{dec}14" using the locale's decimal separator.
	input := "3" + dec + "14"
	e.SetText(input)

	v, err := e.Value()
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, v, 0.001)
}

func TestNumericEntry_LocaleFormat_Validate(t *testing.T) {
	e := widget.NewNumericEntry()
	e.LocaleFormat = true
	min := 0.0
	max := 100.0
	e.Min = &min
	e.Max = &max

	dec := lang.DecimalSeparator()
	e.SetText("50" + dec + "5")
	assert.NoError(t, e.Validate())

	e.SetText("150")
	assert.Error(t, e.Validate())
}

func TestNumericEntry_LocaleFormat_Off_DefaultBehavior(t *testing.T) {
	// Verify that without LocaleFormat, behavior is unchanged.
	e := widget.NewNumericEntry()
	e.SetValue(3.14)
	assert.Equal(t, "3.14", e.Text)

	v, err := e.Value()
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, v, 0.001)
}
