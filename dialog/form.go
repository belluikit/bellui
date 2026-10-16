package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// FormDialog is a simple dialog window for displaying FormItems inside a form.
//
// Since: 1.0
type FormDialog struct {
	*dialog
	items   []*widget.FormItem
	confirm *widget.Button
	cancel  *widget.Button
}

// Submit will submit the form and then hide the dialog if validation passes.
//
// Since: 1.0
func (d *FormDialog) Submit() {
	if d.confirm.Disabled() {
		return
	}

	d.hideWithResponse(true)
}

// setSubmitState is intended to run when the form validation changes to
// enable/disable the submit button accordingly.
func (d *FormDialog) setSubmitState(err error) {
	if err != nil {
		d.confirm.Disable()
		return
	}

	d.confirm.Enable()
}

// NewForm creates and returns a dialog over the specified application using
// the provided FormItems. The cancel button will have the dismiss text set and the confirm button will
// use the confirm text. The response callback is called on user action after validation passes.
// If any Validatable widget reports that validation has failed, then the confirm
// button will be disabled. The initial state of the confirm button will reflect the initial
// validation state of the items added to the form dialog.
//
// Since: 1.0
func NewForm(title, confirm, dismiss string, items []*widget.FormItem, callback func(bool), parent bellui.Window) *FormDialog {
	form := widget.NewForm(items...)

	d := &dialog{content: form, callback: callback, title: title, parent: parent, windowed: true}
	d.dismiss = &widget.Button{
		Text: dismiss, Icon: theme.CancelIcon(),
		OnTapped: d.Hide,
	}
	d.dismiss.ExtendBaseWidget(d.dismiss)
	confirmBtn := &widget.Button{
		Text: confirm, Icon: theme.ConfirmIcon(), Importance: widget.HighImportance,
		OnTapped: func() { d.hideWithResponse(true) },
	}
	confirmBtn.ExtendBaseWidget(confirmBtn)
	formDialog := &FormDialog{
		dialog:  d,
		items:   items,
		confirm: confirmBtn,
		cancel:  d.dismiss,
	}

	formDialog.setSubmitState(form.Validate())
	form.SetOnValidationChanged(formDialog.setSubmitState)

	d.create(container.NewGridWithColumns(2, d.dismiss, confirmBtn))
	return formDialog
}

// ShowForm shows a dialog over the specified application using
// the provided FormItems. The cancel button will have the dismiss text set and the confirm button will
// use the confirm text. The response callback is called on user action after validation passes.
// If any Validatable widget reports that validation has failed, then the confirm
// button will be disabled. The initial state of the confirm button will reflect the initial
// validation state of the items added to the form dialog.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
//
// Since: 1.0
func ShowForm(title, confirm, dismiss string, content []*widget.FormItem, callback func(bool), parent bellui.Window) {
	NewForm(title, confirm, dismiss, content, callback, parent).Show()
}
