//go:build accessibility && linux && !android && !wasm && !js

package glfw

import "github.com/belluikit/bellui/core"

// AT-SPI2 role numbers (AtspiRole enum, at-spi2-core). Pinned to the numeric
// values of the target library version — the tail is only ever appended to, so
// these stay stable. Only the roles the bellui semantic layer produces are
// named; everything else maps to atspiRoleUnknown.
const (
	atspiRoleAlert        = 2
	atspiRoleCheckBox     = 7
	atspiRoleComboBox     = 11
	atspiRoleDialog       = 16
	atspiRoleFrame        = 23
	atspiRoleImage        = 27
	atspiRoleLabel        = 29
	atspiRoleList         = 31
	atspiRoleListItem     = 32
	atspiRoleMenu         = 33
	atspiRoleMenuBar      = 34
	atspiRoleMenuItem     = 35
	atspiRolePageTab      = 37 // a single tab
	atspiRolePageTabList  = 38 // the tab strip
	atspiRolePanel        = 39
	atspiRolePasswordTxt  = 40
	atspiRoleProgressBar  = 42
	atspiRolePushButton   = 43
	atspiRoleRadioButton  = 44
	atspiRoleScrollBar    = 48
	atspiRoleSeparator    = 50
	atspiRoleSlider       = 51
	atspiRoleSpinButton   = 52
	atspiRoleTable        = 55
	atspiRoleTableCell    = 56
	atspiRoleToggleBtn    = 62
	atspiRoleToolBar      = 63
	atspiRoleToolTip      = 64
	atspiRoleTree         = 65
	atspiRoleUnknown      = 67
	atspiRoleEntry        = 79
	atspiRoleHeading      = 83
	atspiRoleApplication  = 75
	atspiRoleForm         = 87
	atspiRoleLink         = 88
	atspiRoleTableRow     = 90
	atspiRoleTreeItem     = 91
	atspiRoleGrouping     = 99
	atspiRoleNotification = 101
	atspiRoleLandmark     = 110
)

// atspiRoleInfo pairs an AtspiRole number with the human-readable role name
// Orca announces (GetRoleName).
type atspiRoleInfo struct {
	num  uint32
	name string
}

// atspiRoleTable maps each bellui semantic role to its AT-SPI2 role. A table
// rather than a switch keeps atspiRole trivial (gocyclo) and the mapping
// scannable. Roles absent here fall back to atspiRoleUnknown.
var atspiRoleMap = map[core.AccessibleRole]atspiRoleInfo{
	core.RoleButton:           {atspiRolePushButton, "push button"},
	core.RoleCheckbox:         {atspiRoleCheckBox, "check box"},
	core.RoleRadioButton:      {atspiRoleRadioButton, "radio button"},
	core.RoleRadioGroup:       {atspiRolePanel, "panel"},
	core.RoleSwitch:           {atspiRoleToggleBtn, "toggle button"},
	core.RoleTextBox:          {atspiRoleEntry, "entry"},
	core.RoleMultiLineTextBox: {atspiRoleEntry, "entry"},
	core.RolePasswordField:    {atspiRolePasswordTxt, "password text"},
	core.RoleComboBox:         {atspiRoleComboBox, "combo box"},
	core.RoleSearchField:      {atspiRoleEntry, "entry"},
	core.RoleSlider:           {atspiRoleSlider, "slider"},
	core.RoleSpinButton:       {atspiRoleSpinButton, "spin button"},
	core.RoleProgressBar:      {atspiRoleProgressBar, "progress bar"},
	core.RoleLabel:            {atspiRoleLabel, "label"},
	core.RoleLink:             {atspiRoleLink, "link"},
	core.RoleImage:            {atspiRoleImage, "image"},
	core.RoleList:             {atspiRoleList, "list"},
	core.RoleListItem:         {atspiRoleListItem, "list item"},
	core.RoleTree:             {atspiRoleTree, "tree"},
	core.RoleTreeItem:         {atspiRoleTreeItem, "tree item"},
	core.RoleTable:            {atspiRoleTable, "table"},
	core.RoleTableRow:         {atspiRoleTableRow, "table row"},
	core.RoleTableCell:        {atspiRoleTableCell, "table cell"},
	core.RoleGrid:             {atspiRoleTable, "table"},
	core.RoleTab:              {atspiRolePageTab, "page tab"},
	core.RoleTabList:          {atspiRolePageTabList, "page tab list"},
	core.RoleTabPanel:         {atspiRolePanel, "panel"},
	core.RoleMenu:             {atspiRoleMenu, "menu"},
	core.RoleMenuItem:         {atspiRoleMenuItem, "menu item"},
	core.RoleMenuBar:          {atspiRoleMenuBar, "menu bar"},
	core.RoleToolbar:          {atspiRoleToolBar, "tool bar"},
	core.RoleNavigation:       {atspiRoleLandmark, "landmark"},
	core.RoleDialog:           {atspiRoleDialog, "dialog"},
	core.RoleAlertDialog:      {atspiRoleAlert, "alert"},
	core.RoleForm:             {atspiRoleForm, "form"},
	core.RoleGroup:            {atspiRoleGrouping, "grouping"},
	core.RoleAccordion:        {atspiRolePanel, "panel"},
	core.RoleCard:             {atspiRolePanel, "panel"},
	core.RoleBadge:            {atspiRoleLabel, "label"},
	core.RoleStatus:           {atspiRoleNotification, "notification"},
	core.RoleTooltip:          {atspiRoleToolTip, "tool tip"},
	core.RoleSeparator:        {atspiRoleSeparator, "separator"},
	core.RoleScrollBar:        {atspiRoleScrollBar, "scroll bar"},
	core.RoleRegion:           {atspiRoleLandmark, "landmark"},
	core.RoleHeading:          {atspiRoleHeading, "heading"},
}

// atspiRole maps a bellui semantic role to its AT-SPI2 role number and the
// human-readable role name Orca announces (GetRoleName).
func atspiRole(r core.AccessibleRole) (uint32, string) {
	if info, ok := atspiRoleMap[r]; ok {
		return info.num, info.name
	}
	return atspiRoleUnknown, "unknown"
}

// AT-SPI2 state bits (AtspiStateType enum). The wire format packs them into a
// two-word "au" bitfield; bits >= 32 live in word 1.
const (
	atspiStateActive        = 1
	atspiStateChecked       = 4
	atspiStateCollapsed     = 5
	atspiStateEditable      = 7
	atspiStateEnabled       = 8
	atspiStateExpandable    = 9
	atspiStateExpanded      = 10
	atspiStateFocusable     = 11
	atspiStateFocused       = 12
	atspiStateModal         = 16
	atspiStateMultiLine     = 17
	atspiStatePressed       = 20
	atspiStateSelectable    = 22
	atspiStateSelected      = 23
	atspiStateSensitive     = 24
	atspiStateShowing       = 25
	atspiStateSingleLine    = 26
	atspiStateVisible       = 30
	atspiStateIndeterminate = 32
	atspiStateRequired      = 33
	atspiStateReadOnly      = 43
	atspiStateCheckable     = 41
)

// setState sets bit b in the two-word AT-SPI state bitfield.
func setState(s *[2]uint32, b uint) { s[b/32] |= 1 << (b % 32) }

// atspiStates derives the two-word AT-SPI state bitfield for a semantic node.
// focused carries the resolved keyboard focus (which may sit on an active
// descendant rather than the node's own StateFocused bit).
func atspiStates(n *nodeStateInput, focused bool) [2]uint32 {
	var s [2]uint32

	// Always visible/showing — culled nodes are never exported.
	setState(&s, atspiStateVisible)
	setState(&s, atspiStateShowing)

	if !n.disabled {
		setState(&s, atspiStateEnabled)
		setState(&s, atspiStateSensitive)
	}
	if n.focusable {
		setState(&s, atspiStateFocusable)
	}
	if focused {
		setState(&s, atspiStateFocused)
	}
	if n.checkable {
		setState(&s, atspiStateCheckable)
		if n.checked {
			setState(&s, atspiStateChecked)
			setState(&s, atspiStatePressed)
		}
		if n.indeterminate {
			setState(&s, atspiStateIndeterminate)
		}
	}
	if n.expandable {
		setState(&s, atspiStateExpandable)
		if n.expanded {
			setState(&s, atspiStateExpanded)
		} else {
			setState(&s, atspiStateCollapsed)
		}
	}
	if n.selectable {
		setState(&s, atspiStateSelectable)
		if n.selected {
			setState(&s, atspiStateSelected)
		}
	}
	if n.editable {
		setState(&s, atspiStateEditable)
		if n.multiLine {
			setState(&s, atspiStateMultiLine)
		} else {
			setState(&s, atspiStateSingleLine)
		}
	}
	if n.readOnly {
		setState(&s, atspiStateReadOnly)
	}
	if n.required {
		setState(&s, atspiStateRequired)
	}
	if n.modal {
		setState(&s, atspiStateModal)
	}
	if n.active {
		setState(&s, atspiStateActive)
	}
	return s
}

// nodeStateInput is the decoded, platform-neutral state a semantic node carries
// into atspiStates — computed once during reconcile so the mapping stays a pure
// function testable without a live bus.
type nodeStateInput struct {
	disabled      bool
	focusable     bool
	checkable     bool
	checked       bool
	indeterminate bool
	expandable    bool
	expanded      bool
	selectable    bool
	selected      bool
	editable      bool
	multiLine     bool
	readOnly      bool
	required      bool
	modal         bool
	active        bool
}
