package widget

import (
	"image/color"
	"sync"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

var (
	_ core.Widget                        = (*CompletionPopup)(nil)
	_ core.Accessible                    = (*CompletionPopup)(nil)
	_ core.AccessibleVirtualNodeProvider = (*CompletionPopup)(nil)
	_ core.AccessibleActiveDescendant    = (*CompletionPopup)(nil)
	_ event.Focusable                    = (*CompletionPopup)(nil)
	_ event.Tappable                     = (*CompletionPopup)(nil)
	_ event.Hoverable                    = (*CompletionPopup)(nil)
	_ event.Scrollable                   = (*CompletionPopup)(nil)
)

// defaultMaxVisibleRows is the visible-row cap applied when
// [CompletionPopup.MaxVisibleRows] is zero.
const defaultMaxVisibleRows = 8

// completionDocPanelWidth is the preferred width of the documentation side
// panel, bounded to half the canvas when space is tight.
const completionDocPanelWidth = 320

// completionDocDividerWidth is the vertical rule between list and doc panel.
const completionDocDividerWidth = 1

// CompletionItem is a single suggestion shown in a [CompletionPopup].
//
// Since: 1.0
type CompletionItem struct {
	// Text is the primary suggestion, shown leading and inserted on selection.
	Text string
	// Detail is optional secondary information about the item. When set, a
	// trailing information icon is shown for the row (leading when the active
	// layout direction is right-to-left).
	Detail string
	// Icon is an optional leading icon (for example a completion-kind glyph).
	Icon bellui.Resource

	// Documentation is optional longer help for the item — e.g. a language
	// server's documentation string. When any item in the popup carries
	// documentation, a scrollable panel is shown beside the list and follows
	// the selection; it replaces the small Detail tooltip while visible (pre-rebrand tracker #435).
	//
	// Since: 1.0
	Documentation string
	// DocumentationMarkdown renders Documentation as markdown when true, and
	// verbatim text otherwise.
	//
	// Since: 1.0
	DocumentationMarkdown bool
}

// CompletionPopup is a lightweight, application-driven completion/suggestion
// popup. The host application supplies the items (autocomplete, history,
// snippets, language-server completions, …) and the popup renders them anchored
// at a caret rectangle.
//
// It is deliberately styled not to compete with the rest of the UI: no shadow,
// a subtle corner radius and a thin outline so it still reads as a distinct
// surface. Each row shows its text with an optional trailing information icon;
// the whole row mirrors when the active layout direction is right-to-left, so
// the icon moves to the leading edge. A list longer than [MaxVisibleRows] is
// capped and scrolls behind the visible window instead of growing taller.
//
// This is distinct from the platform IME candidate window, which the operating
// system owns and positions itself; CompletionPopup complements it for
// app-level suggestions.
//
// Since: 1.0
type CompletionPopup struct {
	BaseWidget

	// OnSelected is called when the user accepts an item (Enter, Space or tap).
	OnSelected func(index int, item CompletionItem)

	// MaxVisibleRows caps how many suggestion rows are visible at once; a
	// longer list scrolls behind that window (Up/Down keep the selection in
	// view, the mouse wheel moves the window, and a thin indicator shows the
	// position). Zero applies the default of 8 rows (pre-rebrand tracker #436).
	//
	// Since: 1.0
	MaxVisibleRows int

	// KeepHostFocus, when set before Show, leaves keyboard focus where it is
	// instead of focusing the popup: the popup's overlay becomes
	// focus-transparent, so the canvas keeps delivering keys, runes and
	// shortcuts to the focused widget through its own dispatch as if the
	// popup were not open. The popup then receives no key events itself; the
	// host offers them via [CompletionPopup.HandleKey] (arrows, Enter, Tab,
	// Escape → consumed) and accepts the selection through OnSelected as
	// usual. The overlay is also tap-through on desktop (pre-rebrand tracker #472): a click
	// outside the popup dismisses it and is re-delivered to the widget
	// beneath, so the same click places the caret in the editor host.
	// Intended for editor hosts whose caret, focus-loss logic, window
	// shortcuts and key routing must survive the popup being open (pre-rebrand tracker #442);
	// plain [Entry.ShowCompletion] hosts keep the default forwarding model
	// (pre-rebrand tracker #361).
	//
	// Since: 1.0
	KeepHostFocus bool

	// OnMoved, when set, is called with the popup's final on-canvas rectangle
	// — the same values [CompletionPopup.Position] and [CompletionPopup.Size]
	// report — whenever the placement changes while the popup is visible: the
	// initial show (after [CompletionPopup.ShowAtCaret]'s below/above flip and
	// on-screen clamping), a live [CompletionPopup.SetItems] resize, or any
	// explicit move. When the popup hides it is called once with the zero
	// rectangle. Hosts use it to anchor a companion surface — e.g. a signature
	// help card — next to wherever the popup actually landed (pre-rebrand tracker #437).
	//
	// Since: 1.0
	OnMoved func(geo.Rectangle) `json:"-"`

	canvas   core.Canvas
	overlay  *intWidget.OverlayContainer
	items    []CompletionItem
	title    string
	selected int

	// top is the first visible row of the scrolled window onto items, kept in
	// [0, len(items)-visibleRows()]. scrollAccum banks sub-row wheel deltas.
	top         int
	scrollAccum float32

	// docScroll/docRich are the documentation side panel (pre-rebrand tracker #435), created with
	// the renderer and shown only while some item carries Documentation.
	docScroll *intWidget.Scroll
	docRich   *RichText

	// lastRect is the placement last reported through OnMoved, so unchanged
	// placements do not re-notify. shown gates the reports to the span between
	// Show and Hide — a fresh BaseWidget claims Visible() before its first
	// Show, which would leak ShowAtCaret's pre-placement Resize (pre-rebrand tracker #437).
	lastRect geo.Rectangle
	shown    bool

	// host, when set, is the Entry that owns this popup. The popup is the focused
	// overlay while open and forwards typing/editing keys back to the host so the
	// text keeps editing and the list filters live (pre-rebrand tracker #361).
	host *Entry

	// Delayed detail tooltip for the active row's info icon (pre-rebrand tracker #362).
	detailMu    sync.Mutex
	detailTimer *time.Timer
	detailPopup *PopUp
	detailRow   int
}

// NewCompletionPopup creates a completion popup for the given items on canvas c.
// Show it with [CompletionPopup.ShowAtCaret].
//
// Since: 1.0
func NewCompletionPopup(c core.Canvas, items []CompletionItem) *CompletionPopup {
	p := &CompletionPopup{canvas: c, items: items, detailRow: -1}
	p.ExtendBaseWidget(p)
	o := intWidget.NewOverlayContainer(p, c, p.dismiss)
	o.Resize(o.MinSize())
	p.overlay = o
	return p
}

// SetTitle sets an optional header shown above the suggestions. An empty string
// removes the header.
func (p *CompletionPopup) SetTitle(title string) {
	p.title = title
	p.Refresh()
}

// SetItems replaces the popup's suggestions, resetting the active selection.
// When the popup is already showing it resizes to fit the new items, so it can
// be driven live as the edited text filters the list.
func (p *CompletionPopup) SetItems(items []CompletionItem) {
	p.items = items
	if p.selected >= len(items) {
		p.selected = 0
	}
	p.top = 0
	p.scrollAccum = 0
	p.scrollSelectionIntoView()
	if p.Visible() {
		p.Resize(p.MinSize())
	}
	p.hideDetail() // the row under the pointer/selection may now mean something else
	p.Refresh()
}

// AccessibleRole returns [core.RoleList].
func (p *CompletionPopup) AccessibleRole() core.AccessibleRole { return core.RoleList }

// AccessibleLabel returns the popup's title, if any.
func (p *CompletionPopup) AccessibleLabel() string { return p.title }

// completionRowID is the stable accessibility identity of a suggestion row. It
// embeds the owning popup so a bare row index never collides with another
// widget's, per [core.AccessibleIdentified]. It is comparable, so backends can
// use it as an element-map key and match it against
// [CompletionPopup.AccessibleActiveDescendant].
type completionRowID struct {
	popup *CompletionPopup
	index int
}

// AccessibleVirtualNodes reports one [core.RoleListItem] node per suggestion in
// the visible window. The popup paints its rows itself (flat canvas objects, no
// per-row widget), so without these virtual nodes a screen reader sees only the
// list container. Only the scrolled-in window is emitted — the same rows the
// renderer lays out — matching the [CompletionPopup.MaxVisibleRows] windowing;
// the keyboard-selected row is always kept inside that window
// (scrollSelectionIntoView), so the active descendant is always present. The
// selected row carries [core.StateSelected]; each node's ID is a
// [completionRowID] so the backend can track it and resolve the active
// descendant (pre-rebrand tracker #801).
func (p *CompletionPopup) AccessibleVirtualNodes() []core.AccessibleVirtualNode {
	if len(p.items) == 0 {
		return nil
	}
	m := p.metrics()
	listX0, listW := m.listArea(p.Size().Width, isRTLAt(p.super()))
	top, visible := p.top, p.visibleRows()

	nodes := make([]core.AccessibleVirtualNode, 0, visible)
	for i := top; i < top+visible && i < len(p.items); i++ {
		item := p.items[i]
		var state core.AccessibleState
		if i == p.selected {
			state = core.StateSelected
		}
		rowY := m.contentTop + float32(i-top)*m.rowHeight
		nodes = append(nodes, core.AccessibleVirtualNode{
			AccessibleNode: core.AccessibleNode{
				Role:        core.RoleListItem,
				Label:       item.Text,
				Description: item.Detail,
				State:       state,
			},
			Pos:  geo.NewPos(listX0, rowY),
			Size: geo.NewSize(listW, m.rowHeight),
			ID:   completionRowID{popup: p, index: i},
		})
	}
	return nodes
}

// AccessibleActiveDescendant returns the identity of the keyboard-selected row,
// so assistive technology follows and announces each candidate as the user
// arrows through the list rather than re-announcing the whole popup (pre-rebrand tracker #688). It
// reports nil when the selection is scrolled outside the emitted window (a
// wheel scroll that left the cursor behind), since a designated descendant must
// be a node in the reconciled tree.
func (p *CompletionPopup) AccessibleActiveDescendant() any {
	if p.selected < 0 || p.selected >= len(p.items) {
		return nil
	}
	if p.selected < p.top || p.selected >= p.top+p.visibleRows() {
		return nil
	}
	return completionRowID{popup: p, index: p.selected}
}

// ShowAtCaret shows the popup anchored to the caret rectangle (in canvas
// coordinates). It is placed just below the caret, flipping above it when there
// is not enough room beneath, so it never covers the line being typed.
func (p *CompletionPopup) ShowAtCaret(caret geo.Rectangle) {
	if len(p.items) == 0 {
		return
	}
	p.Resize(p.MinSize())
	p.placeAt(caret)
	p.Show()
}

// Reposition re-anchors a visible popup to a new caret rectangle, with the
// same below/above flip and on-screen clamping as [CompletionPopup.ShowAtCaret].
// Hosts call it from their scroll handler so the popup follows the caret while
// the content scrolls underneath; a set OnMoved observes the new placement.
// A no-op while the popup is hidden.
//
// Since: 1.0
func (p *CompletionPopup) Reposition(caret geo.Rectangle) {
	if !p.shown {
		return
	}
	p.placeAt(caret)
}

// placeAt positions the popup just below the caret rectangle, flipping above
// it when there is not enough room beneath, so it never covers the line being
// typed.
func (p *CompletionPopup) placeAt(caret geo.Rectangle) {
	size := p.Size()
	_, area := p.canvas.InteractiveArea()

	y := caret.Pos.Y + caret.Size.Height // below the caret
	if y+size.Height > area.Height && caret.Pos.Y-size.Height >= 0 {
		y = caret.Pos.Y - size.Height // not enough room below: flip above
	}
	p.Move(geo.NewPos(caret.Pos.X, y))
}

// Move positions the popup. The position is absolute within the canvas overlay
// and is clamped so the popup stays on-screen. [CompletionPopup.Position]
// reports the final, clamped placement.
func (p *CompletionPopup) Move(pos geo.Position) {
	p.BaseWidget.Move(p.adjustedPosition(pos, p.Size()))
	p.notifyMoved()
}

// Resize changes the popup size, keeping it within the canvas.
func (p *CompletionPopup) Resize(size geo.Size) {
	p.BaseWidget.Move(p.adjustedPosition(p.Position(), size))
	p.BaseWidget.Resize(size)
	p.notifyMoved()
}

// notifyMoved reports the current placement through OnMoved when it differs
// from the last report. Nothing is reported while the popup is hidden; Hide
// sends the zero rectangle itself.
func (p *CompletionPopup) notifyMoved() {
	if p.OnMoved == nil || !p.shown {
		return
	}
	r := geo.Rectangle{Pos: p.Position(), Size: p.Size()}
	if r == p.lastRect {
		return
	}
	p.lastRect = r
	p.OnMoved(r)
}

// Show makes the popup visible and gives it keyboard focus on desktop. When the
// popup has a host Entry it stays focused but forwards typing back to the host
// (see TypedKey/TypedRune), so the Entry keeps its caret and edits live. With
// [CompletionPopup.KeepHostFocus] set, focus is left untouched and the focused
// host drives the popup through [CompletionPopup.HandleKey] instead.
func (p *CompletionPopup) Show() {
	// In KeepHostFocus mode the overlay is focus-transparent: the canvas
	// keeps routing keys, runes and shortcuts to the focused host as if the
	// popup were not there, and the host offers keys via HandleKey. It is
	// also tap-through (pre-rebrand tracker #472): the outside click that dismisses the popup is
	// re-delivered to the widget beneath, so an editor host dismisses and
	// places the caret in one click.
	p.overlay.FocusTransparent = p.KeepHostFocus
	p.overlay.TapThrough = p.KeepHostFocus
	p.overlay.Show()
	p.BaseWidget.Show()
	if !p.KeepHostFocus && !bellui.CurrentDevice().IsMobile() {
		p.canvas.Focus(p)
	}
	p.shown = true
	p.notifyMoved()
}

// Hide hides the popup and any open detail tooltip. A set OnMoved observes the
// zero rectangle, so a companion surface can dismiss with the popup no matter
// what hid it (Escape, an outside tap, or an accepted item).
func (p *CompletionPopup) Hide() {
	p.hideDetail()
	p.overlay.Hide()
	p.BaseWidget.Hide()
	p.shown = false
	if p.OnMoved != nil && p.lastRect != (geo.Rectangle{}) {
		p.lastRect = geo.Rectangle{}
		p.OnMoved(geo.Rectangle{})
	}
}

func (p *CompletionPopup) dismiss() {
	p.Hide()
}

// FocusGained satisfies event.Focusable; the popup needs no visual focus change.
func (p *CompletionPopup) FocusGained() {}

// FocusLost satisfies event.Focusable.
func (p *CompletionPopup) FocusLost() {}

// TypedRune forwards typed characters to the host Entry, so typing continues to
// edit the text (and filter the list) while the popup is open. Standalone popups
// ignore runes.
func (p *CompletionPopup) TypedRune(r rune) {
	if p.host != nil {
		p.host.TypedRune(r)
	}
}

// HandleKey lets a focused host offer a key event to a visible popup: up/down
// move the selection, Enter/Tab accept, Escape dismisses and PageUp/PageDown
// page the documentation panel when it is showing. It reports whether the
// popup consumed the event; when false (including for a hidden popup) the
// host handles the key natively. This is the key-routing surface for
// [CompletionPopup.KeepHostFocus] hosts, and the same switch the popup's own
// TypedKey consumes first in the default focused model.
//
// Since: 1.0
func (p *CompletionPopup) HandleKey(e *event.KeyEvent) bool {
	if !p.shown {
		return false
	}
	return p.consumeKey(e)
}

// HandlesEscape reports whether Escape closes the completion list rather than
// the surface it is shown over. It does so whenever the list is up: the list is
// the innermost thing on screen.
//
// Since: 1.0
func (p *CompletionPopup) HandlesEscape() bool { return p.shown }

// consumeKey is the popup's key switch, shared by HandleKey (which gates it
// on visibility for hosts) and TypedKey (which only fires while focused).
func (p *CompletionPopup) consumeKey(e *event.KeyEvent) bool {
	switch e.Name {
	case event.KeyDown:
		p.moveSelection(1)
		return true
	case event.KeyUp:
		p.moveSelection(-1)
		return true
	case event.KeyEnter, event.KeyReturn, event.KeyTab:
		p.trigger(p.selected)
		return true
	case event.KeyEscape:
		p.dismiss()
		return true
	case event.KeyPageUp, event.KeyPageDown:
		// Without a documentation panel the host pages the editor caret as
		// usual.
		return p.scrollDocPage(e.Name == event.KeyPageDown)
	}
	return false
}

// TypedKey drives the popup: up/down move the selection, Enter/Tab accept and
// Escape dismisses. Any other key is forwarded to the host Entry (backspace,
// cursor movement, …) so editing continues; a standalone popup instead treats
// Space as accept.
func (p *CompletionPopup) TypedKey(e *event.KeyEvent) {
	if p.consumeKey(e) {
		return
	}
	if p.host != nil {
		p.host.TypedKey(e)
		return
	}
	if e.Name == event.KeySpace {
		p.trigger(p.selected)
	}
}

// Tapped accepts the item under the pointer.
func (p *CompletionPopup) Tapped(e *event.PointEvent) {
	if i := p.rowAt(e.Position); i >= 0 {
		p.trigger(i)
	}
}

// HoverIn highlights the item under the pointer.
func (p *CompletionPopup) HoverIn(e *event.HoverEvent) { p.hoverAt(e.Position) }

// HoverMoved highlights the item under the pointer.
func (p *CompletionPopup) HoverMoved(e *event.HoverEvent) { p.hoverAt(e.Position) }

// HoverOut hides the detail tooltip; the selection persists for keyboard use.
func (p *CompletionPopup) HoverOut() { p.hideDetail() }

func (p *CompletionPopup) hoverAt(pos geo.Position) {
	i := p.rowAt(pos)
	if i < 0 {
		return
	}
	if i != p.selected {
		p.selected = i
		p.Refresh()
	}
	p.refreshDetail()
}

func (p *CompletionPopup) moveSelection(delta int) {
	n := len(p.items)
	if n == 0 {
		return
	}
	p.selected = (p.selected + delta + n) % n
	p.scrollSelectionIntoView()
	p.Refresh()
	p.refreshDetail()
}

// visibleRows returns how many suggestion rows the popup shows at once: the
// whole list, bounded by the visible-row cap.
func (p *CompletionPopup) visibleRows() int {
	limit := p.MaxVisibleRows
	if limit <= 0 {
		limit = defaultMaxVisibleRows
	}
	if n := len(p.items); n < limit {
		return n
	}
	return limit
}

// clampTop bounds the window's first visible row so it always covers full rows.
func (p *CompletionPopup) clampTop() {
	if maxTop := len(p.items) - p.visibleRows(); p.top > maxTop {
		p.top = maxTop
	}
	if p.top < 0 {
		p.top = 0
	}
}

// scrollSelectionIntoView moves the window the shortest distance that brings
// the selected row inside it.
func (p *CompletionPopup) scrollSelectionIntoView() {
	if p.selected < p.top {
		p.top = p.selected
	} else if vis := p.visibleRows(); p.selected >= p.top+vis {
		p.top = p.selected - vis + 1
	}
	p.clampTop()
}

// hasDocs reports whether any item carries Documentation, which is what turns
// the side panel on for the whole popup.
func (p *CompletionPopup) hasDocs() bool {
	for i := range p.items {
		if p.items[i].Documentation != "" {
			return true
		}
	}
	return false
}

// docPanelWidth returns the width reserved for the documentation panel, or 0
// when no item has documentation and no panel is shown.
func (p *CompletionPopup) docPanelWidth() float32 {
	if !p.hasDocs() {
		return 0
	}
	w := float32(completionDocPanelWidth)
	if _, area := p.canvas.InteractiveArea(); area.Width/2 < w {
		w = area.Width / 2
	}
	return w
}

// scrollDocPage pages the documentation panel by (almost) one viewport height,
// keyed by PageUp/PageDown while the list keeps focus. It reports whether it
// consumed the key: false when no panel is visible, so the caller can forward
// the key to the host Entry instead.
func (p *CompletionPopup) scrollDocPage(down bool) bool {
	if p.docScroll == nil || p.docPanelWidth() == 0 {
		return false
	}
	viewport := p.docScroll.Size().Height
	maxOff := p.docScroll.Content.Size().Height - viewport
	if maxOff < 0 {
		maxOff = 0
	}
	step := viewport * 0.9
	if !down {
		step = -step
	}
	off := p.docScroll.Offset
	off.Y += step
	if off.Y > maxOff {
		off.Y = maxOff
	}
	if off.Y < 0 {
		off.Y = 0
	}
	p.docScroll.Offset = off
	p.docScroll.Refresh()
	return true
}

// Scrolled moves the visible window over a capped list with the mouse wheel or
// trackpad, banking sub-row deltas so slow trackpad scrolls still add up. The
// selection is left where it is; a row scrolled out of the window simply loses
// its highlight until it returns.
//
// Scrolled implements the [event.Scrollable] interface.
//
// Since: 1.0
func (p *CompletionPopup) Scrolled(e *event.ScrollEvent) {
	if len(p.items) <= p.visibleRows() {
		return
	}
	m := p.metrics()
	p.scrollAccum -= e.Scrolled.DY / m.rowHeight // wheel up (positive DY) reveals earlier rows
	rows := int(p.scrollAccum)
	if rows == 0 {
		return
	}
	p.scrollAccum -= float32(rows)
	p.top += rows
	p.clampTop()
	p.hideDetail() // the icon under the pointer no longer matches the timer's row
	p.Refresh()
}

func (p *CompletionPopup) trigger(i int) {
	if i < 0 || i >= len(p.items) {
		return
	}
	p.selected = i
	item := p.items[i]
	p.Hide()
	if p.OnSelected != nil {
		p.OnSelected(i, item)
	}
}

// --- detail tooltip (pre-rebrand tracker #362) ---
//
// When the active row carries a Detail string, its info icon reveals that text
// in a small inverted tooltip after a short delay, on both hover and keyboard
// selection. It reuses the standard tooltip delay and content layout.

// refreshDetail (re)schedules the detail tooltip for the active row, or clears
// it when that row has no detail.
func (p *CompletionPopup) refreshDetail() {
	row := p.selected
	hasDetail := row >= 0 && row < len(p.items) && p.items[row].Detail != ""
	if p.hasDocs() {
		hasDetail = false // the documentation panel subsumes the tooltip (pre-rebrand tracker #435)
	}

	p.detailMu.Lock()
	defer p.detailMu.Unlock()
	if !hasDetail {
		p.stopDetailLocked()
		return
	}
	if row == p.detailRow {
		return // already scheduled/showing for this row
	}
	p.stopDetailLocked()
	p.detailRow = row
	p.detailTimer = time.AfterFunc(tooltipDelay, func() {
		bellui.Do(func() { p.showDetail(row) })
	})
}

// showDetail displays the detail tooltip for a row, anchored under its info icon.
func (p *CompletionPopup) showDetail(row int) {
	if !p.Visible() || row < 0 || row >= len(p.items) {
		return
	}
	if row < p.top || row >= p.top+p.visibleRows() {
		return // scrolled out of the window since the tooltip was scheduled
	}
	detail := p.items[row].Detail
	if detail == "" {
		return
	}

	th := p.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	bg := canvas.NewRectangle(th.Color(theme.ColorNameInverseSurface, v))
	bg.CornerRadius = th.Size(theme.SizeNameShapeExtraSmall)
	fg := NewRichText(&TextSegment{Text: detail, Style: RichTextStyle{
		ColorName: theme.ColorNameInverseOnSurface,
		SizeName:  theme.SizeNameCaptionText,
	}})
	content := &bellui.Container{
		Objects: []core.CanvasObject{bg, fg},
		Layout:  &tooltipContentLayout{padding: th.Size(theme.SizeNamePadding)},
	}
	pop := newPopUpWithBgColor(content, p.canvas, th.Color(theme.ColorNameInverseSurface, v))

	r := p.infoIconRect(row)
	pos := p.Position().Add(r.Pos).Add(geo.NewPos(0, r.Size.Height))

	p.detailMu.Lock()
	p.stopDetailLocked()
	p.detailPopup = pop
	p.detailRow = row
	p.detailMu.Unlock()

	pop.ShowAtPosition(pos)
}

// hideDetail cancels any pending or visible detail tooltip.
func (p *CompletionPopup) hideDetail() {
	p.detailMu.Lock()
	p.stopDetailLocked()
	p.detailMu.Unlock()
}

// stopDetailLocked clears the timer and popup. Caller must hold detailMu.
func (p *CompletionPopup) stopDetailLocked() {
	if p.detailTimer != nil {
		p.detailTimer.Stop()
		p.detailTimer = nil
	}
	if p.detailPopup != nil {
		p.detailPopup.Hide()
		p.detailPopup = nil
	}
	p.detailRow = -1
}

// infoIconRect returns the info icon's widget-relative rectangle for a row, or
// the zero rectangle when that row shows no info icon. The icon sits on the
// trailing edge, which flips to the leading edge under RTL.
func (p *CompletionPopup) infoIconRect(row int) geo.Rectangle {
	if row < 0 || row >= len(p.items) || p.items[row].Detail == "" {
		return geo.Rectangle{}
	}
	if row < p.top || row >= p.top+p.visibleRows() {
		return geo.Rectangle{} // outside the scrolled window: not on screen
	}
	m := p.metrics()
	rtl := isRTLAt(p.super())
	listX0, listW := m.listArea(p.Size().Width, rtl)
	trailX := listX0 + listW - m.pad - m.iconSize
	if rtl {
		trailX = listX0 + m.pad
	}
	rowY := m.contentTop + float32(row-p.top)*m.rowHeight
	iconY := rowY + (m.rowHeight-m.iconSize)/2
	return geo.Rectangle{Pos: geo.NewPos(trailX, iconY), Size: geo.NewSize(m.iconSize, m.iconSize)}
}

// rowAt returns the item index at a widget-relative position, or -1. The
// position addresses the visible window, so the returned index is offset by
// the scrolled-away rows above it. A position inside the documentation panel
// is not a row.
func (p *CompletionPopup) rowAt(pos geo.Position) int {
	m := p.metrics()
	listX0, listW := m.listArea(p.Size().Width, isRTLAt(p.super()))
	if pos.X < listX0 || pos.X > listX0+listW {
		return -1
	}
	y := pos.Y - m.contentTop
	if y < 0 {
		return -1
	}
	i := int(y / m.rowHeight)
	if i >= p.visibleRows() {
		return -1
	}
	i += p.top
	if i < 0 || i >= len(p.items) {
		return -1
	}
	return i
}

func (p *CompletionPopup) adjustedPosition(pos geo.Position, size geo.Size) geo.Position {
	x, y := pos.X, pos.Y
	_, area := p.canvas.InteractiveArea()
	if x+size.Width > area.Width {
		if x = area.Width - size.Width; x < 0 {
			x = 0
		}
	}
	if y+size.Height > area.Height {
		if y = area.Height - size.Height; y < 0 {
			y = 0
		}
	}
	return geo.NewPos(x, y)
}

// popupMetrics holds the resolved sizing used by both layout and hit-testing.
type popupMetrics struct {
	pad        float32
	gap        float32
	textSize   float32
	iconSize   float32
	rowHeight  float32
	titleH     float32 // 0 when no title
	dividerH   float32 // 0 when no title
	contentTop float32 // y of the first row, relative to the widget
	docW       float32 // documentation panel width, 0 when no panel (pre-rebrand tracker #435)
}

// listArea returns the x origin and width of the suggestion-list portion of a
// popup of the given total width: everything, minus the documentation panel
// and its divider. The panel sits on the trailing edge, so under RTL the list
// area starts after the panel.
func (m popupMetrics) listArea(totalWidth float32, rtl bool) (x0, width float32) {
	if m.docW == 0 {
		return 0, totalWidth
	}
	width = totalWidth - m.docW - completionDocDividerWidth
	if rtl {
		x0 = m.docW + completionDocDividerWidth
	}
	return x0, width
}

func (p *CompletionPopup) metrics() popupMetrics {
	th := p.Theme()
	pad := th.Size(theme.SizeNameInnerPadding)
	gap := th.Size(theme.SizeNamePadding)
	textSize := th.Size(theme.SizeNameText)
	textH := bellui.MeasureText("Ag", textSize, bellui.TextStyle{}).Height

	m := popupMetrics{
		pad:       pad,
		gap:       gap,
		textSize:  textSize,
		iconSize:  textH,
		rowHeight: textH + gap,
	}
	if p.title != "" {
		m.titleH = bellui.MeasureText("Ag", textSize, bellui.TextStyle{}).Height + gap
		m.dividerH = 1
	}
	m.contentTop = pad + m.titleH + m.dividerH
	m.docW = p.docPanelWidth()
	return m
}

func (p *CompletionPopup) rowWidth(item CompletionItem, m popupMetrics) float32 {
	w := bellui.MeasureText(item.Text, m.textSize, bellui.TextStyle{}).Width
	if item.Icon != nil {
		w += m.iconSize + m.gap
	}
	if item.Detail != "" {
		w += m.gap + m.iconSize
	}
	return w
}

// CreateRenderer is private to Bellui.
func (p *CompletionPopup) CreateRenderer() core.WidgetRenderer {
	p.ExtendBaseWidget(p)
	th := p.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	bg := canvas.NewRectangle(th.Color(theme.ColorNameSurfaceContainerMedium, v))
	bg.CornerRadius = th.Size(theme.SizeNameShapeExtraSmall)
	bg.StrokeColor = th.Color(theme.ColorNameOutlineVariant, v)
	bg.StrokeWidth = 1
	sel := canvas.NewRectangle(color.Transparent)
	sel.CornerRadius = th.Size(theme.SizeNameShapeExtraSmall)

	bar := canvas.NewRectangle(th.Color(theme.ColorNameScrollBar, v))
	bar.CornerRadius = th.Size(theme.SizeNameScrollBarSmall) / 2

	if p.docRich == nil {
		p.docRich = NewRichText()
		p.docRich.Wrapping = bellui.TextWrapWord
		p.docScroll = intWidget.NewVScroll(p.docRich)
	}
	docDivider := canvas.NewRectangle(th.Color(theme.ColorNameOutlineVariant, v))

	r := &completionPopupRenderer{popup: p, bg: bg, sel: sel, bar: bar, docDivider: docDivider, docFor: -1}
	r.rebuild()
	return r
}

type completionRow struct {
	text *canvas.Text
	icon *canvas.Image // leading kind icon, or nil
	info *canvas.Image // trailing info icon, or nil
}

type completionPopupRenderer struct {
	popup   *CompletionPopup
	bg      *canvas.Rectangle
	sel     *canvas.Rectangle
	bar     *canvas.Rectangle // scroll-position indicator, hidden when all rows fit
	title   *canvas.Text
	divider *canvas.Rectangle
	rows    []completionRow

	// Documentation side panel (pre-rebrand tracker #435): a vertical rule and, on the popup
	// itself, a RichText in a scroll. docFor tracks which item's documentation
	// is currently rendered so selection moves re-fill it only when needed.
	docDivider *canvas.Rectangle
	docFor     int

	objects []core.CanvasObject
}

// rebuild recreates the per-item canvas objects when the item set changes.
func (r *completionPopupRenderer) rebuild() {
	p := r.popup
	th := p.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()
	textCol := th.Color(theme.ColorNameOnSurface, v)

	r.objects = []core.CanvasObject{r.bg, r.sel}

	r.title = nil
	r.divider = nil
	if p.title != "" {
		t := canvas.NewText(p.title, th.Color(theme.ColorNameOnSurfaceVariant, v))
		t.TextSize = th.Size(theme.SizeNameText)
		t.TextStyle = bellui.TextStyle{Bold: true}
		r.title = t
		r.divider = canvas.NewRectangle(th.Color(theme.ColorNameOutlineVariant, v))
		r.objects = append(r.objects, t, r.divider)
	}

	r.rows = make([]completionRow, len(p.items))
	for i, item := range p.items {
		row := completionRow{}
		row.text = canvas.NewText(item.Text, textCol)
		row.text.TextSize = th.Size(theme.SizeNameText)
		r.objects = append(r.objects, row.text)
		if item.Icon != nil {
			row.icon = canvas.NewImageFromResource(item.Icon)
			row.icon.FillMode = canvas.ImageFillContain
			r.objects = append(r.objects, row.icon)
		}
		if item.Detail != "" {
			row.info = canvas.NewImageFromResource(theme.InfoIcon())
			row.info.FillMode = canvas.ImageFillContain
			r.objects = append(r.objects, row.info)
		}
		r.rows[i] = row
	}
	r.objects = append(r.objects, r.bar)
	if p.docScroll != nil {
		r.objects = append(r.objects, r.docDivider, p.docScroll)
	}
	r.docFor = -1 // the new items' documentation must be re-rendered
}

func (r *completionPopupRenderer) Layout(size geo.Size) {
	p := r.popup
	m := p.metrics()
	rtl := lang.IsRTLDirection()

	r.bg.Resize(size)
	r.bg.Move(geo.NewPos(0, 0))

	if r.title != nil {
		r.title.Move(geo.NewPos(m.pad, m.pad))
		r.divider.Resize(geo.NewSize(size.Width, m.dividerH))
		r.divider.Move(geo.NewPos(0, m.pad+m.titleH))
	}

	textH := bellui.MeasureText("Ag", m.textSize, bellui.TextStyle{}).Height
	listX0, listW := m.listArea(size.Width, rtl)
	top, visible := p.top, p.visibleRows()
	for i, row := range r.rows {
		// Only the scrolled window's rows are laid out; the rest are hidden.
		if i < top || i >= top+visible {
			row.text.Hide()
			if row.icon != nil {
				row.icon.Hide()
			}
			if row.info != nil {
				row.info.Hide()
			}
			continue
		}
		row.text.Show()
		if row.icon != nil {
			row.icon.Show()
		}
		if row.info != nil {
			row.info.Show()
		}

		rowY := m.contentTop + float32(i-top)*m.rowHeight
		iconY := rowY + (m.rowHeight-m.iconSize)/2
		textY := rowY + (m.rowHeight-textH)/2

		// The leading edge holds the optional kind icon then the text; the
		// trailing edge holds the info icon. Both edges flip with the layout
		// direction, so in RTL the info icon leads and the text is right-aligned.
		// All edges are relative to the list area, which excludes the
		// documentation panel when one is shown.
		leadX := listX0 + m.pad
		trailX := listX0 + listW - m.pad - m.iconSize
		if rtl {
			leadX, trailX = listX0+listW-m.pad-m.iconSize, listX0+m.pad
		}

		if row.icon != nil {
			row.icon.Resize(geo.NewSize(m.iconSize, m.iconSize))
			row.icon.Move(geo.NewPos(leadX, iconY))
		}
		if rtl {
			tw := bellui.MeasureText(row.text.Text, m.textSize, bellui.TextStyle{}).Width
			tx := listX0 + listW - m.pad - tw
			if row.icon != nil {
				tx = leadX - m.gap - tw
			}
			row.text.Move(geo.NewPos(tx, textY))
		} else {
			tx := leadX
			if row.icon != nil {
				tx = leadX + m.iconSize + m.gap
			}
			row.text.Move(geo.NewPos(tx, textY))
		}

		if row.info != nil {
			row.info.Resize(geo.NewSize(m.iconSize, m.iconSize))
			row.info.Move(geo.NewPos(trailX, iconY))
		}
	}

	r.layoutSelection(size, m)
	r.layoutScrollBar(size, m)
	r.layoutDocPanel(size, m)
}

func (r *completionPopupRenderer) layoutSelection(size geo.Size, m popupMetrics) {
	p := r.popup
	sel := p.selected - p.top
	if len(r.rows) == 0 || sel < 0 || sel >= p.visibleRows() {
		r.sel.Hide() // no rows, or the selected row is scrolled out of the window
		return
	}
	r.sel.Show()
	listX0, listW := m.listArea(size.Width, lang.IsRTLDirection())
	rowY := m.contentTop + float32(sel)*m.rowHeight
	inset := m.gap / 2
	r.sel.Resize(geo.NewSize(listW-inset*2, m.rowHeight))
	r.sel.Move(geo.NewPos(listX0+inset, rowY))
}

// layoutScrollBar places the thin scroll-position indicator along the list's
// trailing edge (leading under RTL), proportional to the window within the
// full list.
func (r *completionPopupRenderer) layoutScrollBar(size geo.Size, m popupMetrics) {
	p := r.popup
	total, visible := len(p.items), p.visibleRows()
	if total <= visible {
		r.bar.Hide()
		return
	}
	r.bar.Show()

	barW := p.Theme().Size(theme.SizeNameScrollBarSmall)
	trackY := m.contentTop
	trackH := size.Height - m.contentTop - m.pad
	barH := trackH * float32(visible) / float32(total)
	barY := trackY + trackH*float32(p.top)/float32(total)

	rtl := lang.IsRTLDirection()
	listX0, listW := m.listArea(size.Width, rtl)
	x := listX0 + listW - barW - 1
	if rtl {
		x = listX0 + 1
	}
	r.bar.Resize(geo.NewSize(barW, barH))
	r.bar.Move(geo.NewPos(x, barY))
}

// layoutDocPanel places the documentation panel and its divider in the space
// the list area leaves free: the trailing side, which under RTL is the left.
func (r *completionPopupRenderer) layoutDocPanel(size geo.Size, m popupMetrics) {
	p := r.popup
	if m.docW == 0 || p.docScroll == nil {
		if r.docDivider != nil {
			r.docDivider.Hide()
		}
		if p.docScroll != nil {
			p.docScroll.Hide()
		}
		return
	}
	rtl := lang.IsRTLDirection()
	listX0, listW := m.listArea(size.Width, rtl)
	divX := listX0 + listW // LTR: rule right of the list
	panelX := divX + completionDocDividerWidth
	if rtl {
		divX = m.docW
		panelX = 0
	}

	r.docDivider.Show()
	r.docDivider.Resize(geo.NewSize(completionDocDividerWidth, size.Height-2*m.pad))
	r.docDivider.Move(geo.NewPos(divX, m.pad))

	p.docScroll.Show()
	p.docScroll.Move(geo.NewPos(panelX+m.pad, m.pad))
	p.docScroll.Resize(geo.NewSize(m.docW-2*m.pad, size.Height-2*m.pad))
}

func (r *completionPopupRenderer) MinSize() geo.Size {
	p := r.popup
	m := p.metrics()

	var widest float32
	for _, item := range p.items {
		if w := p.rowWidth(item, m); w > widest {
			widest = w
		}
	}
	if r.title != nil {
		if w := bellui.MeasureText(p.title, m.textSize, bellui.TextStyle{}).Width; w > widest {
			widest = w
		}
	}

	width := widest + m.pad*2
	if m.docW > 0 {
		width += completionDocDividerWidth + m.docW
	}
	height := m.contentTop + float32(p.visibleRows())*m.rowHeight + m.pad
	return geo.NewSize(width, height)
}

func (r *completionPopupRenderer) Refresh() {
	p := r.popup
	th := p.Theme()
	v := bellui.CurrentApp().Settings().ThemeVariant()

	if len(r.rows) != len(p.items) || (r.title == nil) != (p.title == "") {
		r.rebuild()
	}
	r.refreshDoc()

	r.bg.FillColor = th.Color(theme.ColorNameSurfaceContainerMedium, v)
	r.bg.StrokeColor = th.Color(theme.ColorNameOutlineVariant, v)
	r.bg.CornerRadius = th.Size(theme.SizeNameShapeExtraSmall)
	r.sel.FillColor = th.Color(theme.ColorNameSecondaryContainer, v)
	r.bar.FillColor = th.Color(theme.ColorNameScrollBar, v)
	r.bar.CornerRadius = th.Size(theme.SizeNameScrollBarSmall) / 2
	r.docDivider.FillColor = th.Color(theme.ColorNameOutlineVariant, v)

	onSurface := th.Color(theme.ColorNameOnSurface, v)
	onSelected := th.Color(theme.ColorNameOnSecondaryContainer, v)
	for i, row := range r.rows {
		if i == p.selected {
			row.text.Color = onSelected
		} else {
			row.text.Color = onSurface
		}
		row.text.Refresh()
	}

	// Outside a layout walk (typing, arrow keys), so lay out in the popup's
	// own direction (#696).
	inDirectionOf(p.super(), func() { r.Layout(p.Size()) })
	canvas.Refresh(p)
}

// refreshDoc re-fills the documentation panel when the selection moved to a
// different item: markdown documentation is parsed into rich segments,
// verbatim documentation becomes a single wrapped paragraph, and an item with
// no documentation leaves the panel empty. The panel scrolls back to the top
// for each newly selected item.
func (r *completionPopupRenderer) refreshDoc() {
	p := r.popup
	if p.docScroll == nil || p.docPanelWidth() == 0 {
		return
	}
	sel := p.selected
	if sel == r.docFor || sel < 0 || sel >= len(p.items) {
		return
	}
	r.docFor = sel

	var segments []RichTextSegment
	item := p.items[sel]
	if item.Documentation != "" {
		if item.DocumentationMarkdown {
			segments = NewRichTextFromMarkdown(item.Documentation).Segments
		} else {
			segments = []RichTextSegment{&TextSegment{Text: item.Documentation, Style: RichTextStyleParagraph}}
		}
	}
	p.docRich.Segments = segments
	p.docRich.Refresh()
	p.docScroll.Offset = geo.Position{}
	p.docScroll.Refresh()
}

func (r *completionPopupRenderer) Objects() []core.CanvasObject { return r.objects }

func (r *completionPopupRenderer) Destroy() {}
