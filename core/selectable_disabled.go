package core

// SelectableWhenDisabled is implemented by a Disableable whose text stays
// selectable and copyable while it is disabled, so it may still take the focus
// a selection needs: from a click, from code or from assistive tech. Tab
// traversal skips it all the same. The focus manager refuses the focus to
// every other disabled widget (#1251).
//
// widget.Entry answers true: a disabled Entry keeps its text selectable and
// copyable. So do the selection layers of widget.Label and widget.RichText,
// which report themselves disabled only so that Tab skips them. A disabled
// RichEntry or code editor answers nothing, and can't be focused at all.
//
// On mobile a disabled widget that summons the soft keyboard is refused the
// focus even when it answers true, so a disabled Entry shows no keyboard.
//
// Since: 1.3
type SelectableWhenDisabled interface {
	SelectableWhenDisabled() bool
}
