package cache

import (
	"sync/atomic"

	"github.com/belluikit/bellui"
)

// selectableModeSet is set the first time any canvas is given a selectable
// mode other than bellui.SelectablePerWidget, and never cleared. Until then
// every canvas is in the default mode, so a widget can work out whether it is
// selectable from its own field without looking up its canvas, which was about
// a tenth of a Label.SetText (#492).
var selectableModeSet atomic.Bool

// NoteSelectableMode records that a canvas's selectable mode was set to mode.
// Every canvas implementation calls it from SetSelectableMode.
func NoteSelectableMode(mode bellui.SelectableMode) {
	if mode != bellui.SelectablePerWidget {
		selectableModeSet.Store(true)
	}
}

// SelectableModeSet reports whether any canvas may be in a selectable mode
// other than bellui.SelectablePerWidget.
func SelectableModeSet() bool {
	return selectableModeSet.Load()
}
