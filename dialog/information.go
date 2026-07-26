package dialog

import (
	"unicode"
	"unicode/utf8"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

func createInformationDialog(title, message string, icon bellui.Resource, parent bellui.Window) Dialog {
	d := newTextDialog(title, message, icon, parent)
	d.dismiss = &widget.Button{
		Text:     lang.L("OK"),
		OnTapped: d.Hide,
	}
	d.dismiss.ExtendBaseWidget(d.dismiss)
	d.create(container.NewGridWithColumns(1, d.dismiss))
	return d
}

// NewInformation creates a dialog over the specified window for user information.
// The title is used for the dialog window and message is the content.
// After creation you should call Show().
func NewInformation(title, message string, parent bellui.Window) Dialog {
	return createInformationDialog(title, message, theme.InfoIcon(), parent)
}

// ShowInformation shows a dialog over the specified window for user information.
// The title is used for the dialog window and message is the content.
func ShowInformation(title, message string, parent bellui.Window) {
	NewInformation(title, message, parent).Show()
}

// NewError creates a dialog over the specified window for an application error.
// The message is extracted from the provided error (should not be nil).
// After creation you should call Show().
func NewError(err error, parent bellui.Window) Dialog {
	dialogText := err.Error()
	r, size := utf8.DecodeRuneInString(dialogText)
	if r != utf8.RuneError {
		dialogText = string(unicode.ToUpper(r)) + dialogText[size:]
	}
	return createInformationDialog(lang.L("Error"), dialogText, theme.ErrorIcon(), parent)
}

// ShowError shows a dialog over the specified window for an application error.
// The message is extracted from the provided error (should not be nil).
func ShowError(err error, parent bellui.Window) {
	NewError(err, parent).Show()
}
