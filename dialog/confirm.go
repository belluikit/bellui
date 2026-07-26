package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// ConfirmDialog is like the standard Dialog but with an additional confirmation button
type ConfirmDialog struct {
	*dialog

	confirm *widget.Button
}

// Confirm instructs the dialog to close agreeing with whatever content was displayed.
//
// Since: 2.6
func (d *ConfirmDialog) Confirm() {
	d.hideWithResponse(true)
}

// SetConfirmText allows custom text to be set in the confirmation button
func (d *ConfirmDialog) SetConfirmText(label string) {
	d.confirm.SetText(label)
	d.refreshHost()
}

// SetConfirmImportance sets the importance level of the confirm button.
//
// Since 2.4
func (d *ConfirmDialog) SetConfirmImportance(importance widget.Importance) {
	d.confirm.Importance = importance
}

// NewConfirm creates a dialog over the specified window for user confirmation.
// The title is used for the dialog window and message is the content.
// The callback is executed when the user decides. After creation you should call Show().
func NewConfirm(title, message string, callback func(bool), parent bellui.Window) *ConfirmDialog {
	d := newTextDialog(title, message, theme.QuestionIcon(), parent)
	d.callback = callback

	d.dismiss = &widget.Button{
		Text: lang.L("No"), Icon: theme.CancelIcon(),
		OnTapped: d.Hide,
	}
	d.dismiss.ExtendBaseWidget(d.dismiss)
	confirm := &widget.Button{
		Text: lang.L("Yes"), Icon: theme.ConfirmIcon(), Importance: widget.HighImportance,
		OnTapped: func() {
			d.hideWithResponse(true)
		},
	}
	confirm.ExtendBaseWidget(confirm)
	d.create(container.NewGridWithColumns(2, d.dismiss, confirm))

	return &ConfirmDialog{dialog: d, confirm: confirm}
}

// ShowConfirm shows a dialog over the specified window for a user
// confirmation. The title is used for the dialog window and message is the content.
// The callback is executed when the user decides.
func ShowConfirm(title, message string, callback func(bool), parent bellui.Window) {
	NewConfirm(title, message, callback, parent).Show()
}
