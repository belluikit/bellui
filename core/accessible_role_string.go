package core

import "strconv"

// roleNames maps each AccessibleRole to a readable name, indexed in the same
// iota order the constants are declared in accessible.go.
var roleNames = [...]string{
	RoleNone:             "None",
	RoleButton:           "Button",
	RoleCheckbox:         "Checkbox",
	RoleRadioButton:      "RadioButton",
	RoleRadioGroup:       "RadioGroup",
	RoleSwitch:           "Switch",
	RoleTextBox:          "TextBox",
	RoleMultiLineTextBox: "MultiLineTextBox",
	RolePasswordField:    "PasswordField",
	RoleComboBox:         "ComboBox",
	RoleSearchField:      "SearchField",
	RoleSlider:           "Slider",
	RoleSpinButton:       "SpinButton",
	RoleProgressBar:      "ProgressBar",
	RoleLabel:            "Label",
	RoleLink:             "Link",
	RoleImage:            "Image",
	RoleList:             "List",
	RoleListItem:         "ListItem",
	RoleTree:             "Tree",
	RoleTreeItem:         "TreeItem",
	RoleTable:            "Table",
	RoleTableRow:         "TableRow",
	RoleTableCell:        "TableCell",
	RoleGrid:             "Grid",
	RoleTab:              "Tab",
	RoleTabList:          "TabList",
	RoleTabPanel:         "TabPanel",
	RoleMenu:             "Menu",
	RoleMenuItem:         "MenuItem",
	RoleMenuBar:          "MenuBar",
	RoleToolbar:          "Toolbar",
	RoleNavigation:       "Navigation",
	RoleDialog:           "Dialog",
	RoleAlertDialog:      "AlertDialog",
	RoleForm:             "Form",
	RoleGroup:            "Group",
	RoleAccordion:        "Accordion",
	RoleCard:             "Card",
	RoleBadge:            "Badge",
	RoleStatus:           "Status",
	RoleTooltip:          "Tooltip",
	RoleSeparator:        "Separator",
	RoleScrollBar:        "ScrollBar",
	RoleRegion:           "Region",
	RoleHeading:          "Heading",
}

// String returns a human-readable name for the role, or "AccessibleRole(n)"
// for an unrecognized value. It is intended for debugging and developer
// tooling (logs, the widget inspector); assistive-technology backends map
// roles to their own platform constants rather than using this text.
func (r AccessibleRole) String() string {
	if r >= 0 && int(r) < len(roleNames) {
		if name := roleNames[r]; name != "" {
			return name
		}
	}
	return "AccessibleRole(" + strconv.Itoa(int(r)) + ")"
}
