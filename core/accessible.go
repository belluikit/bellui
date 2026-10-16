package core

import "github.com/belluikit/bellui/geo"

// AccessibleRole identifies the semantic purpose of a widget for
// assistive technologies. Values are inspired by WAI-ARIA roles.
type AccessibleRole int

const (
	RoleNone             AccessibleRole = iota // Not semantically meaningful
	RoleButton                                 // Clickable action trigger
	RoleCheckbox                               // Toggleable boolean
	RoleRadioButton                            // One-of-N selection
	RoleRadioGroup                             // Container for radio buttons
	RoleSwitch                                 // On/off toggle
	RoleTextBox                                // Single-line text input
	RoleMultiLineTextBox                       // Multi-line text input
	RolePasswordField                          // Masked text input
	RoleComboBox                               // Dropdown selector
	RoleSearchField                            // Search input
	RoleSlider                                 // Range selection
	RoleSpinButton                             // Numeric stepper
	RoleProgressBar                            // Determinate/indeterminate progress
	RoleLabel                                  // Static text
	RoleLink                                   // Hyperlink
	RoleImage                                  // Image content
	RoleList                                   // Scrollable list container
	RoleListItem                               // Item within a list
	RoleTree                                   // Hierarchical tree
	RoleTreeItem                               // Item within a tree
	RoleTable                                  // Data table
	RoleTableRow                               // Row in a table
	RoleTableCell                              // Cell in a table
	RoleGrid                                   // Grid layout
	RoleTab                                    // Tab button
	RoleTabList                                // Container of tabs
	RoleTabPanel                               // Content panel for a tab
	RoleMenu                                   // Menu container
	RoleMenuItem                               // Item in a menu
	RoleMenuBar                                // Top-level menu bar
	RoleToolbar                                // Toolbar container
	RoleNavigation                             // Navigation container
	RoleDialog                                 // Modal dialog
	RoleAlertDialog                            // Confirmation/alert dialog
	RoleForm                                   // Form container
	RoleGroup                                  // Generic grouping
	RoleAccordion                              // Expandable section container
	RoleCard                                   // Card container
	RoleBadge                                  // Notification indicator
	RoleStatus                                 // Status message (toast/snackbar)
	RoleTooltip                                // Tooltip popup
	RoleSeparator                              // Visual separator
	RoleScrollBar                              // Scroll indicator
	RoleRegion                                 // Generic landmark region
	RoleHeading                                // Section heading text
)

// ChildrenPresentational reports whether the descendants of a widget with
// this role are presentation details that must not appear in the semantic
// tree. It mirrors WAI-ARIA's "children presentational" characteristic: a
// button is announced as one control, so the internal text widget that
// paints its label carries no semantics of its own. Tree walkers drop the
// accessible descendants of such a node.
//
// Text-bearing roles that can host embedded interactive controls (text
// inputs with an action button, rich text with hyperlinks) are not in this
// set — see [AccessibleRole.NonInteractiveChildrenPresentational].
func (r AccessibleRole) ChildrenPresentational() bool {
	switch r {
	case RoleButton, RoleCheckbox, RoleRadioButton, RoleSwitch, RoleSlider,
		RoleSpinButton, RoleProgressBar, RoleScrollBar, RoleSeparator,
		RoleTab, RoleHeading, RoleLink, RoleImage,
		RoleMenuItem, RoleBadge, RoleTooltip:
		return true
	}
	return false
}

// NonInteractiveChildrenPresentational reports whether the widget's
// non-interactive accessible descendants are presentation details while its
// interactive descendants remain part of the semantic tree. This covers
// bellui's text-bearing controls: their renderers are built from internal
// accessible text widgets that would duplicate the control's own label or
// value (and are dropped), but they can also host real embedded controls
// that must stay reachable — an Entry's ActionItem button (a DateEntry's
// calendar, a SearchField's clear button) or hyperlinks inside rich text.
//
// Tree walkers keep the descendants that expose actions or respond to taps
// and drop the rest. WAI-ARIA does not mark textbox as children
// presentational; this is the analogous, selective treatment.
func (r AccessibleRole) NonInteractiveChildrenPresentational() bool {
	switch r {
	case RoleLabel, RoleTextBox, RoleMultiLineTextBox, RolePasswordField,
		RoleSearchField, RoleComboBox:
		return true
	}
	return false
}

// AccessibleState is a bitmask of boolean states for assistive technologies.
type AccessibleState uint32

const (
	StateChecked  AccessibleState = 1 << iota // Checkbox/switch is on
	StateExpanded                             // Accordion/tree node is open
	StateSelected                             // List/tab item is selected
	StateDisabled                             // Widget is disabled
	StateFocused                              // Widget has keyboard focus
	StateRequired                             // Form field is required
	StateReadOnly                             // Text field is read-only
	StatePartial                              // Indeterminate check state
	StateModal                                // Dialog is modal
)

// Has reports whether s contains the given state flag.
func (s AccessibleState) Has(flag AccessibleState) bool {
	return s&flag != 0
}

// AccessibleAction represents a named action that a widget supports.
type AccessibleAction struct {
	Name        string // Machine-readable name (e.g. "press", "toggle")
	Description string // Human-readable description
}

// Standard action names for use in [AccessibleActioner] implementations.
const (
	ActionPress     = "press"     // Button, link, menu item
	ActionToggle    = "toggle"    // Checkbox, switch
	ActionIncrement = "increment" // Slider, spinbox
	ActionDecrement = "decrement" // Slider, spinbox
	ActionExpand    = "expand"    // Accordion, tree node
	ActionCollapse  = "collapse"  // Accordion, tree node
	ActionSelect    = "select"    // List item, tab
	ActionDismiss   = "dismiss"   // Dialog, toast, snackbar
	ActionRename    = "rename"    // Tree/list rows with inline rename
	ActionDelete    = "delete"    // Tree/list rows, removable chips
	ActionUndo      = "undo"      // Text editors with undo history
	ActionRedo      = "redo"      // Text editors with redo history
)

// Accessible is implemented by widgets that provide semantic information
// for assistive technologies such as screen readers.
//
// Widgets opt in by implementing this interface, following the same
// composition pattern as [Disableable] or [event.Tappable].
type Accessible interface {
	// AccessibleRole returns the semantic role of this widget.
	AccessibleRole() AccessibleRole

	// AccessibleLabel returns the human-readable name of this widget.
	// For a button this is the button text; for an entry, the placeholder.
	AccessibleLabel() string
}

// AccessibleLabelOverrider optionally provides an application-supplied
// accessible label that takes precedence over [Accessible.AccessibleLabel].
// It is the plumbing behind widget.BaseWidget.SetAccessibilityLabel: widgets
// without intrinsic text (icon-only buttons, images) return an empty label
// from AccessibleLabel, and the application names them explicitly.
type AccessibleLabelOverrider interface {
	// AccessibilityLabel returns the application-supplied label, or an
	// empty string when no override is set.
	AccessibilityLabel() string
}

// AccessibleDescriber optionally provides a longer description beyond
// the label. Useful for icon-only buttons or complex widgets where the
// label alone is insufficient.
type AccessibleDescriber interface {
	// AccessibleDescription returns help text describing the widget's purpose.
	AccessibleDescription() string
}

// AccessibleValuer reports the current value and valid range of a widget.
// Implemented by sliders, spin boxes, progress bars, and similar
// range-based widgets.
type AccessibleValuer interface {
	// AccessibleValue returns a human-readable representation of the
	// current value (e.g. "42", "50%").
	AccessibleValue() string

	// AccessibleRange returns the minimum, maximum, and current numeric
	// values. Returns (0, 0, 0) if a numeric range is not applicable.
	//
	// The sentinel means a widget whose range is genuinely 0..0 with a
	// current value of 0 (e.g. a slider constructed with min == max == 0)
	// is reported as having no range; such a range is degenerate — there
	// is nothing for assistive technology to adjust — so this is the
	// intended behavior.
	AccessibleRange() (min, max, current float64)
}

// AccessibleStateful reports dynamic boolean states such as "checked",
// "expanded", or "selected". Platform bridges use this to communicate
// widget state to assistive technologies.
type AccessibleStateful interface {
	// AccessibleState returns the current state flags.
	// Only flags relevant to the widget's role are meaningful.
	AccessibleState() AccessibleState
}

// AccessibleActioner reports supported actions and allows invoking them
// by name. Platform bridges translate these to platform-specific action
// protocols (NSAccessibilityAction, ATK_ACTION, UIA Invoke).
type AccessibleActioner interface {
	// AccessibleActions returns the list of actions this widget supports.
	AccessibleActions() []AccessibleAction

	// PerformAccessibleAction invokes the named action.
	// Unknown or unsupported names are silently ignored.
	PerformAccessibleAction(name string)
}

// AccessibleTexter exposes the text content, caret and selection of a text
// widget to assistive technologies, and lets them edit it. Platform bridges
// map this to their text protocols (the NSAccessibility text attributes, the
// UIA Text pattern); without it a screen reader can read a text field's value
// but cannot track the caret, read the selection or edit by voice.
//
// All offsets are rune indices into [AccessibleTexter.AccessibleText];
// bridges convert to their platform's code-unit space. Mutating calls go
// through the widget's normal gated, undo-recorded edit paths and are no-ops
// on read-only or disabled widgets.
type AccessibleTexter interface {
	// AccessibleText returns the text assistive technology reads and
	// navigates. Password fields return a masked representation of the
	// same rune length, never the secret.
	AccessibleText() string

	// AccessibleSelection returns the selection as a half-open rune range
	// with start <= end; when nothing is selected both are the caret offset.
	AccessibleSelection() (start, end int)

	// AccessibleCaret returns the rune offset of the insertion point.
	AccessibleCaret() int

	// SetAccessibleSelection selects [start, end), clamped to the content;
	// equal offsets place the caret there with no selection.
	SetAccessibleSelection(start, end int)

	// ReplaceAccessibleSelection replaces the current selection with text —
	// or inserts it at the caret when nothing is selected — as one
	// undo-recorded edit.
	ReplaceAccessibleSelection(text string)
}

// AccessibleTextLayouter is an optional extension of [AccessibleTexter] for
// text widgets that can expose their visual layout: soft-wrapped (visual)
// line boundaries and the on-screen geometry of text ranges (pre-rebrand tracker #990). Platform
// bridges use it for line-granularity navigation that matches what the user
// sees (an [AccessibleTexter] alone yields logical, newline-delimited lines)
// and for the range-geometry queries behind screen-reader magnifiers and
// cursor tracking (AXBoundsForRange, the UIA GetBoundingRectangles, Android's
// character-location extra). Widgets without it degrade to logical lines and
// element-level geometry.
//
// All offsets are rune indices into [AccessibleTexter.AccessibleText], and
// all geometry is in the widget's own coordinate space adjusted for its
// current scroll offset — the same space as [event.PreeditPositioner]'s
// caret rectangle — so bridges translate with the widget's absolute canvas
// position exactly as they do for the IME caret.
type AccessibleTextLayouter interface {
	AccessibleTexter

	// AccessibleLineCount returns the number of visual (soft-wrapped) lines.
	AccessibleLineCount() int

	// AccessibleLineForOffset returns the zero-based visual line containing
	// the rune offset, clamped to the content.
	AccessibleLineForOffset(off int) int

	// AccessibleLineRange returns the half-open rune range [start, end) of
	// the zero-based visual line: end is the start of the following line —
	// covering a terminating newline or the soft-wrap boundary — or the
	// content length on the last line. ok is false for a line out of range.
	AccessibleLineRange(line int) (start, end int, ok bool)

	// AccessibleRangeBounds returns one rectangle per visual line touched by
	// the half-open rune range [start, end), clipped to the widget's visible
	// viewport: lines scrolled out of view are omitted, so the result stays
	// bounded by the viewport no matter how large the range. A degenerate
	// range (start == end) yields the caret rectangle at that offset. An
	// empty slice means no part of the range is currently visible, or
	// geometry is unavailable. On bidirectional rows the per-line rectangle
	// spans the range's endpoints and may over- or under-cover reordered runs.
	AccessibleRangeBounds(start, end int) []geo.Rectangle
}

// AccessibleConcealed is implemented by widgets whose content is sensitive and
// must be excluded wholesale from semantic auto-capture (and similar
// introspection) — a password Entry, or any field showing concealed text.
// Auto-capture (analytics.EnableAutoCapture) skips a target that reports true,
// belt-and-suspenders on top of never capturing values in the first place.
//
// It is distinct from [RolePasswordField]: a masked-role widget still exposes a
// role and label, whereas a concealed one opts out of capture entirely.
type AccessibleConcealed interface {
	// AccessibleConcealed reports whether the widget currently conceals its
	// content (e.g. a password field). When true, semantic auto-capture omits
	// the widget.
	AccessibleConcealed() bool
}

// AccessibleGridItem reports the coordinates of a cell within its table or
// grid. Platform bridges map this to coordinate-aware patterns
// (NSAccessibility row/column indexes, UIA GridItem).
type AccessibleGridItem interface {
	// AccessibleCellPosition returns the cell's zero-based row and column in
	// the grid's current presentation order — the position as the user
	// perceives it, after any sort, filter or scroll. It is a display
	// coordinate, not a stable source/model index: a re-sort or re-filter
	// changes it, mirroring the on-screen layout that platform grid patterns
	// (UIA GridItem Row/Column, NSAccessibility AXRow order) expect. Stable
	// data identity — for letting assistive-tech focus follow a record across
	// reindexing — travels on a separate channel ([AccessibleVirtualNode.ID]
	// for virtualized rows, [AccessibleIdentified] for realized cells), never
	// on this coordinate.
	AccessibleCellPosition() (row, col int)
}

// AccessibleGridDimensioner reports the dimensions of a table or grid
// container. Virtualized tables realize only the visible cells, so without
// this the full extent is invisible to assistive technologies — platform
// bridges map it to dimension-aware patterns (the UIA Grid pattern's
// RowCount/ColumnCount).
type AccessibleGridDimensioner interface {
	// AccessibleGridDimensions returns the row and column count of the grid
	// as currently presented — including rows that are scrolled off-screen or
	// otherwise not realized, but reflecting the current sort and filter. A
	// filtered grid reports the filtered length, not the unfiltered source
	// total, so it matches the "row R of N" a screen reader announces against
	// the AccessibleCellPosition display coordinates.
	AccessibleGridDimensions() (rows, cols int)
}

// AccessibleVirtualCellRealizer lets assistive technology reach a grid or list
// cell that is NOT in the current viewport window. A virtualized container
// realizes only the visible cells and reports the full extent via
// [AccessibleGridDimensioner]; [AccessibleVirtualNodeProvider] surfaces only the
// near-viewport window so the tree walk stays O(viewport). This interface answers
// a targeted request for one off-window cell the screen reader navigated to — its
// table navigation reads a cell the user has not scrolled to — without the
// container materializing every logical row.
//
// The cell is delivered to the platform bridge on demand and becomes a transient
// element there; it never flows through the bounds-aware tree walk, so an
// off-screen realized cell is not culled the way an off-screen node emitted by
// [AccessibleVirtualNodeProvider] would be. Platform bridges map this to the lazy
// item patterns: the UIA ItemContainer pattern (FindItemByProperty) and
// NSAccessibility's accessibilityCell(forColumn:row:).
type AccessibleVirtualCellRealizer interface {
	// AccessibleRealizeCell returns the semantic node for the cell at the given
	// zero-based (row, col) in the grid's current presentation order — the same
	// display coordinate space as [AccessibleGridItem.AccessibleCellPosition] and
	// [AccessibleGridDimensioner.AccessibleGridDimensions], filter- and
	// sort-aware. ok is false when the coordinates are out of range.
	//
	// The returned node may have off-screen bounds; the bridge positions the AT
	// cursor on it without requiring visibility. Set [AccessibleVirtualNode.ID] to
	// the cell's stable data identity so the element tracks the record across
	// reindexing, and the embedded [AccessibleNode]'s HasCellPosition / CellRow /
	// CellCol so the bridge places it in the grid. A read must have no visible
	// side effect — in particular it must not scroll; the explicit bring-into-view
	// path is [AccessibleScrollIntoView].
	AccessibleRealizeCell(row, col int) (AccessibleVirtualNode, bool)
}

// AccessibleScrollIntoView is implemented by a virtualized grid or list to bring
// a not-currently-realized cell into the visible viewport when assistive
// technology explicitly realizes it — the UIA VirtualizedItem pattern's Realize,
// or a VoiceOver activate on an off-screen cell. It is deliberately separate from
// the read path ([AccessibleVirtualCellRealizer.AccessibleRealizeCell]) so that
// merely announcing an off-screen row as the AT cursor passes over it does not
// move the viewport; only an explicit realize/activate scrolls.
type AccessibleScrollIntoView interface {
	// AccessibleScrollCellIntoView scrolls the container so the cell at the
	// zero-based display (row, col) — the same coordinate space as
	// [AccessibleGridDimensioner] — becomes visible and realized. Out-of-range
	// coordinates are ignored. Called on the main goroutine, like other
	// accessible actions.
	AccessibleScrollCellIntoView(row, col int)
}

// AccessibleLeveler reports the nesting depth of an item in a hierarchy,
// such as a tree node. Rendered trees lay rows out flat, so without this
// the outline structure is invisible to assistive technologies.
type AccessibleLeveler interface {
	// AccessibleLevel returns the 1-based nesting level (a root item is 1).
	AccessibleLevel() int
}

// AccessibleIdentified gives a node a stable data identity independent of its
// CanvasObject. Platform bridges normally reconcile a realized node by its
// object pointer, but a virtualized collection (a table/list with a million
// logical rows) recycles a small pool of cell objects across the data, so the
// same object represents a different record after a sort, scroll or filter.
// A node that reports an identity is reconciled by that key instead, so its
// platform element — and the assistive-tech cursor on it — follows the record
// rather than the recycled screen slot.
//
// This is the realized-node counterpart of [AccessibleVirtualNode.ID] (which
// carries the identity of a node that has no CanvasObject at all); both feed
// the same AccessibleNode.NodeID identity channel.
type AccessibleIdentified interface {
	// AccessibleIdentity returns a stable, COMPARABLE key for the logical item
	// this node represents (it is used as a map key; a non-comparable value
	// panics). It must be unique within the window: incorporate an owner
	// reference (e.g. the owning widget) if a bare index could collide with
	// another widget's. Return nil to fall back to CanvasObject identity.
	AccessibleIdentity() any
}

// AccessibleActiveDescendant is implemented by a focusable container that
// manages an internal keyboard cursor over descendant items — a data grid
// moving a cell cursor with the arrow keys, a tree or virtualized list moving a
// row cursor. The container is the single [event.Focusable] that receives the
// key events, but assistive technology should follow and announce the item the
// cursor is on, not the container as a whole. This is the ARIA
// aria-activedescendant analog.
//
// It designates the AT-focused descendant by stable identity — the
// [AccessibleIdentity] key of a realized descendant, or the
// [AccessibleVirtualNode.ID] of a virtual one (both ride the shared
// AccessibleNode.NodeID channel). Platform bridges resolve AT focus to the
// descendant whose NodeID matches this key and raise a platform focus-change
// event when the cursor moves to a different descendant, so a screen reader
// speaks each item during arrow-key navigation.
//
// This is an AT-only focus designation: it must not change which object
// [Canvas.Focused] reports, since that would reroute real keyboard event
// delivery. The designated descendant must already be a node in the reconciled
// tree — realized, or within a provider's virtual window
// ([AccessibleVirtualNodeProvider]); the container is responsible for scrolling
// its cursor into that window. A descendant that also marks itself
// [StateFocused] reports keyboard focus to the platform even without matching by
// identity.
type AccessibleActiveDescendant interface {
	// AccessibleActiveDescendant returns the stable identity of the currently
	// focused descendant, or nil when the container itself holds AT focus (no
	// internal cursor, or the cursor is on the container as a whole).
	AccessibleActiveDescendant() any
}
