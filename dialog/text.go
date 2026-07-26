package dialog

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

const (
	// absolute max width of text dialogs
	// (prevent them from looking unnaturally large on desktop)
	maxTextDialogAbsoluteWidth float32 = 600

	// max width of text dialogs as a percentage of the current window width
	maxTextDialogWinPcntWidth float32 = .9
)

func newTextDialog(title, message string, icon bellui.Resource, parent bellui.Window) *dialog {
	d := &dialog{
		title:    title,
		icon:     icon,
		parent:   parent,
		content:  newCenterWrappedLabel(message),
		windowed: true,
	}
	d.beforeShowHook = createBeforeShowHook(d, message)

	return d
}

// returns a beforeShowHook that sets the desired width of the dialog to the min of:
// - width needed to show message without wrapping
// - maxTextDialogAbsoluteWidth
// - current window width * maxTextDialogWinPcntWidth
func createBeforeShowHook(d *dialog, message string) func() {
	// Until issue #4648 is resolved, we need to create a label here
	// rather than just using bellui.MeasureText, because the label's minsize
	// also depends on the internal padding that label adds, which is unknown here
	noWrapWidth := widget.NewLabel(message).MinSize().Width + padWidth + theme.Padding()*2
	return func() {
		if d.desiredSize.IsZero() {
			maxWinWitth := d.parent.Canvas().Size().Width * maxTextDialogWinPcntWidth
			w := geo.Min(geo.Min(noWrapWidth, maxTextDialogAbsoluteWidth), maxWinWitth)
			d.desiredSize = geo.NewSize(w, d.MinSize().Height)
		}
	}
}

func newCenterWrappedLabel(message string) core.CanvasObject {
	l := &widget.Label{Text: message, Alignment: bellui.TextAlignCenter, Wrapping: bellui.TextWrapWord}
	l.ExtendBaseWidget(l)
	return l
}
