package widget

import "github.com/belluikit/bellui/core"

// The widgets here draw their own disabled look, so the paint walk does not dim
// them again (core.DisabledLook, #1223). Every walk dims a disabled widget's
// subtree to 38% as a fallback; on these that dimmed MD3's disabled colours a
// second time, a Button's label to about 14% of the text colour instead of 38%.
//
// A widget belongs here when, disabled, it looks disabled without the walk's
// help: it draws the disabled colours itself, or it disables children that do.
// SwipeArea and WheelPicker draw no disabled look and keep the fallback, as
// does a widget an app builds on DisableableWidget.
//
// Types embedding one of these (DateEntry, DateTimeEntry and SelectEntry embed
// Entry, FileTree embeds Tree) inherit the method.

var _ core.DisabledLook = (*Button)(nil)

// They draw the disabled colours themselves.

func (*AuthButton) DrawsDisabledLook() bool        { return true }
func (*Button) DrawsDisabledLook() bool            { return true }
func (*Carousel) DrawsDisabledLook() bool          { return true }
func (*Check) DrawsDisabledLook() bool             { return true }
func (*Chip) DrawsDisabledLook() bool              { return true }
func (*DatePicker) DrawsDisabledLook() bool        { return true }
func (*Entry) DrawsDisabledLook() bool             { return true }
func (*FAB) DrawsDisabledLook() bool               { return true }
func (*IconButton) DrawsDisabledLook() bool        { return true }
func (*List) DrawsDisabledLook() bool              { return true }
func (*NavigationBar) DrawsDisabledLook() bool     { return true }
func (*navBarItemWidget) DrawsDisabledLook() bool  { return true }
func (*NavigationRail) DrawsDisabledLook() bool    { return true }
func (*navRailItemWidget) DrawsDisabledLook() bool { return true }
func (*radioItem) DrawsDisabledLook() bool         { return true }
func (*RangeSlider) DrawsDisabledLook() bool       { return true }
func (*RichEntry) DrawsDisabledLook() bool         { return true }
func (*SearchSelect) DrawsDisabledLook() bool      { return true }
func (*SegmentedButton) DrawsDisabledLook() bool   { return true }
func (*Select) DrawsDisabledLook() bool            { return true }
func (*Slider) DrawsDisabledLook() bool            { return true }
func (*SplitButton) DrawsDisabledLook() bool       { return true }
func (*Switch) DrawsDisabledLook() bool            { return true }
func (*TimePicker) DrawsDisabledLook() bool        { return true }
func (*Tree) DrawsDisabledLook() bool              { return true }
func (*TreeView) DrawsDisabledLook() bool          { return true }

// They disable children that draw the disabled colours, and dimming the whole
// subtree would dim those children twice. TopAppBar disables its controls and
// keeps its title at full colour (#1226, #1251). BottomAppBar disables its FAB;
// the app's actions in it are the app's.

func (*BottomAppBar) DrawsDisabledLook() bool   { return true }
func (*ButtonGroup) DrawsDisabledLook() bool    { return true }
func (*CheckGroup) DrawsDisabledLook() bool     { return true }
func (*DateTimePicker) DrawsDisabledLook() bool { return true }
func (*FontPicker) DrawsDisabledLook() bool     { return true }
func (*Form) DrawsDisabledLook() bool           { return true }
func (*RadioGroup) DrawsDisabledLook() bool     { return true }
func (*SpinBox) DrawsDisabledLook() bool        { return true }
func (*ThemeSelector) DrawsDisabledLook() bool  { return true }
func (*TopAppBar) DrawsDisabledLook() bool      { return true }

// focusSelectable, a selectable Label's selection layer, reports itself
// disabled so that Tab skips it. It isn't disabled, and must not be dimmed:
// the walk dimmed a selectable Label's selection to 38% on desktop.
func (*focusSelectable) DrawsDisabledLook() bool { return true }
