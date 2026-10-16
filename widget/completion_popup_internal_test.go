package widget

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

func newTestCompletionPopup(t *testing.T, items []CompletionItem) (bellui.Window, *CompletionPopup) {
	t.Helper()
	test.NewTempApp(t)
	w := test.NewTempWindow(t, canvas.NewRectangle(nil))
	w.Resize(geo.NewSize(300, 200))
	p := NewCompletionPopup(w.Canvas(), items)
	t.Cleanup(p.hideDetail) // stop any pending detail-tooltip timer
	return w, p
}

func sampleItems() []CompletionItem {
	return []CompletionItem{
		{Text: "alpha", Detail: "first letter", Icon: theme.InfoIcon()},
		{Text: "beta", Detail: "second letter"},
		{Text: "gamma"},
	}
}

func TestCompletionPopup_ShowHide(t *testing.T) {
	w, p := newTestCompletionPopup(t, sampleItems())

	assert.Empty(t, w.Canvas().Overlays().List())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.Len(t, w.Canvas().Overlays().List(), 1)

	p.Hide()
	assert.Empty(t, w.Canvas().Overlays().List())
}

func TestCompletionPopup_EmptyItemsDoesNotShow(t *testing.T) {
	w, p := newTestCompletionPopup(t, nil)
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.Empty(t, w.Canvas().Overlays().List())
}

func TestCompletionPopup_KeyboardNavigation(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())

	var got CompletionItem
	gotIndex := -1
	p.OnSelected = func(i int, item CompletionItem) { gotIndex, got = i, item }

	assert.Equal(t, 0, p.selected)
	p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 1, p.selected)
	p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 2, p.selected)
	p.TypedKey(&event.KeyEvent{Name: event.KeyDown}) // wraps
	assert.Equal(t, 0, p.selected)
	p.TypedKey(&event.KeyEvent{Name: event.KeyUp}) // wraps back
	assert.Equal(t, 2, p.selected)

	p.TypedKey(&event.KeyEvent{Name: event.KeyEnter})
	assert.Equal(t, 2, gotIndex)
	assert.Equal(t, "gamma", got.Text)
}

func TestCompletionPopup_EscapeDismisses(t *testing.T) {
	w, p := newTestCompletionPopup(t, sampleItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	require.Len(t, w.Canvas().Overlays().List(), 1)

	p.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.Empty(t, w.Canvas().Overlays().List())
}

func TestCompletionPopup_TappedSelectsRow(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	gotIndex := -1
	p.OnSelected = func(i int, _ CompletionItem) { gotIndex = i }

	m := p.metrics()
	// Tap within the second row.
	y := m.contentTop + m.rowHeight + m.rowHeight/2
	p.Tapped(&event.PointEvent{Position: geo.NewPos(20, y)})
	assert.Equal(t, 1, gotIndex)
}

func TestCompletionPopup_PlacedBelowCaret(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	caret := geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)}
	p.ShowAtCaret(caret)
	// Plenty of room below: top of popup is at the caret's bottom edge.
	assert.Equal(t, caret.Pos.Y+caret.Size.Height, p.Position().Y)
}

func TestCompletionPopup_FlipsAboveWhenCramped(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	// Caret near the bottom of the 200px-tall canvas: no room beneath.
	caret := geo.Rectangle{Pos: geo.NewPos(10, 188), Size: geo.NewSize(2, 10)}
	p.ShowAtCaret(caret)
	assert.Less(t, p.Position().Y, caret.Pos.Y, "popup should flip above the caret")
	assert.Equal(t, caret.Pos.Y-p.Size().Height, p.Position().Y)
}

func TestCompletionPopup_TitleGrowsContent(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	without := p.MinSize().Height
	p.SetTitle("Suggestions")
	with := p.MinSize().Height
	assert.Greater(t, with, without, "title should add a header above the items")
}

func TestCompletionPopup_InfoIconTrailsText(t *testing.T) {
	_, p := newTestCompletionPopup(t, []CompletionItem{{Text: "alpha", Detail: "info"}})
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	require.NotNil(t, r.rows[0].info)
	// LTR: info icon sits to the right of the text.
	assert.Greater(t, r.rows[0].info.Position().X, r.rows[0].text.Position().X)
}

func TestCompletionPopup_HoverSelectsRow(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	m := p.metrics()
	y := m.contentTop + 2*m.rowHeight + m.rowHeight/2 // third row
	p.HoverMoved(&event.HoverEvent{PointEvent: event.PointEvent{Position: geo.NewPos(20, y)}})
	assert.Equal(t, 2, p.selected)

	p.HoverOut() // selection persists for keyboard use
	assert.Equal(t, 2, p.selected)
}

func TestCompletionPopup_SetItemsResetsSelection(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	p.selected = 2
	p.SetItems([]CompletionItem{{Text: "only"}})
	assert.Equal(t, 0, p.selected, "selection should reset when it falls outside the new items")
	assert.Len(t, p.items, 1)
}

func TestCompletionPopup_GuardsDoNotPanic(t *testing.T) {
	_, empty := newTestCompletionPopup(t, nil)
	assert.NotPanics(t, func() {
		empty.moveSelection(1)                                      // no items
		empty.trigger(0)                                            // out of range
		empty.Tapped(&event.PointEvent{Position: geo.NewPos(5, 5)}) // no row
		empty.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(1, 1)})     // empty: no-op
	})
	assert.Equal(t, -1, empty.rowAt(geo.NewPos(0, -100)), "above content yields no row")
}

func TestCompletionPopup_DetailTooltipShowHide(t *testing.T) {
	w, p := newTestCompletionPopup(t, []CompletionItem{{Text: "alpha", Detail: "the detail"}})
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	require.Len(t, w.Canvas().Overlays().List(), 1)

	p.showDetail(0)
	assert.Len(t, w.Canvas().Overlays().List(), 2, "detail tooltip is a second overlay")

	p.hideDetail()
	assert.Len(t, w.Canvas().Overlays().List(), 1, "hiding the tooltip leaves only the popup")
}

func TestCompletionPopup_DetailScheduledForActiveRowOnly(t *testing.T) {
	_, p := newTestCompletionPopup(t, []CompletionItem{{Text: "a"}, {Text: "b", Detail: "d"}})
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.Equal(t, -1, p.detailRow, "nothing scheduled until selection changes")

	p.moveSelection(1) // row 1 has detail
	assert.Equal(t, 1, p.detailRow)

	p.moveSelection(1) // wraps to row 0, no detail
	assert.Equal(t, -1, p.detailRow)
}

func TestCompletionPopup_InfoIconRectFlipsWithDirection(t *testing.T) {
	_, p := newTestCompletionPopup(t, []CompletionItem{{Text: "alpha", Detail: "d"}})
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	ltr := p.infoIconRect(0)
	assert.Greater(t, ltr.Pos.X, p.Size().Width/2, "info icon trails on the right in LTR")

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionLTR) })
	rtl := p.infoIconRect(0)
	assert.Less(t, rtl.Pos.X, p.Size().Width/2, "info icon leads on the left in RTL")

	assert.Equal(t, geo.Rectangle{}, p.infoIconRect(5), "out-of-range row has no info icon")
}

func TestCompletionPopup_Renders(t *testing.T) {
	test.NewTempApp(t)
	items := []CompletionItem{
		{Text: "fmt.Println", Detail: "func(a ...any)", Icon: theme.InfoIcon()},
		{Text: "fmt.Printf", Detail: "func(format string, a ...any)", Icon: theme.InfoIcon()},
		{Text: "fmt.Sprintf", Detail: "func(format string, a ...any) string"},
		{Text: "fmt.Errorf"},
	}
	p := NewCompletionPopup(test.Canvas(), items)
	p.SetTitle("Completions")
	p.Resize(p.MinSize())
	test.AssertObjectRendersToImage(t, "completion_popup.png", p)
}

func TestCompletionPopup_InfoIconLeadsTextInRTL(t *testing.T) {
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionLTR) })

	_, p := newTestCompletionPopup(t, []CompletionItem{{Text: "alpha", Detail: "info"}})
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	require.NotNil(t, r.rows[0].info)
	// RTL: info icon mirrors to the leading (left) side, before the text.
	assert.Less(t, r.rows[0].info.Position().X, r.rows[0].text.Position().X)
}

// --- visible-row cap + scrolling (pre-rebrand tracker #436) ---

func manyItems(n int) []CompletionItem {
	items := make([]CompletionItem, n)
	for i := range items {
		items[i] = CompletionItem{Text: fmt.Sprintf("item-%02d", i), Detail: "d"}
	}
	return items
}

func TestCompletionPopup_CapsVisibleRows(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))

	assert.Equal(t, defaultMaxVisibleRows, p.visibleRows())
	m := p.metrics()
	wantH := m.contentTop + float32(defaultMaxVisibleRows)*m.rowHeight + m.pad
	assert.Equal(t, wantH, p.MinSize().Height, "height is capped, not sized to all 20 rows")

	p.MaxVisibleRows = 3
	assert.Equal(t, 3, p.visibleRows())
	wantH = m.contentTop + 3*m.rowHeight + m.pad
	assert.Equal(t, wantH, p.MinSize().Height)

	// A list shorter than the cap sizes to its own length.
	p.SetItems(manyItems(2))
	assert.Equal(t, 2, p.visibleRows())
}

func TestCompletionPopup_KeyboardKeepsSelectionInWindow(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	// Walk below the bottom edge: the window follows the selection.
	for i := 0; i < 11; i++ {
		p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	}
	assert.Equal(t, 11, p.selected)
	assert.Equal(t, 11-defaultMaxVisibleRows+1, p.top, "window scrolled so the selection stays visible")

	// Walk back above the top edge.
	for i := 0; i < 9; i++ {
		p.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	}
	assert.Equal(t, 2, p.selected)
	assert.Equal(t, 2, p.top)

	// Wrap-around from the top lands on the last row, window at the end.
	for i := 0; i < 3; i++ {
		p.TypedKey(&event.KeyEvent{Name: event.KeyUp})
	}
	assert.Equal(t, 19, p.selected)
	assert.Equal(t, 20-defaultMaxVisibleRows, p.top)

	// …and wrapping down from the bottom returns to the start.
	p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 0, p.selected)
	assert.Equal(t, 0, p.top)
}

func TestCompletionPopup_ScrolledMovesWindow(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	m := p.metrics()

	// Wheel down (negative DY) reveals later rows; selection stays put.
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -2*m.rowHeight)})
	assert.Equal(t, 2, p.top)
	assert.Equal(t, 0, p.selected)

	// Sub-row deltas bank up instead of being dropped.
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -m.rowHeight/2)})
	assert.Equal(t, 2, p.top)
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -m.rowHeight/2)})
	assert.Equal(t, 3, p.top)

	// Clamped at both ends.
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -100*m.rowHeight)})
	assert.Equal(t, 20-defaultMaxVisibleRows, p.top)
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, 100*m.rowHeight)})
	assert.Equal(t, 0, p.top)

	// A list that fits does not scroll.
	p.SetItems(manyItems(3))
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -2*m.rowHeight)})
	assert.Equal(t, 0, p.top)
}

func TestCompletionPopup_HitTestingAccountsForScroll(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	m := p.metrics()
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -5*m.rowHeight)})
	require.Equal(t, 5, p.top)

	firstRow := geo.NewPos(m.pad+1, m.contentTop+m.rowHeight/2)
	assert.Equal(t, 5, p.rowAt(firstRow), "the first visible row is the fifth item")

	// Below the visible window is a miss even though more items exist.
	belowWindow := geo.NewPos(m.pad+1, m.contentTop+float32(defaultMaxVisibleRows)*m.rowHeight+1)
	assert.Equal(t, -1, p.rowAt(belowWindow))

	var got int
	p.OnSelected = func(i int, _ CompletionItem) { got = i }
	p.Tapped(&event.PointEvent{Position: firstRow})
	assert.Equal(t, 5, got, "tapping accepts the item behind the scrolled window")

	// The info-icon anchor is window-relative too, and empty off-window.
	assert.Equal(t, geo.Rectangle{}, p.infoIconRect(2), "row above the window has no on-screen icon")
	r := p.infoIconRect(5)
	assert.Equal(t, m.contentTop+(m.rowHeight-m.iconSize)/2, r.Pos.Y)
}

func TestCompletionPopup_RendererWindowsRows(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	m := p.metrics()

	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	r.Layout(p.Size())
	assert.False(t, r.rows[0].text.Hidden)
	assert.False(t, r.rows[7].text.Hidden)
	assert.True(t, r.rows[8].text.Hidden, "rows past the window are not drawn")
	assert.False(t, r.bar.Hidden, "overflow shows the scroll indicator")
	assert.False(t, r.sel.Hidden)

	// Scroll the selection out of the window: its highlight hides with it.
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -5*m.rowHeight)})
	r.Layout(p.Size())
	assert.True(t, r.rows[0].text.Hidden, "rows before the window are not drawn")
	assert.False(t, r.rows[5].text.Hidden)
	assert.True(t, r.sel.Hidden, "selected row 0 is scrolled away")

	// The indicator tracks the window position proportionally.
	trackH := p.Size().Height - m.contentTop - m.pad
	assert.InDelta(t, m.contentTop+trackH*5/20, r.bar.Position().Y, 0.001)
	assert.InDelta(t, trackH*float32(defaultMaxVisibleRows)/20, r.bar.Size().Height, 0.001)

	// A fitting list hides the indicator and draws every row.
	p.SetItems(manyItems(3))
	r.Refresh()
	assert.True(t, r.bar.Hidden)
	assert.False(t, r.rows[2].text.Hidden)
}

func TestCompletionPopup_SetItemsResetsWindow(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	m := p.metrics()
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -6*m.rowHeight)})
	require.Equal(t, 6, p.top)

	p.SetItems(manyItems(15))
	assert.Equal(t, 0, p.top, "new items start at the top of the window")
}

// --- documentation side panel (pre-rebrand tracker #435) ---

func docItems() []CompletionItem {
	return []CompletionItem{
		{Text: "plain", Detail: "d", Documentation: "verbatim help text"},
		{Text: "rich", Documentation: "**bold** help", DocumentationMarkdown: true},
		{Text: "bare"},
	}
}

func TestCompletionPopup_DocPanelWidensPopup(t *testing.T) {
	items := sampleItems()
	_, p := newTestCompletionPopup(t, items)
	without := p.MinSize().Width

	items[0].Documentation = "help"
	p.SetItems(items)
	_, area := p.canvas.InteractiveArea()
	assert.Equal(t, area.Width/2, p.docPanelWidth(), "clamped to half the canvas on a narrow window")
	assert.Less(t, p.docPanelWidth(), float32(completionDocPanelWidth))
	assert.Equal(t, without+p.docPanelWidth()+completionDocDividerWidth, p.MinSize().Width,
		"popup widens by exactly the panel and its divider")

	items[0].Documentation = ""
	p.SetItems(items)
	assert.Zero(t, p.docPanelWidth(), "no documentation anywhere: no panel")
	assert.Equal(t, without, p.MinSize().Width)
}

func TestCompletionPopup_DocPanelFollowsSelection(t *testing.T) {
	_, p := newTestCompletionPopup(t, docItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	r.Refresh()

	require.NotNil(t, p.docRich)
	assert.Contains(t, p.docRich.String(), "verbatim help text")

	p.moveSelection(1) // markdown item
	assert.Contains(t, p.docRich.String(), "bold")
	assert.Greater(t, len(p.docRich.Segments), 1, "markdown parsed into rich segments")

	p.moveSelection(1) // item without documentation: panel goes empty
	assert.Empty(t, p.docRich.String())
}

func TestCompletionPopup_DocPanelSuppressesDetailTooltip(t *testing.T) {
	_, p := newTestCompletionPopup(t, docItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	p.moveSelection(3) // wrap to row 0, which has a Detail string
	require.Equal(t, 0, p.selected)
	assert.Equal(t, -1, p.detailRow, "detail tooltip is subsumed by the doc panel")
}

func TestCompletionPopup_DocPanelHitTesting(t *testing.T) {
	_, p := newTestCompletionPopup(t, docItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	m := p.metrics()
	yFirst := m.contentTop + m.rowHeight/2

	listX0, listW := m.listArea(p.Size().Width, false)
	assert.Equal(t, 0, p.rowAt(geo.NewPos(listX0+m.pad, yFirst)), "list area hits rows")
	assert.Equal(t, -1, p.rowAt(geo.NewPos(listX0+listW+10, yFirst)), "doc panel is not a row (LTR)")

	bellui.SetLayoutDirection(bellui.DirectionRTL)
	t.Cleanup(func() { bellui.SetLayoutDirection(bellui.DirectionLTR) })
	listX0, _ = m.listArea(p.Size().Width, true)
	assert.Equal(t, -1, p.rowAt(geo.NewPos(m.docW/2, yFirst)), "doc panel is not a row (RTL, leading side)")
	assert.Equal(t, 0, p.rowAt(geo.NewPos(listX0+m.pad, yFirst)), "list area shifted past the panel in RTL")
}

func TestCompletionPopup_DocPanelLayout(t *testing.T) {
	_, p := newTestCompletionPopup(t, docItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	r.Layout(p.Size())

	m := p.metrics()
	_, listW := m.listArea(p.Size().Width, false)
	assert.True(t, p.docScroll.Visible())
	assert.False(t, r.docDivider.Hidden)
	assert.Greater(t, p.docScroll.Position().X, listW, "panel sits on the trailing side of the list (LTR)")
	assert.Equal(t, listW, r.docDivider.Position().X)

	// Without documentation the panel and divider disappear and rows regain
	// the full width.
	p.SetItems(sampleItems())
	r.Refresh()
	assert.False(t, p.docScroll.Visible())
	assert.True(t, r.docDivider.Hidden)
}

func TestCompletionPopup_PageKeysScrollDocPanel(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += fmt.Sprintf("line %d\n", i)
	}
	_, p := newTestCompletionPopup(t, []CompletionItem{{Text: "a", Documentation: long}})
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	r.Refresh()
	require.Positive(t, p.docScroll.Content.Size().Height-p.docScroll.Size().Height,
		"the long documentation must overflow the panel for this test")

	p.TypedKey(&event.KeyEvent{Name: event.KeyPageDown})
	assert.Positive(t, p.docScroll.Offset.Y, "PageDown scrolls the doc panel")

	p.TypedKey(&event.KeyEvent{Name: event.KeyPageUp})
	assert.Zero(t, p.docScroll.Offset.Y, "PageUp scrolls back to the top")

	// Without a panel the key is not consumed by the popup.
	_, bare := newTestCompletionPopup(t, sampleItems())
	assert.False(t, bare.scrollDocPage(true))
}

// --- final-bounds signal for companion surfaces (pre-rebrand tracker #437) ---

func TestCompletionPopup_OnMovedReportsFinalPlacement(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	var got []geo.Rectangle
	p.OnMoved = func(r geo.Rectangle) { got = append(got, r) }

	caret := geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)}
	p.ShowAtCaret(caret)
	require.Len(t, got, 1, "one report for the initial placement")
	assert.Equal(t, geo.Rectangle{Pos: p.Position(), Size: p.Size()}, got[0],
		"the reported rectangle is exactly Position()+Size()")
	assert.Equal(t, caret.Pos.Y+caret.Size.Height, got[0].Pos.Y)

	p.ShowAtCaret(caret)
	assert.Len(t, got, 1, "unchanged placement is not re-reported")

	p.SetItems(manyItems(9)) // live resize while visible
	require.Len(t, got, 2)
	assert.Equal(t, geo.Rectangle{Pos: p.Position(), Size: p.Size()}, got[1])
	assert.NotEqual(t, got[0].Size, got[1].Size)

	p.Hide()
	require.Len(t, got, 3)
	assert.Equal(t, geo.Rectangle{}, got[2], "hiding reports the zero rectangle")
	p.Hide()
	assert.Len(t, got, 3, "already hidden: no duplicate zero report")

	p.ShowAtCaret(caret)
	assert.Len(t, got, 4, "re-showing after a hide re-reports the anchor")
}

func TestCompletionPopup_OnMovedSeesFlipAndDismiss(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	var last geo.Rectangle
	p.OnMoved = func(r geo.Rectangle) { last = r }

	// Caret near the bottom: the popup flips above, and that final placement
	// is what a companion gets to anchor against.
	caret := geo.Rectangle{Pos: geo.NewPos(10, 188), Size: geo.NewSize(2, 10)}
	p.ShowAtCaret(caret)
	assert.Equal(t, caret.Pos.Y-p.Size().Height, last.Pos.Y)

	// Escape dismissal zeroes the anchor even though the host never called Hide.
	p.TypedKey(&event.KeyEvent{Name: event.KeyEscape})
	assert.Equal(t, geo.Rectangle{}, last)
}

// --- pre-rebrand tracker #442: non-focus-stealing mode for editor hosts ---

func TestCompletionPopup_KeepHostFocusLeavesFocus(t *testing.T) {
	test.NewTempApp(t)
	host := NewMultiLineEntry()
	w := test.NewTempWindow(t, host)
	w.Resize(geo.NewSize(300, 200))
	w.Canvas().Focus(host)

	p := NewCompletionPopup(w.Canvas(), sampleItems())
	t.Cleanup(p.hideDetail)
	p.KeepHostFocus = true
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	assert.Len(t, w.Canvas().Overlays().List(), 1)
	assert.Equal(t, host, w.Canvas().Focused(), "focus stays on the host")

	// The crux of the mode: driver key dispatch targets Canvas().Focused(),
	// which is still the host while the popup overlay is up — typing edits
	// the host through its own dispatch, no forwarding involved.
	w.Canvas().Focused().TypedRune('a')
	assert.Equal(t, "a", host.Text)

	p.Hide()
	assert.Equal(t, host, w.Canvas().Focused(), "nothing to restore on hide")
}

func TestCompletionPopup_KeepHostFocusOverlayFlags(t *testing.T) {
	test.NewTempApp(t)
	host := NewMultiLineEntry()
	w := test.NewTempWindow(t, host)
	w.Resize(geo.NewSize(300, 200))

	p := NewCompletionPopup(w.Canvas(), sampleItems())
	t.Cleanup(p.hideDetail)
	p.KeepHostFocus = true
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.True(t, p.overlay.FocusTransparent, "host keeps key routing (#442)")
	assert.True(t, p.overlay.TapThrough, "outside tap passes through to the host (#472)")
	p.Hide()

	p2 := NewCompletionPopup(w.Canvas(), sampleItems())
	t.Cleanup(p2.hideDetail)
	p2.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.False(t, p2.overlay.TapThrough, "default popup keeps the swallowing outside tap")
}

func TestCompletionPopup_DefaultModeStillFocuses(t *testing.T) {
	test.NewTempApp(t)
	if bellui.CurrentDevice().IsMobile() {
		t.Skip("the default model only takes focus on desktop-class devices")
	}
	host := NewMultiLineEntry()
	w := test.NewTempWindow(t, host)
	w.Resize(geo.NewSize(300, 200))
	w.Canvas().Focus(host)

	p := NewCompletionPopup(w.Canvas(), sampleItems())
	t.Cleanup(p.hideDetail)
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	assert.Equal(t, p, w.Canvas().Focused(), "#361 model unchanged by default")
}

func TestCompletionPopup_HandleKey(t *testing.T) {
	w, p := newTestCompletionPopup(t, sampleItems())
	p.KeepHostFocus = true

	assert.False(t, p.HandleKey(&event.KeyEvent{Name: event.KeyDown}),
		"hidden popup consumes nothing")

	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	gotIndex := -1
	p.OnSelected = func(i int, _ CompletionItem) { gotIndex = i }

	assert.True(t, p.HandleKey(&event.KeyEvent{Name: event.KeyDown}))
	assert.Equal(t, 1, p.selected)
	assert.True(t, p.HandleKey(&event.KeyEvent{Name: event.KeyUp}))
	assert.Equal(t, 0, p.selected)

	assert.False(t, p.HandleKey(&event.KeyEvent{Name: event.KeyBackspace}),
		"editing keys are the host's — no forwarding needed, the host kept focus")
	assert.False(t, p.HandleKey(&event.KeyEvent{Name: event.KeyPageDown}),
		"no documentation panel: host pages the editor caret")

	assert.True(t, p.HandleKey(&event.KeyEvent{Name: event.KeyReturn}))
	assert.Equal(t, 0, gotIndex, "Enter accepts the selection")

	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.True(t, p.HandleKey(&event.KeyEvent{Name: event.KeyEscape}))
	assert.Empty(t, w.Canvas().Overlays().List(), "Escape dismisses")
	assert.False(t, p.HandleKey(&event.KeyEvent{Name: event.KeyDown}),
		"dismissed popup consumes nothing again")
}

func TestCompletionPopup_HandleKeyPagesDocPanel(t *testing.T) {
	_, p := newTestCompletionPopup(t, docItems())
	p.KeepHostFocus = true
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	r := test.TempWidgetRenderer(t, p).(*completionPopupRenderer)
	r.Refresh()
	require.NotNil(t, p.docScroll)

	assert.True(t, p.HandleKey(&event.KeyEvent{Name: event.KeyPageDown}),
		"PageDown consumed while the documentation panel shows")
}

func TestCompletionPopup_Reposition(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())

	var moved []geo.Rectangle
	p.OnMoved = func(r geo.Rectangle) { moved = append(moved, r) }

	p.Reposition(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})
	assert.Empty(t, moved, "Reposition is a no-op while hidden")

	caret := geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)}
	p.ShowAtCaret(caret)
	require.Len(t, moved, 1)
	below := p.Position()
	assert.Equal(t, caret.Pos.Y+caret.Size.Height, below.Y)

	// The caret moved down with a scroll: the popup follows and reports it.
	scrolled := geo.Rectangle{Pos: geo.NewPos(10, 60), Size: geo.NewSize(2, 14)}
	p.Reposition(scrolled)
	assert.Equal(t, scrolled.Pos.Y+scrolled.Size.Height, p.Position().Y)
	require.Len(t, moved, 2)
	assert.Equal(t, p.Position(), moved[1].Pos)

	// Near the bottom edge it flips above the caret, like ShowAtCaret.
	low := geo.Rectangle{Pos: geo.NewPos(10, 180), Size: geo.NewSize(2, 14)}
	p.Reposition(low)
	assert.Equal(t, low.Pos.Y-p.Size().Height, p.Position().Y, "flips above when cramped")
}

func TestCompletionPopup_AccessibleContainer(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	p.SetTitle("Suggestions")

	assert.Equal(t, core.RoleList, p.AccessibleRole())
	assert.Equal(t, "Suggestions", p.AccessibleLabel())
}

func TestCompletionPopup_AccessibleVirtualNodes(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	nodes := p.AccessibleVirtualNodes()
	require.Len(t, nodes, 3, "one node per suggestion when all fit")
	for i, n := range nodes {
		assert.Equal(t, core.RoleListItem, n.Role)
		assert.Equal(t, sampleItems()[i].Text, n.Label)
		assert.Equal(t, sampleItems()[i].Detail, n.Description)
		assert.Equal(t, completionRowID{popup: p, index: i}, n.ID)
	}

	// The first row is selected by default.
	assert.True(t, nodes[0].State.Has(core.StateSelected))
	assert.False(t, nodes[1].State.Has(core.StateSelected))

	// Selecting another row moves the StateSelected flag.
	p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	nodes = p.AccessibleVirtualNodes()
	assert.False(t, nodes[0].State.Has(core.StateSelected))
	assert.True(t, nodes[1].State.Has(core.StateSelected))
}

func TestCompletionPopup_AccessibleVirtualNodesWindowed(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.MaxVisibleRows = 5
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	// Only the visible window is emitted, not all 20 rows.
	nodes := p.AccessibleVirtualNodes()
	require.Len(t, nodes, 5)
	assert.Equal(t, "item-00", nodes[0].Label)

	// Arrow past the bottom edge: the window follows, still 5 nodes, and the
	// selected row (kept in view) is present and flagged.
	for i := 0; i < 10; i++ {
		p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	}
	nodes = p.AccessibleVirtualNodes()
	require.Len(t, nodes, 5)
	var selected int
	for i, n := range nodes {
		if n.State.Has(core.StateSelected) {
			selected = n.ID.(completionRowID).index
			assert.Equal(t, "item-10", n.Label)
			_ = i
		}
	}
	assert.Equal(t, 10, selected)
}

func TestCompletionPopup_AccessibleActiveDescendant(t *testing.T) {
	_, p := newTestCompletionPopup(t, sampleItems())
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	assert.Equal(t, completionRowID{popup: p, index: 0}, p.AccessibleActiveDescendant())

	p.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, completionRowID{popup: p, index: 1}, p.AccessibleActiveDescendant())

	// The active descendant is always one of the emitted virtual nodes.
	desc := p.AccessibleActiveDescendant()
	var found bool
	for _, n := range p.AccessibleVirtualNodes() {
		if n.ID == desc {
			found = true
		}
	}
	assert.True(t, found, "active descendant must be present in the virtual node window")
}

func TestCompletionPopup_ActiveDescendantNilWhenScrolledOut(t *testing.T) {
	_, p := newTestCompletionPopup(t, manyItems(20))
	p.MaxVisibleRows = 5
	p.ShowAtCaret(geo.Rectangle{Pos: geo.NewPos(10, 20), Size: geo.NewSize(2, 14)})

	// Selection stays at row 0; scroll the window far past it with the wheel.
	p.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -1000)})
	require.Greater(t, p.top, p.selected, "the selected row scrolled out of the window")

	assert.Nil(t, p.AccessibleActiveDescendant(),
		"a designated descendant must be a node in the emitted window")
}
