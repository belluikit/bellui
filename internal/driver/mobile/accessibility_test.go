//go:build accessibility

package mobile

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/accessibility"
	"github.com/belluikit/bellui/internal/scale"
	"github.com/belluikit/bellui/widget"
)

func newA11yTestWindow(t *testing.T) *window {
	t.Helper()
	d := &driver{}
	w := d.CreateWindow("a11y").(*window)
	t.Cleanup(func() { mobileA11yForgetWindow(w) })
	return w
}

func TestMobileA11yGather_RolesAndOrder(t *testing.T) {
	w := newA11yTestWindow(t)

	check := widget.NewCheck("Agree", nil)
	check.SetChecked(true)
	slider := widget.NewSlider(0, 10)
	w.SetContent(container.NewVBox(
		widget.NewLabel("Hello"),
		widget.NewButton("Tap", nil),
		check,
		slider,
	))
	w.Resize(geo.NewSize(200, 400))

	nodes, removed := w.mobileA11yGather()
	assert.Empty(t, removed)
	require.Len(t, nodes, 4)

	// Flat list in reading order; the VBox contributes no element of its own.
	assert.Equal(t, mobileRoleText, nodes[0].role)
	assert.Equal(t, "Hello", nodes[0].label)
	assert.Equal(t, mobileRoleButton, nodes[1].role)
	assert.Equal(t, "Tap", nodes[1].label)
	assert.NotZero(t, nodes[1].flags&mobileA11yFlagPressable, "buttons dispatch an explicit activate")
	assert.Equal(t, mobileRoleCheckbox, nodes[2].role)
	assert.NotZero(t, nodes[2].flags&mobileA11yFlagCheckable)
	assert.NotZero(t, nodes[2].flags&mobileA11yFlagChecked)
	assert.Equal(t, mobileRoleSlider, nodes[3].role)
	assert.NotZero(t, nodes[3].flags&mobileA11yFlagAdjustable, "sliders adjust via increment/decrement")

	// Reading order runs top to bottom in a VBox.
	for i := 1; i < len(nodes); i++ {
		assert.Greater(t, nodes[i].y, nodes[i-1].y, "node %d should be below node %d", i, i-1)
	}
}

func TestMobileA11yGather_HandleStabilityAndRemoval(t *testing.T) {
	w := newA11yTestWindow(t)

	btn := widget.NewButton("Stable", nil)
	w.SetContent(container.NewVBox(btn, widget.NewLabel("Text")))
	w.Resize(geo.NewSize(200, 200))

	first, _ := w.mobileA11yGather()
	require.Len(t, first, 2)
	second, removed := w.mobileA11yGather()
	require.Len(t, second, 2)
	assert.Empty(t, removed)
	assert.Equal(t, first[0].handle, second[0].handle, "unchanged widgets keep their handle")
	assert.Equal(t, first[1].handle, second[1].handle)

	// The handle resolves to a dispatch target for the action callbacks.
	target, ok := mobileA11yLookup(first[0].handle)
	require.True(t, ok)
	assert.Same(t, w, target.win)

	w.SetContent(widget.NewLabel("Replaced"))
	w.Resize(geo.NewSize(200, 200))
	third, removed := w.mobileA11yGather()
	require.Len(t, third, 1)
	assert.Len(t, removed, 2, "both old elements leave the tree")
	_, ok = mobileA11yLookup(first[0].handle)
	assert.False(t, ok, "stale handles are pruned from the registry")
}

func TestMobileA11yGather_DisabledFlag(t *testing.T) {
	w := newA11yTestWindow(t)

	check := widget.NewCheck("Off", nil)
	check.Disable()
	w.SetContent(container.NewVBox(check))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 1)
	assert.NotZero(t, nodes[0].flags&mobileA11yFlagDisabled)
}

func TestMobileA11yGather_EmptyTextDropped(t *testing.T) {
	w := newA11yTestWindow(t)

	w.SetContent(container.NewVBox(widget.NewLabel(""), widget.NewLabel("Kept")))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 1)
	assert.Equal(t, "Kept", nodes[0].label)
}

func TestMobileA11yGather_CullsScrolledOutNodes(t *testing.T) {
	w := newA11yTestWindow(t)

	var labels []string
	box := container.NewVBox()
	for i := 0; i < 50; i++ {
		text := fmt.Sprintf("Row %d", i)
		labels = append(labels, text)
		box.Add(widget.NewLabel(text))
	}
	w.SetContent(container.NewScroll(box))
	w.Resize(geo.NewSize(200, 120))

	nodes, _ := w.mobileA11yGather()
	require.NotEmpty(t, nodes)
	assert.Less(t, len(nodes), len(labels), "rows scrolled out of the viewport are culled")
	assert.Equal(t, "Row 0", nodes[0].label)
}

func TestMobileA11yGather_ScrollableFlag(t *testing.T) {
	w := newA11yTestWindow(t)

	tall := widget.NewButton("Inside", nil)
	tall.Resize(geo.NewSize(200, 800))
	scroll := container.NewScroll(tall)
	outside := widget.NewButton("Outside", nil)
	w.SetContent(container.NewVBox(scroll, outside))
	w.Resize(geo.NewSize(200, 300))

	nodes, _ := w.mobileA11yGather()
	var inside, out *mobileA11yNode
	for i := range nodes {
		switch nodes[i].label {
		case "Inside":
			inside = &nodes[i]
		case "Outside":
			out = &nodes[i]
		}
	}
	require.NotNil(t, inside)
	require.NotNil(t, out)
	assert.NotZero(t, inside.flags&mobileA11yFlagScrollable, "a node inside a scroll exposes the scroll gesture")
	assert.Zero(t, out.flags&mobileA11yFlagScrollable, "a node outside any scroll does not")

	// The dispatch target carries the resolved scroller for scroll routing.
	target, ok := mobileA11yLookup(inside.handle)
	require.True(t, ok)
	assert.Same(t, scroll, target.node.ScrollAncestor)
}

// fakeScrollable records the scroll events routed to it.
type fakeScrollable struct {
	widget.BaseWidget
	events []*event.ScrollEvent
}

func (f *fakeScrollable) Scrolled(e *event.ScrollEvent) { f.events = append(f.events, e) }

func TestMobileA11yScroll_RoutesPagedScroll(t *testing.T) {
	sc := &fakeScrollable{}
	sc.Resize(geo.NewSize(100, 200))
	n := &accessibility.Node{Pos: geo.NewPos(0, 0), Size: geo.NewSize(100, 200), ScrollAncestor: sc}

	mobileA11yScroll(n, true)  // forward
	mobileA11yScroll(n, false) // backward
	require.Len(t, sc.events, 2)
	// Forward reveals content below → negative DY; one page ≈ 0.8 × viewport.
	assert.InDelta(t, -160, sc.events[0].Scrolled.DY, 0.01)
	assert.InDelta(t, 160, sc.events[1].Scrolled.DY, 0.01)
	assert.Equal(t, event.ScrollSourceTouch, sc.events[0].Source)
}

// fakeLongPressable is an accessible object that records long-press + secondary.
type fakeLongPressable struct {
	widget.BaseWidget
	longPresses int
}

func (f *fakeLongPressable) LongPressed(*event.PointEvent) { f.longPresses++ }

func TestMobileA11yLongPress_RoutesToLongPressable(t *testing.T) {
	lp := &fakeLongPressable{}
	n := &accessibility.Node{Pos: geo.NewPos(10, 20), Size: geo.NewSize(40, 40), AccessibleNode: core.AccessibleNode{Object: lp}}

	mobileA11yLongPress(n)
	assert.Equal(t, 1, lp.longPresses)
	assert.True(t, a11yIsLongPressable(lp))
}

func TestMobileA11yFocusRing_ResolvesFocusedBounds(t *testing.T) {
	w := newA11yTestWindow(t)
	btn := widget.NewButton("Focusable", nil)
	w.SetContent(container.NewVBox(btn))
	w.Resize(geo.NewSize(200, 200))
	nodes, _ := w.mobileA11yGather()
	require.NotEmpty(t, nodes)
	t.Cleanup(func() { a11yFocusedHandle.Store(0) })

	// No focus → no ring.
	a11yFocusedHandle.Store(0)
	_, ok := w.a11yFocusRing()
	assert.False(t, ok)

	// Focused element → ring at its canvas-space bounds.
	h := nodes[0].handle
	a11yFocusedHandle.Store(h)
	ring, ok := w.a11yFocusRing()
	require.True(t, ok)
	assert.Positive(t, ring.Size.Width)
	assert.Positive(t, ring.Size.Height)

	// A handle that left the tree → no ring.
	a11yFocusedHandle.Store(99999)
	_, ok = w.a11yFocusRing()
	assert.False(t, ok)
}

func TestMobileA11ySetFocus_ClearsOnlyCurrentHandle(t *testing.T) {
	t.Cleanup(func() { a11yFocusedHandle.Store(0) })

	mobileA11ySetFocus(5, true)
	assert.Equal(t, uintptr(5), a11yFocusedHandle.Load())

	// A stale focus-cleared for a different (already-superseded) element must
	// not wipe the current focus.
	mobileA11ySetFocus(3, false)
	assert.Equal(t, uintptr(5), a11yFocusedHandle.Load())

	// Clearing the current element clears the ring.
	mobileA11ySetFocus(5, false)
	assert.Equal(t, uintptr(0), a11yFocusedHandle.Load())
}

// passThroughTestOverlay is a minimal input-pass-through overlay (Toast/Snackbar
// analog) for the modal-inert test: it reports PassesInputThrough so it is not a
// modal barrier, and paints a labelled child so the gather can find it.
type passThroughTestOverlay struct {
	widget.BaseWidget
	text string
}

func (o *passThroughTestOverlay) PassesInputThrough() bool { return true }

func (o *passThroughTestOverlay) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(widget.NewLabel(o.text))
}

func a11yNodeLabels(nodes []mobileA11yNode) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.label)
	}
	return out
}

func TestMobileA11yGather_ModalOverlayMakesContentInert(t *testing.T) {
	w := newA11yTestWindow(t)
	w.SetContent(container.NewVBox(widget.NewButton("Background", nil)))
	w.Resize(geo.NewSize(300, 500))

	// Baseline: with no overlay, the background is reachable.
	nodes, _ := w.mobileA11yGather()
	assert.Contains(t, a11yNodeLabels(nodes), "Background")

	// A modal pop-up makes the background inert: only its own content is exposed.
	pop := widget.NewModalPopUp(widget.NewButton("Dialog OK", nil), w.Canvas())
	pop.Show()
	w.Resize(geo.NewSize(300, 500)) // lay the pop-up content out
	nodes, _ = w.mobileA11yGather()
	labels := a11yNodeLabels(nodes)
	assert.Contains(t, labels, "Dialog OK")
	assert.NotContains(t, labels, "Background", "modal overlay makes background inert (#807)")
	w.Canvas().Overlays().Remove(pop)

	// A pass-through overlay (Toast/Snackbar) is not a barrier: the background
	// stays reachable and the overlay is exposed too.
	over := &passThroughTestOverlay{text: "Toast"}
	over.ExtendBaseWidget(over)
	w.Canvas().Overlays().Add(over)
	over.Resize(w.Canvas().Size())
	nodes, _ = w.mobileA11yGather()
	labels = a11yNodeLabels(nodes)
	assert.Contains(t, labels, "Background", "pass-through overlay keeps the background reachable")
	assert.Contains(t, labels, "Toast")
}

func TestMobileA11yGather_ClipsOverscanRowBoundsToViewport(t *testing.T) {
	w := newA11yTestWindow(t)

	box := container.NewVBox()
	for i := 0; i < 30; i++ {
		box.Add(widget.NewLabel(fmt.Sprintf("Row %d", i)))
	}
	w.SetContent(container.NewScroll(box))
	w.Resize(geo.NewSize(200, 100)) // short viewport → a row straddles the bottom

	nodes, _ := w.mobileA11yGather()
	require.NotEmpty(t, nodes)

	// No emitted row extends past the scroll viewport: an overscan row realized
	// just past the fold has its focus bounds clipped to the visible area, so the
	// screen reader and the in-scene ring never land under the system nav bar
	// (#808). The interactive area is the full canvas in the test harness, so the
	// viewport bottom is the canvas height.
	viewportBottom := scale.ToScreenCoordinate(w.canvas, 100)
	for _, n := range nodes {
		assert.LessOrEqual(t, n.y+n.h, viewportBottom,
			"row %q bounds must be clipped to the viewport", n.label)
	}
}

// The editable-text wire fields (#974 option D): a text-input node backed by
// core.AccessibleTexter publishes its live document and rune selection, and
// the TextEditable flag gates the mutating protocol.
func TestMobileA11yGather_EditableTextDocument(t *testing.T) {
	w := newA11yTestWindow(t)

	entry := widget.NewEntry()
	entry.SetText("héllo wörld")
	w.SetContent(container.NewVBox(entry, widget.NewLabel("plain")))
	w.Resize(geo.NewSize(200, 200))
	entry.SetAccessibleSelection(2, 7)

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 2)

	require.Equal(t, mobileRoleTextField, nodes[0].role)
	assert.Equal(t, "héllo wörld", nodes[0].text)
	assert.Equal(t, 2, nodes[0].selStart)
	assert.Equal(t, 7, nodes[0].selEnd)
	assert.NotZero(t, nodes[0].flags&mobileA11yFlagTextEditable)

	// A non-texter node carries no document.
	assert.Equal(t, "", nodes[1].text)
	assert.Equal(t, -1, nodes[1].selStart)
	assert.Equal(t, -1, nodes[1].selEnd)
	assert.Zero(t, nodes[1].flags&mobileA11yFlagTextEditable)
}

func TestMobileA11yGather_ReadOnlyEntryKeepsDocumentNotEditable(t *testing.T) {
	w := newA11yTestWindow(t)

	entry := widget.NewEntry()
	entry.SetText("fixed")
	entry.ReadOnly = true
	w.SetContent(container.NewVBox(entry))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 1)
	assert.Equal(t, "fixed", nodes[0].text, "read-only editors keep the document for select/read")
	assert.GreaterOrEqual(t, nodes[0].selStart, 0)
	assert.Zero(t, nodes[0].flags&mobileA11yFlagTextEditable, "read-only editors do not offer edits")
}

func TestMobileA11yGather_PasswordDocumentIsMask(t *testing.T) {
	w := newA11yTestWindow(t)

	entry := widget.NewPasswordEntry()
	entry.SetText("sécret")
	w.SetContent(container.NewVBox(entry))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.NotEmpty(t, nodes) // the reveal ActionItem contributes a second node
	require.Equal(t, mobileRolePassword, nodes[0].role)
	assert.NotContains(t, nodes[0].text, "s", "the secret never crosses the wire")
	assert.Len(t, []rune(nodes[0].text), 6, "length-preserving mask so offsets line up")
	assert.NotZero(t, nodes[0].flags&mobileA11yFlagTextEditable)
}

// The text-edit dispatch lands on the editor's own gated paths via the
// internal/textinput primitives, exactly like the desktop backends (#974).
func TestMobileA11yTextDispatch_EditsThroughSeam(t *testing.T) {
	w := newA11yTestWindow(t)

	entry := widget.NewEntry()
	entry.SetText("hello world")
	w.SetContent(container.NewVBox(entry))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 1)
	h := nodes[0].handle

	mobileA11yTextDispatchOnMain(h, mobileA11yTextActSetSelection, 0, 5, "")
	start, end := entry.AccessibleSelection()
	assert.Equal(t, 0, start)
	assert.Equal(t, 5, end)

	mobileA11yTextDispatchOnMain(h, mobileA11yTextActInsert, 0, 0, "goodbye")
	assert.Equal(t, "goodbye world", entry.Text, "insert replaces the live selection")

	mobileA11yTextDispatchOnMain(h, mobileA11yTextActReplace, 8, 13, "moon")
	assert.Equal(t, "goodbye moon", entry.Text)

	mobileA11yTextDispatchOnMain(h, mobileA11yTextActSetText, 0, 0, "reset")
	assert.Equal(t, "reset", entry.Text)

	// Unknown handles and non-texter targets are ignored, never a panic.
	mobileA11yTextDispatchOnMain(99999, mobileA11yTextActSetText, 0, 0, "x")
}

// Undo/redo travel as custom actions (#1025): declared once history exists,
// carried on the wire node as stable (registry id, label) pairs, and
// dispatched back through the custom kind block onto the widget's own
// undo/redo paths.
func TestMobileA11yGather_CustomActionsUndoRedo(t *testing.T) {
	w := newA11yTestWindow(t)

	entry := widget.NewEntry()
	w.SetContent(container.NewVBox(entry))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 1)
	assert.Empty(t, nodes[0].customActs, "no history, no custom actions")

	entry.InsertText("hello")
	nodes, _ = w.mobileA11yGather()
	require.Len(t, nodes, 1)
	require.Len(t, nodes[0].customActs, 1)
	undoAct := nodes[0].customActs[0]
	name, ok := mobileA11yCustomActionName(undoAct.id)
	require.True(t, ok)
	assert.Equal(t, core.ActionUndo, name)
	assert.NotEmpty(t, undoAct.label, "the localized label rides the wire")

	// Dispatching the custom kind performs the widget's own undo.
	mobileA11yDispatchOnMain(nodes[0].handle, mobileA11yActionCustom+undoAct.id)
	assert.Equal(t, "", entry.Text)

	// The registry id is stable and the declaration now flips to redo.
	nodes, _ = w.mobileA11yGather()
	require.Len(t, nodes[0].customActs, 1)
	name, ok = mobileA11yCustomActionName(nodes[0].customActs[0].id)
	require.True(t, ok)
	assert.Equal(t, core.ActionRedo, name)

	mobileA11yDispatchOnMain(nodes[0].handle, mobileA11yActionCustom+nodes[0].customActs[0].id)
	assert.Equal(t, "hello", entry.Text)

	// A custom kind with no registered name — or one the node no longer
	// declares — is a no-op, never a panic or a misfire.
	mobileA11yDispatchOnMain(nodes[0].handle, mobileA11yActionCustom+9999)
	assert.Equal(t, "hello", entry.Text)
}

// The iOS bridge's per-frame diff must see every wire-node field, including
// the customActs slice that broke plain == comparability (#1036/#1027).
func TestMobileA11yNodesEqual(t *testing.T) {
	base := mobileA11yNode{
		handle: 7, role: mobileRoleTextField, flags: 3,
		label: "l", value: "v", hint: "h",
		x: 1, y: 2, w: 3, h: 4,
		cellRow: -1, cellCol: -1, selStart: 5, selEnd: 6, text: "doc",
		customActs: []mobileA11yCustomAct{{id: 0, label: "Undo"}},
	}
	same := base
	same.customActs = []mobileA11yCustomAct{{id: 0, label: "Undo"}}
	assert.True(t, mobileA11yNodesEqual(base, same), "distinct but equal slices compare equal")

	diffActs := base
	diffActs.customActs = []mobileA11yCustomAct{{id: 1, label: "Redo"}}
	assert.False(t, mobileA11yNodesEqual(base, diffActs))

	noActs := base
	noActs.customActs = nil
	assert.False(t, mobileA11yNodesEqual(base, noActs))

	diffText := base
	diffText.text = "changed"
	assert.False(t, mobileA11yNodesEqual(base, diffText))

	diffSel := base
	diffSel.selEnd = 9
	assert.False(t, mobileA11yNodesEqual(base, diffSel))
}

func TestA11yEncodeCustomActs(t *testing.T) {
	assert.Equal(t, "", a11yEncodeCustomActs(nil))
	got := a11yEncodeCustomActs([]mobileA11yCustomAct{
		{id: 3, label: "Undo"},
		{id: 4, label: "Redo"},
	})
	assert.Equal(t, "3\x1fUndo\x1e4\x1fRedo", got)
}

// fakeTextLayouter is a minimal core.AccessibleTextLayouter over fixed visual
// line ranges: line i covers runes [starts[i], starts[i+1]) of text. Range
// bounds are synthetic — 10dp per rune, 20dp per line — except offsets in
// offscreen, which report no geometry (scrolled out of the viewport).
type fakeTextLayouter struct {
	starts    []int // line start offsets plus the final end boundary
	offscreen map[int]bool
}

func (f *fakeTextLayouter) AccessibleLineCount() int { return len(f.starts) - 1 }

func (f *fakeTextLayouter) AccessibleLineForOffset(off int) int {
	line := 0
	for line+1 < f.AccessibleLineCount() && f.starts[line+1] <= off {
		line++
	}
	return line
}

func (f *fakeTextLayouter) AccessibleLineRange(line int) (int, int, bool) {
	if line < 0 || line >= f.AccessibleLineCount() {
		return 0, 0, false
	}
	return f.starts[line], f.starts[line+1], true
}

func (f *fakeTextLayouter) AccessibleRangeBounds(start, end int) []geo.Rectangle {
	if f.offscreen[start] {
		return nil
	}
	line := f.AccessibleLineForOffset(start)
	col := start - f.starts[line]
	return []geo.Rectangle{{
		Pos:  geo.NewPos(float32(col*10), float32(line*20)),
		Size: geo.NewSize(float32((end-start)*10), 20),
	}}
}

func TestMobileA11yTextLayout(t *testing.T) {
	idPx := func(v float32) int { return int(v) }

	// Two visual lines: "abc" / "de". Caret at end.
	lay := &fakeTextLayouter{starts: []int{0, 3, 5}}
	blob := mobileA11yTextLayout(lay, "abcde", 5, 5, geo.NewPos(100, 200), idPx)
	require.NotEmpty(t, blob)
	// Line boundaries: starts 0, 3 plus the end of the last line (5).
	assert.Contains(t, blob, "L:0,3,5")
	// Char rects: window covers the whole doc (selection line +/- 1); rects
	// are node-origin translated (100, 200).
	assert.Contains(t, blob, "\nC:0:")
	assert.Contains(t, blob, "100,200,10,20") // 'a' at col 0 line 0
	assert.Contains(t, blob, "110,200,10,20") // 'b'
	assert.Contains(t, blob, "100,220,10,20") // 'd' at col 0 line 1

	// A surrogate-pair rune repeats its rect for both UTF-16 units, and the
	// line boundaries are UTF-16 offsets: "x😀y" is one visual line of 3
	// runes = 4 UTF-16 units.
	lay = &fakeTextLayouter{starts: []int{0, 3}}
	blob = mobileA11yTextLayout(lay, "x😀y", 0, 0, geo.Position{}, idPx)
	assert.Contains(t, blob, "L:0,4")
	body := blob[strings.Index(blob, "\nC:0:")+len("\nC:0:"):]
	entries := strings.Split(body, ";")
	require.Len(t, entries, 4, "4 UTF-16 units")
	assert.Equal(t, entries[1], entries[2], "both surrogate units carry the same rect")

	// Offscreen runes encode as '-' (no geometry).
	lay = &fakeTextLayouter{starts: []int{0, 3}, offscreen: map[int]bool{1: true}}
	blob = mobileA11yTextLayout(lay, "abc", 0, 0, geo.Position{}, idPx)
	assert.Contains(t, blob, ";-;")

	// Empty layout yields no sidecar.
	lay = &fakeTextLayouter{starts: []int{0}}
	assert.Empty(t, mobileA11yTextLayout(lay, "", 0, 0, geo.Position{}, idPx))
}

// groupHeaderStub mirrors a virtualized grid's group-header row (fytables'
// groupRowA11y): a labeled RoleTableRow carrying an expand/collapse action,
// painting nothing itself.
type groupHeaderStub struct {
	widget.BaseWidget
	expanded bool
}

func newGroupHeaderStub() *groupHeaderStub {
	g := &groupHeaderStub{}
	g.ExtendBaseWidget(g)
	return g
}

func (g *groupHeaderStub) CreateRenderer() core.WidgetRenderer {
	g.ExtendBaseWidget(g)
	rect := belluicanvas.NewRectangle(color.Transparent)
	rect.SetMinSize(geo.NewSize(50, 20))
	return widget.NewSimpleRenderer(rect)
}

func (g *groupHeaderStub) AccessibleRole() core.AccessibleRole { return core.RoleTableRow }
func (g *groupHeaderStub) AccessibleLabel() string             { return "West (125)" }

func (g *groupHeaderStub) AccessibleState() core.AccessibleState {
	if g.expanded {
		return core.StateExpanded
	}
	return 0
}

func (g *groupHeaderStub) AccessibleActions() []core.AccessibleAction {
	if g.expanded {
		return []core.AccessibleAction{{Name: core.ActionCollapse, Description: "Collapse group"}}
	}
	return []core.AccessibleAction{{Name: core.ActionExpand, Description: "Expand group"}}
}

func (g *groupHeaderStub) PerformAccessibleAction(name string) {
	switch name {
	case core.ActionExpand:
		g.expanded = true
	case core.ActionCollapse:
		g.expanded = false
	}
}

// A labeled RoleTableRow — a grid group header — is a real reading stop with
// expand/collapse offered through the custom-actions channel. Before the #951
// re-verify fix the row fell through mobileA11yRole as a container, so group
// headers were unreachable under TalkBack/VoiceOver.
func TestMobileA11yGather_LabeledTableRowSurfaces(t *testing.T) {
	w := newA11yTestWindow(t)

	header := newGroupHeaderStub()
	w.SetContent(container.NewVBox(widget.NewLabel("above"), header))
	w.Resize(geo.NewSize(200, 200))

	nodes, _ := w.mobileA11yGather()
	require.Len(t, nodes, 2)
	assert.Equal(t, mobileRoleHeading, nodes[1].role,
		"a section header announces as a heading, distinguishing it from data rows")
	assert.Equal(t, "West (125)", nodes[1].label)
	require.Len(t, nodes[1].customActs, 1)
	name, ok := mobileA11yCustomActionName(nodes[1].customActs[0].id)
	require.True(t, ok)
	assert.Equal(t, core.ActionExpand, name)

	// Dispatching the custom action toggles the group and the declaration
	// flips to collapse on the next gather.
	mobileA11yDispatchOnMain(nodes[1].handle, mobileA11yActionCustom+nodes[1].customActs[0].id)
	assert.True(t, header.expanded)
	nodes, _ = w.mobileA11yGather()
	require.Len(t, nodes[1].customActs, 1)
	name, ok = mobileA11yCustomActionName(nodes[1].customActs[0].id)
	require.True(t, ok)
	assert.Equal(t, core.ActionCollapse, name)
}

// The tree walk emits a grid's regions block by block (pinned column subtree,
// then main cells), but linear AT navigation needs reading order: sorted per
// row band, cells by display column, a spanning header row alone in its band.
// Non-grid nodes keep their positions (#951).
func TestMobileA11ySortGridChildren_ReadingOrder(t *testing.T) {
	cell := func(row, col, x, y int) mobileA11yNode {
		return mobileA11yNode{gridHandle: 7, cellRow: row, cellCol: col, x: x, y: y, role: mobileRoleCell}
	}
	nodes := []mobileA11yNode{
		{role: mobileRoleText, label: "info", cellRow: -1}, // outside the grid
		{role: mobileRoleGrid, handle: 7, cellRow: -1},     // the container itself
		cell(1, 0, 0, 100), cell(2, 0, 0, 200), cell(3, 0, 0, 300), // pinned column block
		{gridHandle: 7, cellRow: -1, x: 80, y: 250, role: mobileRoleHeading, label: "West (125)"}, // header row band
		cell(1, 1, 80, 100), cell(1, 2, 160, 100), // main cells, rows 1-3
		cell(2, 1, 80, 200), cell(2, 2, 160, 200),
		cell(3, 1, 80, 300), cell(3, 2, 160, 300),
	}
	mobileA11ySortGridChildren(nodes)

	assert.Equal(t, "info", nodes[0].label, "non-grid nodes keep their position")
	assert.Equal(t, mobileRoleGrid, nodes[1].role)
	var order []string
	for _, n := range nodes[2:] {
		if n.role == mobileRoleHeading {
			order = append(order, "hdr")
			continue
		}
		order = append(order, fmt.Sprintf("r%dc%d", n.cellRow, n.cellCol))
	}
	assert.Equal(t,
		[]string{"r1c0", "r1c1", "r1c2", "r2c0", "r2c1", "r2c2", "hdr", "r3c0", "r3c1", "r3c2"},
		order, "row-major reading order with the header alone in its band")
}
