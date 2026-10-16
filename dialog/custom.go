package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

var _ Dialog = (*CustomDialog)(nil)

// CustomDialog implements a custom dialog.
//
// Since: 1.0
type CustomDialog struct {
	*dialog
}

// NewCustom creates and returns a dialog over the specified application using custom
// content. The button will have the dismiss text set.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
func NewCustom(title, dismiss string, content core.CanvasObject, parent bellui.Window) *CustomDialog {
	d := &dialog{content: content, title: title, parent: parent, windowed: true}

	d.dismiss = &widget.Button{Text: dismiss, OnTapped: d.Hide}
	d.dismiss.ExtendBaseWidget(d.dismiss)
	d.create(container.NewGridWithColumns(1, d.dismiss))

	return &CustomDialog{dialog: d}
}

// ShowCustom shows a dialog over the specified application using custom
// content. The button will have the dismiss text set.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
func ShowCustom(title, dismiss string, content core.CanvasObject, parent bellui.Window) {
	NewCustom(title, dismiss, content, parent).Show()
}

// NewCustomWithoutButtons creates a new custom dialog without any buttons.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
//
// Since: 1.0
func NewCustomWithoutButtons(title string, content core.CanvasObject, parent bellui.Window) *CustomDialog {
	d := &dialog{content: content, title: title, parent: parent, windowed: true}
	d.create(container.NewGridWithColumns(1))

	return &CustomDialog{dialog: d}
}

// SetButtons sets the row of buttons at the bottom of the dialog.
// Passing an empty slice will result in a dialog with no buttons.
//
// Since: 1.0
func (d *CustomDialog) SetButtons(buttons []core.CanvasObject) {
	d.dismiss = nil // New button row invalidates possible dismiss button.
	d.setButtons(container.NewGridWithRows(1, buttons...))
}

// SetIcon sets an icon to be shown in the top right of the dialog.
// Passing a nil resource will remove the icon from the dialog.
//
// Since: 1.0
func (d *CustomDialog) SetIcon(icon bellui.Resource) {
	d.setIcon(icon)
}

// ShowCustomWithoutButtons shows a dialog, without buttons, over the specified application
// using custom content.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
//
// Since: 1.0
func ShowCustomWithoutButtons(title string, content core.CanvasObject, parent bellui.Window) {
	NewCustomWithoutButtons(title, content, parent).Show()
}

// NewCustomConfirm creates and returns a dialog over the specified application using
// custom content. The cancel button will have the dismiss text set and the "OK" will
// use the confirm text. The response callback is called on user action.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
func NewCustomConfirm(title, confirm, dismiss string, content core.CanvasObject,
	callback func(bool), parent bellui.Window,
) *ConfirmDialog {
	d := &dialog{content: content, title: title, parent: parent, callback: callback, windowed: true}

	d.dismiss = &widget.Button{
		Text: dismiss, Icon: theme.CancelIcon(),
		OnTapped: d.Hide,
	}
	d.dismiss.ExtendBaseWidget(d.dismiss)
	ok := &widget.Button{
		Text: confirm, Icon: theme.ConfirmIcon(), Importance: widget.HighImportance,
		OnTapped: func() {
			d.hideWithResponse(true)
		},
	}
	ok.ExtendBaseWidget(ok)
	d.create(container.NewGridWithColumns(2, d.dismiss, ok))

	return &ConfirmDialog{dialog: d, confirm: ok}
}

// ShowCustomConfirm shows a dialog over the specified application using custom
// content. The cancel button will have the dismiss text set and the "OK" will use
// the confirm text. The response callback is called on user action.
// The MinSize() of the CanvasObject passed will be used to set the size of the window.
func ShowCustomConfirm(title, confirm, dismiss string, content core.CanvasObject,
	callback func(bool), parent bellui.Window,
) {
	NewCustomConfirm(title, confirm, dismiss, content, callback, parent).Show()
}
