package render

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// headerCellA11y is an invisible accessible node overlaid on one column-header
// cell so assistive tech can read the column title — the header row's visuals
// are raw canvas objects (canvas.Text labels, sort arrows), which publish no
// semantic nodes at all, leaving the column titles unreachable (#109).
//
// Role: fyne core has no RoleColumnHeader (yet), so the node is a RoleHeading —
// the same model group-header rows surface through: on Android/iOS the mobile
// flattener maps it to a native heading, which TalkBack/VoiceOver announce as
// such and include in heading navigation; a RoleTableCell would instead need a
// coordinate in the grid's cell space, where row 0 is already the first data
// row. When the fork grows a dedicated column-header role, this is the one
// place to adopt it.
//
// A sortable column's header also declares the "press" action ("Sort by …"),
// routed through the same OnHeaderTapped path a pointer tap uses, so a
// double-tap activation cycles the sort; the current sort state is carried in
// the accessible description. Not Tappable/Hoverable — the HeaderWidget's own
// pointer handling keeps owning taps, resize drags and hover.
type headerCellA11y struct {
	widget.BaseWidget

	grid     HeaderAccess
	col      int // absolute display column index
	title    string
	sort     SortIndicator
	sortable bool
}

var (
	_ core.Accessible          = (*headerCellA11y)(nil)
	_ core.AccessibleDescriber = (*headerCellA11y)(nil)
	_ core.AccessibleActioner  = (*headerCellA11y)(nil)
)

func newHeaderCellA11y(grid HeaderAccess) *headerCellA11y {
	h := &headerCellA11y{grid: grid}
	h.ExtendBaseWidget(h)
	return h
}

// set records the column's identity and current header state for announcement.
func (h *headerCellA11y) set(col int, title string, sort SortIndicator, sortable bool) {
	h.col = col
	h.title = title
	h.sort = sort
	h.sortable = sortable
}

func (h *headerCellA11y) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	// A transparent rectangle gives the node bounds without drawing anything.
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (h *headerCellA11y) AccessibleRole() core.AccessibleRole { return core.RoleHeading }
func (h *headerCellA11y) AccessibleLabel() string             { return h.title }

// AccessibleDescription identifies the node as a column header and carries the
// column's current sort direction, so a screen reader announces e.g.
// "Name, heading, column header, sorted ascending".
func (h *headerCellA11y) AccessibleDescription() string {
	switch h.sort {
	case SortIndicatorAsc:
		return "Column header, sorted ascending"
	case SortIndicatorDesc:
		return "Column header, sorted descending"
	default:
		return "Column header"
	}
}

// AccessibleActions declares the "press" action for a sortable column, so
// assistive tech's activation gesture (double-tap, Enter) cycles the sort.
// Non-sortable columns declare nothing and read as plain headings.
func (h *headerCellA11y) AccessibleActions() []core.AccessibleAction {
	if !h.sortable {
		return nil
	}
	return []core.AccessibleAction{{Name: core.ActionPress, Description: "Sort by " + h.title}}
}

// PerformAccessibleAction routes the press through the same header-tap path a
// pointer tap uses (plain tap, no modifier: cycle this column's sort).
func (h *headerCellA11y) PerformAccessibleAction(name string) {
	if h.grid == nil || !h.sortable {
		return
	}
	if name == core.ActionPress {
		h.grid.OnHeaderTapped(h.col, 0)
	}
}
