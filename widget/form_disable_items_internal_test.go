package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// formLabelColor is the colour item i's label is drawn in.
func formLabelColor(f *Form, i int) any {
	return f.itemGrid.Objects[i*2].(*bellui.Container).Objects[0].(*canvas.Text).Color
}

// Disable disables the form's items too, as a disabled container does its
// content, and its labels take the disabled colour; it disabled only the
// submit and cancel buttons, so a form in a disabled tab kept live-looking
// fields and labels (#1403). Enable brings back only what Disable switched
// off: an item the app disabled itself stays disabled.
func TestForm_DisableDisablesItems(t *testing.T) {
	test.NewTempApp(t)
	name, email, own := NewEntry(), NewEntry(), NewEntry()
	own.Disable() // the app's own
	f := NewForm(NewFormItem("Name", name), NewFormItem("Email", email), NewFormItem("Own", own))
	f.OnSubmit = func() {}
	w := test.NewTempWindow(t, f)
	_ = w
	onSurface := theme.Color(theme.ColorNameOnSurface)
	disabled := theme.Color(theme.ColorNameDisabled)
	assert.Equal(t, onSurface, formLabelColor(f, 0))
	assert.Equal(t, disabled, formLabelColor(f, 2), "the app's disabled item")

	f.Disable()
	assert.True(t, name.Disabled())
	assert.True(t, email.Disabled())
	for i := range f.Items {
		assert.Equal(t, disabled, formLabelColor(f, i), "label %d", i)
	}

	extra := NewEntry()
	f.Append("Extra", extra)
	assert.True(t, extra.Disabled(), "an item added while disabled is disabled")

	f.Enable()
	assert.False(t, name.Disabled())
	assert.False(t, email.Disabled())
	assert.False(t, extra.Disabled())
	assert.True(t, own.Disabled(), "the app disabled it, so it stays disabled")
	assert.Equal(t, onSurface, formLabelColor(f, 0))
	assert.Equal(t, disabled, formLabelColor(f, 2))
}

// A form disabled before it is drawn draws disabled.
func TestForm_DisableBeforeDrawn(t *testing.T) {
	test.NewTempApp(t)
	e := NewEntry()
	f := NewForm(NewFormItem("Name", e))
	f.Disable()
	assert.True(t, e.Disabled())
	test.NewTempWindow(t, f)
	assert.Equal(t, theme.Color(theme.ColorNameDisabled), formLabelColor(f, 0))
}

// An item that cannot be disabled, such as a Label, still gets its form label
// in the disabled colour while the form is disabled.
func TestForm_DisableLabelsItemsThatCannotBeDisabled(t *testing.T) {
	test.NewTempApp(t)
	f := NewForm(NewFormItem("Plain", NewLabel("read-only value")))
	test.NewTempWindow(t, f)
	assert.Equal(t, theme.Color(theme.ColorNameOnSurface), formLabelColor(f, 0))
	f.Disable()
	assert.Equal(t, theme.Color(theme.ColorNameDisabled), formLabelColor(f, 0))
	f.Enable()
	assert.Equal(t, theme.Color(theme.ColorNameOnSurface), formLabelColor(f, 0))
}
