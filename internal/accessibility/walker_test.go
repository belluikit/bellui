//go:build accessibility

package accessibility_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// accWidget is a minimal accessible widget with a fixed role and label whose
// renderer shows the given content, letting tests build trees with exact,
// theme-independent geometry.
type accWidget struct {
	widget.BaseWidget
	label   string
	role    core.AccessibleRole
	content core.CanvasObject
}

func newAccWidget(label string, role core.AccessibleRole, content core.CanvasObject) *accWidget {
	w := &accWidget{label: label, role: role, content: content}
	w.ExtendBaseWidget(w)
	return w
}

func (w *accWidget) AccessibleRole() core.AccessibleRole { return w.role }

func (w *accWidget) AccessibleLabel() string { return w.label }

func (w *accWidget) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(w.content)
}

func labels(nodes []*accessibility.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Label)
	}
	return out
}

func TestBuildTree_LabelsRolesBoundsOrder(t *testing.T) {
	test.NewTempApp(t)

	button := widget.NewButton("OK", nil)
	button.Move(geo.NewPos(10, 20))
	button.Resize(geo.NewSize(100, 30))

	child := newAccWidget("Child", core.RoleLabel, container.NewWithoutLayout())
	child.Move(geo.NewPos(5, 5))
	child.Resize(geo.NewSize(20, 10))
	group := newAccWidget("Group", core.RoleGroup, container.NewWithoutLayout(child))
	group.Move(geo.NewPos(150, 40))
	group.Resize(geo.NewSize(80, 60))

	hidden := widget.NewButton("Hidden", nil)
	hidden.Hide()

	root := container.NewWithoutLayout(button, group, hidden)
	forest := accessibility.BuildTree(root)

	// The plain container contributes no node; the hidden button is pruned.
	require.Len(t, forest, 2)

	ok := forest[0]
	assert.Equal(t, "OK", ok.Label)
	assert.Equal(t, core.RoleButton, ok.Role)
	assert.Equal(t, geo.NewPos(10, 20), ok.Pos)
	assert.Equal(t, geo.NewSize(100, 30), ok.Size)
	assert.False(t, ok.HasClip)
	assert.Empty(t, ok.Children)

	grp := forest[1]
	assert.Equal(t, "Group", grp.Label)
	assert.Equal(t, core.RoleGroup, grp.Role)
	assert.Equal(t, geo.NewPos(150, 40), grp.Pos)
	require.Len(t, grp.Children, 1)

	// The child nests under its closest accessible ancestor and its bounds
	// accumulate every ancestor offset (group position + renderer content).
	nested := grp.Children[0]
	assert.Equal(t, "Child", nested.Label)
	assert.Equal(t, core.RoleLabel, nested.Role)
	assert.Equal(t, geo.NewPos(155, 45), nested.Pos)
	assert.Equal(t, geo.NewSize(20, 10), nested.Size)

	// Flatten preserves tree (reading) order, parents before children.
	assert.Equal(t, []string{"OK", "Group", "Child"}, labels(accessibility.Flatten(forest)))
}

// A button is one semantic unit: the internal RichText its renderer paints
// the label with is accessible too, but must not surface as a child node
// (core.RoleButton.ChildrenPresentational()).
func TestBuildTree_PresentationalChildrenPruned(t *testing.T) {
	test.NewTempApp(t)

	button := widget.NewButton("Once", nil)
	button.Resize(geo.NewSize(100, 30))
	button.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(button))
	require.Len(t, forest, 1)
	assert.Empty(t, forest[0].Children)
	assert.Equal(t, []string{"Once"}, labels(accessibility.Flatten(forest)))
}

// A text input drops the internal widgets painting its value but keeps
// embedded interactive controls reachable (#522): the SearchField's clear
// button and a DateEntry-style ActionItem surface as child nodes, while the
// text-painting RichText stays pruned.
func TestBuildTree_TextInputKeepsInteractiveChildren(t *testing.T) {
	test.NewTempApp(t)

	entry := widget.NewEntry()
	action := widget.NewIconButton(nil, func() {})
	action.SetAccessibilityLabel("Choose date")
	entry.ActionItem = action // before first render, like DateEntry's constructor
	entry.SetText("hello")
	entry.Resize(geo.NewSize(200, 40))
	entry.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(entry))
	require.Len(t, forest, 1)
	root := forest[0]
	assert.Equal(t, core.RoleTextBox, root.Role)

	require.Len(t, root.Children, 1, "only the ActionItem must survive the prune")
	assert.Equal(t, core.RoleButton, root.Children[0].Role)
	assert.Equal(t, "Choose date", root.Children[0].Label)
	assert.Empty(t, root.Children[0].Children, "the button prunes its own internals")
}

func TestBuildTree_SearchFieldClearButtonReachable(t *testing.T) {
	test.NewTempApp(t)

	sf := widget.NewSearchField(nil)
	sf.SetText("query") // clear button becomes visible
	sf.Resize(geo.NewSize(200, 40))
	sf.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(sf))
	require.Len(t, forest, 1)
	root := forest[0]
	assert.Equal(t, core.RoleSearchField, root.Role)

	require.NotEmpty(t, root.Children, "the clear button must be reachable")
	for _, c := range root.Children {
		assert.Equal(t, core.RoleButton, c.Role)
	}
}

// The password reveal control is a labeled, actionable child of a password
// entry (#531).
func TestBuildTree_PasswordRevealerReachable(t *testing.T) {
	test.NewTempApp(t)

	entry := widget.NewPasswordEntry()
	entry.SetText("secret")
	entry.Resize(geo.NewSize(200, 40))
	entry.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(entry))
	require.Len(t, forest, 1)
	root := forest[0]
	assert.Equal(t, core.RolePasswordField, root.Role)

	require.Len(t, root.Children, 1, "exactly the reveal button must survive the prune")
	reveal := root.Children[0]
	assert.Equal(t, core.RoleButton, reveal.Role)
	assert.Equal(t, "Show password", reveal.Label)
	assert.NotEmpty(t, reveal.Actions)
}

// Hyperlinks inside rich text surface as RoleLink nodes (#523): the
// HyperlinkSegment renders a real Hyperlink widget, and RichText's selective
// prune keeps it while dropping the textual segments.
func TestBuildTree_RichTextKeepsHyperlinks(t *testing.T) {
	test.NewTempApp(t)

	rt := widget.NewRichTextFromMarkdown("plain text [bellui](https://bellui.io) more")
	rt.Resize(geo.NewSize(300, 50))
	rt.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(rt))
	require.Len(t, forest, 1)
	root := forest[0]
	assert.Equal(t, core.RoleLabel, root.Role)

	require.Len(t, root.Children, 1)
	link := root.Children[0]
	assert.Equal(t, core.RoleLink, link.Role)
	assert.Equal(t, "bellui", link.Label)

	// A plain label still reports as a single unit.
	label := widget.NewLabel("just text")
	label.Refresh()
	plain := accessibility.BuildTree(container.NewWithoutLayout(label))
	require.Len(t, plain, 1)
	assert.Empty(t, plain[0].Children)
}

// Headings inside rich text surface as virtual RoleHeading nodes with
// levels (#533): they have no CanvasObject of their own, so RichText
// reports them via core.AccessibleVirtualNodeProvider and the walker
// attaches them with absolute bounds.
func TestBuildTree_RichTextHeadingNodes(t *testing.T) {
	test.NewTempApp(t)

	rt := widget.NewRichTextFromMarkdown("# Title\n\nbody text\n\n## Section\n\nmore text")
	rt.Move(geo.NewPos(10, 20))
	rt.Resize(geo.NewSize(300, 400))
	rt.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(rt))
	require.Len(t, forest, 1)
	root := forest[0]
	assert.Equal(t, core.RoleLabel, root.Role)

	var headings []*accessibility.Node
	for _, c := range root.Children {
		if c.Role == core.RoleHeading {
			headings = append(headings, c)
		}
	}
	require.Len(t, headings, 2)

	h1, h2 := headings[0], headings[1]
	assert.Equal(t, "Title", h1.Label)
	assert.Equal(t, 1, h1.Level)
	assert.Nil(t, h1.Object, "virtual nodes are not backed by a CanvasObject")
	assert.NotNil(t, h1.NodeID)

	assert.Equal(t, "Section", h2.Label)
	assert.Equal(t, 2, h2.Level)

	// Bounds are absolute (offset by the RichText position) and non-empty,
	// and the second heading sits below the first.
	assert.GreaterOrEqual(t, h1.Pos.X, float32(10))
	assert.GreaterOrEqual(t, h1.Pos.Y, float32(20))
	assert.Greater(t, h1.Size.Width, float32(0))
	assert.Greater(t, h1.Size.Height, float32(0))
	assert.Greater(t, h2.Pos.Y, h1.Pos.Y)
}

// A document mixing links and headings lists them in document order, not
// all real nodes followed by all virtual ones (#545).
func TestBuildTree_VirtualNodesInReadingOrder(t *testing.T) {
	test.NewTempApp(t)

	rt := widget.NewRichTextFromMarkdown("# Title\n\nsee [bellui](https://bellui.io) for more\n\n## Section\n\ntail")
	rt.Resize(geo.NewSize(300, 400))
	rt.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(rt))
	require.Len(t, forest, 1)

	var order []string
	for _, c := range forest[0].Children {
		order = append(order, c.Label)
	}
	require.Equal(t, []string{"Title", "bellui", "Section"}, order)

	// Sanity: the sort key really is the on-screen position.
	kids := forest[0].Children
	assert.Less(t, kids[0].Pos.Y, kids[1].Pos.Y)
	assert.Less(t, kids[1].Pos.Y, kids[2].Pos.Y)
}

func TestBuildTree_StateValueActions(t *testing.T) {
	test.NewTempApp(t)

	check := widget.NewCheck("Terms", nil)
	check.SetChecked(true)

	forest := accessibility.BuildTree(container.NewWithoutLayout(check))
	require.Len(t, forest, 1)

	node := forest[0]
	assert.Equal(t, core.RoleCheckbox, node.Role)
	assert.True(t, node.State.Has(core.StateChecked))
	assert.NotEmpty(t, node.Actions)
}

func TestBuildTree_ScrollClipBounds(t *testing.T) {
	test.NewTempApp(t)

	button := widget.NewButton("Inside", nil)
	button.Resize(geo.NewSize(200, 400))
	scroll := container.NewScroll(button)
	scroll.Move(geo.NewPos(30, 10))
	scroll.Resize(geo.NewSize(100, 50))

	forest := accessibility.BuildTree(container.NewWithoutLayout(scroll))
	flat := accessibility.Flatten(forest)
	require.NotEmpty(t, flat)

	inside := flat[len(flat)-1]
	assert.Equal(t, "Inside", inside.Label)
	assert.True(t, inside.HasClip)
	assert.Equal(t, geo.NewPos(30, 10), inside.Clip.Pos)
	assert.Equal(t, geo.NewSize(100, 50), inside.Clip.Size)
}

func TestBuildTree_ScrollAncestorStamped(t *testing.T) {
	test.NewTempApp(t)

	inside := widget.NewButton("Inside", nil)
	inside.Resize(geo.NewSize(200, 400))
	scroll := container.NewScroll(inside)
	scroll.Resize(geo.NewSize(100, 50))
	outside := widget.NewButton("Outside", nil)

	forest := accessibility.BuildTree(container.NewWithoutLayout(scroll, outside))
	flat := accessibility.Flatten(forest)

	var in, out *accessibility.Node
	for _, n := range flat {
		switch n.Label {
		case "Inside":
			in = n
		case "Outside":
			out = n
		}
	}
	require.NotNil(t, in)
	require.NotNil(t, out)

	// The button inside the scroll resolves the scroller as its scroll ancestor;
	// the scroll container is not itself an accessible node.
	assert.Same(t, scroll, in.ScrollAncestor)
	// A sibling outside any scroll has none.
	assert.Nil(t, out.ScrollAncestor)
}

func TestCanvasNodes_ContentMenuOverlayOrder(t *testing.T) {
	w := test.NewTempWindow(t, widget.NewButton("Content", nil))
	c := w.Canvas()

	menu := widget.NewMenu(bellui.NewMenu("", bellui.NewMenuItem("Item", nil)))
	c.Overlays().Add(widget.NewButton("Overlay", nil))

	forest := accessibility.CanvasNodes(c, menu)
	require.Len(t, forest, 3)

	assert.Equal(t, "Content", forest[0].Label)
	assert.Equal(t, core.RoleButton, forest[0].Role)
	assert.Equal(t, core.RoleMenu, forest[1].Role)
	assert.Equal(t, "Overlay", forest[2].Label)

	// Without a menu the canvas still yields content before overlays.
	noMenu := accessibility.CanvasNodes(c, nil)
	require.Len(t, noMenu, 2)
	assert.Equal(t, "Content", noMenu[0].Label)
	assert.Equal(t, "Overlay", noMenu[1].Label)
}

// Under RTL the emitted order stays the logical reading order — mirroring
// only moves objects, so bounds reflect the mirrored positions while the
// node order is unchanged. This is the ordering contract documented in the
// package doc.
func TestBuildTree_RTLOrderIsReadingOrder(t *testing.T) {
	test.NewTempApp(t)
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionAuto) })

	first := widget.NewButton("First", nil)
	second := widget.NewButton("Second", nil)
	box := container.NewHBox(first, second)
	box.Resize(geo.NewSize(300, 50))

	flat := accessibility.Flatten(accessibility.BuildTree(box))
	require.Equal(t, []string{"First", "Second"}, labels(flat))
	assert.Greater(t, flat[0].Pos.X, flat[1].Pos.X,
		"RTL mirroring should place the first (reading-order) item to the right")
}

// virtualGrid is a fake virtualized grid: a large logical extent reported via
// AccessibleGridDimensioner, but AccessibleVirtualNodes returns only a bounded
// window of flat RoleTableCell nodes carrying grid coordinates — the model a
// real virtualized table (fytables DataGrid) uses (#630).
type virtualGrid struct {
	widget.BaseWidget
	totalRows, cols    int
	winOffset, winRows int
	scrolledTo         *[2]int // last AccessibleScrollCellIntoView request, nil until scrolled
}

func newVirtualGrid() *virtualGrid {
	g := &virtualGrid{totalRows: 1_000_000, cols: 3, winOffset: 4995, winRows: 10}
	g.ExtendBaseWidget(g)
	return g
}

func (g *virtualGrid) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(""))
}
func (g *virtualGrid) AccessibleRole() core.AccessibleRole        { return core.RoleTable }
func (g *virtualGrid) AccessibleLabel() string                    { return "grid" }
func (g *virtualGrid) AccessibleGridDimensions() (rows, cols int) { return g.totalRows, g.cols }

func (g *virtualGrid) AccessibleVirtualNodes() []core.AccessibleVirtualNode {
	var out []core.AccessibleVirtualNode
	for r := g.winOffset; r < g.winOffset+g.winRows; r++ {
		for c := 0; c < g.cols; c++ {
			out = append(out, core.AccessibleVirtualNode{
				AccessibleNode: core.AccessibleNode{
					Role:            core.RoleTableCell,
					Label:           fmt.Sprintf("r%dc%d", r, c),
					HasCellPosition: true,
					CellRow:         r,
					CellCol:         c,
				},
				Pos:  geo.NewPos(float32(c*100), float32((r-g.winOffset)*30)),
				Size: geo.NewSize(100, 30),
				ID:   [2]int{r, c},
			})
		}
	}
	return out
}

// AccessibleRealizeCell realizes any single cell on demand, including rows far
// outside the AccessibleVirtualNodes window, without materializing the rows
// between — the point-addressed lazy path a screen reader uses to navigate a
// virtualized grid past its viewport (#637). The off-viewport cell reports its
// logical position (row 500000 sits far below the realized window).
func (g *virtualGrid) AccessibleRealizeCell(row, col int) (core.AccessibleVirtualNode, bool) {
	if row < 0 || row >= g.totalRows || col < 0 || col >= g.cols {
		return core.AccessibleVirtualNode{}, false
	}
	return core.AccessibleVirtualNode{
		AccessibleNode: core.AccessibleNode{
			Role:            core.RoleTableCell,
			Label:           fmt.Sprintf("r%dc%d", row, col),
			HasCellPosition: true,
			CellRow:         row,
			CellCol:         col,
		},
		Pos:  geo.NewPos(float32(col*100), float32(row*30)),
		Size: geo.NewSize(100, 30),
		ID:   [2]int{row, col},
	}, true
}

// AccessibleScrollCellIntoView records the bring-into-view request; a real grid
// would scroll its viewport. Out-of-range coordinates are ignored.
func (g *virtualGrid) AccessibleScrollCellIntoView(row, col int) {
	if row < 0 || row >= g.totalRows || col < 0 || col >= g.cols {
		return
	}
	g.scrolledTo = &[2]int{row, col}
}

// Scrolled makes the grid its own scroll handler (event.Scrollable), like a
// widget that pages its virtualized viewport internally.
func (g *virtualGrid) Scrolled(*event.ScrollEvent) {}

// A virtual node has no real ancestors in the walk, so it inherits its scroll
// target from the provider: the provider itself when that handles scroll
// events (#952) — the AT scroll gesture on a virtual cell then pages the grid.
func TestBuildTree_VirtualNodeScrollAncestor(t *testing.T) {
	test.NewTempApp(t)
	g := newVirtualGrid()
	g.Resize(geo.NewSize(300, 300))
	g.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(g))
	require.Len(t, forest, 1)
	var checked int
	for _, c := range forest[0].Children {
		if c.Role != core.RoleTableCell {
			continue
		}
		assert.Same(t, g, c.ScrollAncestor, "virtual cell should scroll its provider")
		checked++
	}
	assert.NotZero(t, checked)
}

// A virtualized grid surfaces a bounded window of virtual cells with grid
// coordinates and a stable identity, while the dimensioner reports the full
// extent — the walker never touches all N rows (#630 windowing).
func TestBuildTree_VirtualizedGridWindow(t *testing.T) {
	test.NewTempApp(t)
	g := newVirtualGrid()
	g.Resize(geo.NewSize(300, 300))
	g.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(g))
	require.Len(t, forest, 1)
	table := forest[0]
	assert.Equal(t, core.RoleTable, table.Role)
	assert.True(t, table.HasGridDims)
	assert.Equal(t, 1_000_000, table.GridRows)

	var cells []*accessibility.Node
	for _, c := range table.Children {
		if c.Role == core.RoleTableCell {
			cells = append(cells, c)
		}
	}
	// Only the window is materialized, not all million rows.
	require.Len(t, cells, g.winRows*g.cols)
	for _, c := range cells {
		assert.True(t, c.HasCellPosition, "virtual cell missing grid position")
		assert.GreaterOrEqual(t, c.CellRow, g.winOffset)
		assert.Less(t, c.CellRow, g.winOffset+g.winRows)
		assert.NotNil(t, c.NodeID, "virtual cell missing stable identity")
	}
}

// identifiedWidget is a realized accessible widget that advertises a stable
// data identity (core.AccessibleIdentified, #635) — the shape a virtualized
// grid's recycled cell host uses so its element tracks the record.
type identifiedWidget struct {
	widget.BaseWidget
	label    string
	identity any
}

func newIdentifiedWidget(label string, identity any) *identifiedWidget {
	w := &identifiedWidget{label: label, identity: identity}
	w.ExtendBaseWidget(w)
	return w
}

func (w *identifiedWidget) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(w.label))
}
func (w *identifiedWidget) AccessibleRole() core.AccessibleRole { return core.RoleTableCell }
func (w *identifiedWidget) AccessibleLabel() string             { return w.label }
func (w *identifiedWidget) AccessibleIdentity() any             { return w.identity }

// A realized node that implements AccessibleIdentified surfaces its stable
// identity as NodeID, so platform bridges key it by record rather than by its
// (recyclable) CanvasObject.
func TestBuildTree_RealizedIdentity(t *testing.T) {
	test.NewTempApp(t)

	cell := newIdentifiedWidget("r5000c2", [2]int{5000, 2})
	cell.Resize(geo.NewSize(100, 30))
	cell.Refresh()

	forest := accessibility.BuildTree(container.NewWithoutLayout(cell))
	require.Len(t, forest, 1)
	node := forest[0]
	assert.Equal(t, core.RoleTableCell, node.Role)
	assert.NotNil(t, node.Object, "the node is backed by a real object")
	assert.Equal(t, [2]int{5000, 2}, node.NodeID, "identity surfaces as NodeID for bridge keying")
}

// A shown CompletionPopup exposes its suggestion rows as RoleListItem virtual
// nodes, and its active descendant matches the selected row's NodeID (#801).
func TestBuildTree_CompletionPopupRows(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewTempWindow(t, canvas.NewRectangle(nil))
	w.Resize(geo.NewSize(300, 200))

	items := []widget.CompletionItem{{Text: "alpha"}, {Text: "beta"}, {Text: "gamma"}}
	p := widget.NewCompletionPopup(w.Canvas(), items)
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	forest := accessibility.BuildTree(p)
	require.Len(t, forest, 1)
	root := forest[0]
	assert.Equal(t, core.RoleList, root.Role)

	var rows []*accessibility.Node
	for _, c := range root.Children {
		if c.Role == core.RoleListItem {
			rows = append(rows, c)
		}
	}
	require.Len(t, rows, 3)
	assert.Equal(t, "alpha", rows[0].Label)
	assert.Nil(t, rows[0].Object, "list-item rows are virtual, no CanvasObject")
	assert.NotNil(t, rows[0].NodeID)

	// The container's active descendant resolves to the selected row's NodeID.
	assert.Equal(t, rows[0].NodeID, p.AccessibleActiveDescendant(),
		"active descendant identity must match the selected row node")
}
