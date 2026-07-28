package widget

import (
	"errors"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

var _ bellui.Validatable = (*Entry)(nil)

// hasValidator reports whether any validation rule applies -- the app's public
// [Entry.Validator], the data-binding conversion check installed by
// [Entry.Bind], or both.
func (e *Entry) hasValidator() bool {
	return e.Validator != nil || e.validatorInternal != nil
}

// runValidator composes the data-binding conversion check with the app's
// Validator. A conversion error wins: text that cannot be written to the bound
// source is invalid whatever the app's rule says about it.
func (e *Entry) runValidator(text string) error {
	if e.validatorInternal != nil {
		if err := e.validatorInternal(text); err != nil {
			return err
		}
	}
	if e.Validator != nil {
		return e.Validator(text)
	}
	return nil
}

// Validate validates the current text in the widget.
func (e *Entry) Validate() error {
	if !e.hasValidator() {
		return nil
	}

	err := e.runValidator(e.Text)
	e.SetValidationError(err)
	return err
}

// validate works like Validate but only updates the internal state and does not refresh.
func (e *Entry) validate() {
	if !e.hasValidator() {
		return
	}

	err := e.runValidator(e.Text)
	e.setValidationError(err)
}

// SetOnValidationChanged is intended for parent widgets or containers to hook into the validation.
// The function might be overwritten by a parent that cares about child validation (e.g. widget.Form).
func (e *Entry) SetOnValidationChanged(callback func(error)) {
	e.onValidationChanged = callback
}

// SetValidationError manually updates the validation status until the next input change.
func (e *Entry) SetValidationError(err error) {
	if !e.hasValidator() && !e.AlwaysShowValidationError {
		return
	}

	if !e.setValidationError(err) {
		return
	}

	e.Refresh()
}

// setValidationError sets the validation error and returns a bool to indicate if it changes.
// It assumes that the widget has a validator.
func (e *Entry) setValidationError(err error) bool {
	if err == nil && e.validationError == nil {
		return false
	}
	if errors.Is(err, e.validationError) {
		return false
	}

	changed := e.validationError != err
	e.validationError = err

	if e.onValidationChanged != nil && changed {
		e.onValidationChanged(err)
	}

	return true
}

var _ core.Widget = (*validationStatus)(nil)

type validationStatus struct {
	BaseWidget
	entry *Entry
}

func newValidationStatus(e *Entry) *validationStatus {
	rs := &validationStatus{
		entry: e,
	}

	rs.ExtendBaseWidget(rs)
	return rs
}

func (r *validationStatus) CreateRenderer() core.WidgetRenderer {
	icon := &canvas.Image{}
	icon.Hide()
	return &validationStatusRenderer{
		WidgetRenderer: NewSimpleRenderer(icon),
		icon:           icon,
		entry:          r.entry,
	}
}

var _ core.WidgetRenderer = (*validationStatusRenderer)(nil)

type validationStatusRenderer struct {
	core.WidgetRenderer
	entry *Entry
	icon  *canvas.Image
}

func (r *validationStatusRenderer) Layout(size geo.Size) {
	iconSize := r.entry.Theme().Size(theme.SizeNameInlineIcon)
	r.icon.Resize(geo.NewSquareSize(iconSize))
	r.icon.Move(geo.NewPos((size.Width-iconSize)/2, (size.Height-iconSize)/2))
}

func (r *validationStatusRenderer) MinSize() geo.Size {
	iconSize := r.entry.Theme().Size(theme.SizeNameInlineIcon)
	return geo.NewSquareSize(iconSize)
}

func (r *validationStatusRenderer) Refresh() {
	th := r.entry.Theme()
	if r.entry.Disabled() {
		r.icon.Hide()
		return
	}

	if r.entry.validationError == nil && r.entry.Text != "" && r.entry.hasValidator() {
		r.icon.Resource = th.Icon(theme.IconNameConfirm)
		r.icon.Show()
	} else if r.entry.validationError != nil && !r.entry.focused && (r.entry.dirty || r.entry.AlwaysShowValidationError) {
		r.icon.Resource = theme.NewErrorThemedResource(th.Icon(theme.IconNameError))
		r.icon.Show()
	} else {
		r.icon.Hide()
	}

	r.icon.Refresh()
}
