package editor

import (
	"context"
	"image/color"
	"strings"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// HoverContent is what a [HoverProvider] returns for a document position. A
// provider supplies its body as markdown ([HoverContent.Markdown]) or as plain
// text ([HoverContent.PlainText]); both may be set (markdown shown first). When
// both are empty the card shows nothing (unless diagnostics contribute).
type HoverContent struct {
	// Markdown is rendered as markdown into the hover card.
	Markdown string
	// PlainText is rendered verbatim into the hover card, so a value containing
	// markdown-significant characters is not reinterpreted (see [MarkupKind]).
	PlainText string
	// Range, when non-empty, is outlined in the editor while the card is shown.
	Range TextRange
}

// HoverProvider supplies hover cards at a document position. It is optional and
// LSP-shaped but carries no LSP types — a host can back it with a symbol table,
// a static dictionary, or a language-server bridge.
//
// Hover is called on the UI goroutine after a short dwell and should return
// promptly.
type HoverProvider interface {
	Hover(ctx context.Context, row, col int) (*HoverContent, error)
}

// SetHoverProvider installs (or clears, with nil) the hover provider. With no
// provider set, editor behavior is unchanged.
func (e *Editor) SetHoverProvider(p HoverProvider) {
	e.hover.provider = p
	if p == nil {
		e.hover.dismiss()
	}
}

// ShowHoverAtCursor shows the hover card for the symbol at the current cursor
// position — the keyboard counterpart to mouse dwell, bound by default to the
// Ctrl+K Ctrl+I chord (⌘K ⌘I on macOS). It is a no-op when no hover content is
// available (no provider and no diagnostics). The card is dismissed on the next
// cursor move, edit, or Escape.
func (e *Editor) ShowHoverAtCursor() {
	if e.hover != nil {
		e.hover.invokeAtCursor()
	}
}

const (
	defaultHoverDelay = 450 * time.Millisecond
	// hoverGraceDelay bridges the token→card pointer transition: when the pointer
	// leaves the hovered token the card is dismissed after this delay unless the
	// pointer lands on the card (which cancels it). Without it the card would
	// flicker out in the 1–2px gap between the token and the card's edge.
	hoverGraceDelay = 150 * time.Millisecond
	maxHoverWidth   = 480
	// maxHoverHeight caps the card height; taller content scrolls inside it.
	maxHoverHeight = 320
)

// WordHoverProvider is a simple [HoverProvider] that looks up the identifier
// under the cursor in a markdown dictionary — handy for demos, tests, and
// keyword/builtin documentation.
type WordHoverProvider struct {
	// Docs maps an identifier to the markdown shown when it is hovered.
	Docs map[string]string
	// Text returns the current document text, used to find the hovered word.
	Text func() string
}

// Hover returns the documentation for the identifier under (row, col), outlining
// that identifier's range.
func (p *WordHoverProvider) Hover(_ context.Context, row, col int) (*HoverContent, error) {
	if p.Text == nil {
		return nil, nil
	}
	word, rng := wordAt(p.Text(), row, col)
	if word == "" {
		return nil, nil
	}
	if md, ok := p.Docs[word]; ok {
		return &HoverContent{Markdown: md, Range: rng}, nil
	}
	return nil, nil
}

// wordAt returns the identifier covering (row, col) and its range, scanning both
// directions over identifier characters. The word is empty when (row, col) is
// not on an identifier character.
func wordAt(text string, row, col int) (string, TextRange) {
	lines := strings.Split(text, "\n")
	if row < 0 || row >= len(lines) {
		return "", TextRange{}
	}
	runes := []rune(lines[row])
	if col < 0 || col > len(runes) {
		return "", TextRange{}
	}
	start, end := col, col
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	for end < len(runes) && isIdentRune(runes[end]) {
		end++
	}
	if start == end {
		return "", TextRange{}
	}
	return string(runes[start:end]), TextRange{StartRow: row, StartCol: start, EndRow: row, EndCol: end}
}

// hoverController owns hover state: the dwell timer, the markdown card popover,
// and the outlined range.
type hoverController struct {
	e        *Editor
	provider HoverProvider
	delay    time.Duration

	pop     *popover
	rich    *widget.RichText
	sizer   *fixedSizeLayout
	content *bellui.Container

	shownRange TextRange // document range the visible card describes
	hasRange   bool
	timer      *time.Timer

	overCard   bool        // pointer is currently over the card
	graceTimer *time.Timer // pending dismiss while bridging token→card
	// keyboardShown marks a card opened via ShowHoverAtCursor (no pointer anchor),
	// so it is dismissed on the next cursor move or edit rather than on move-away.
	keyboardShown bool
}

func newHoverController(e *Editor) *hoverController {
	return &hoverController{e: e, delay: defaultHoverDelay}
}

func (h *hoverController) visible() bool {
	return h.pop != nil && h.e.popoverShown(h.pop)
}

// onMouseMoved is called by the hover-capture layer for each pointer move
// (viewport coordinates). It fires only while the pointer is over the text (the
// card, when shown, is the topmost Hoverable and intercepts its own moves). It
// keeps the card while the pointer stays within the described range, arms a
// grace dismiss on move-away (so sliding onto the card cancels it), and (re)arms
// the dwell timer for a new token.
func (h *hoverController) onMouseMoved(pos geo.Position) {
	if !h.active() {
		return
	}
	row, col, ok := h.e.positionAtPoint(pos)
	if h.visible() {
		if ok && h.withinShown(row, col) {
			h.cancelGrace() // still over the hovered token — keep the card
			return
		}
		h.armGrace() // moved off the token: dismiss soon unless we land on the card
	}
	if ok {
		h.schedule(pos)
	}
}

// onMouseOut is called when the pointer leaves the text-capture layer — which
// includes moving onto the card. Arm a grace dismiss rather than closing
// immediately; the card's HoverIn cancels it, while leaving the editor lets it
// fire.
func (h *hoverController) onMouseOut() {
	h.cancelTimer()
	if h.visible() {
		h.armGrace()
		return
	}
	h.dismiss()
}

// onCardIn / onCardOut are driven by the Hoverable card wrapper: the pointer is
// over the card (keep it alive) or has left it (re-arm the grace dismiss).
func (h *hoverController) onCardIn() {
	h.overCard = true
	h.cancelGrace()
	h.cancelTimer() // suppress any pending dwell from the pre-card move
}

func (h *hoverController) onCardOut() {
	h.overCard = false
	h.armGrace()
}

// armGrace (re)starts the grace timer; on expiry the card is dismissed unless
// the pointer is over it.
func (h *hoverController) armGrace() {
	h.cancelGrace()
	h.graceTimer = time.AfterFunc(hoverGraceDelay, func() {
		bellui.Do(h.graceFire)
	})
}

// graceFire is the grace timer's action (split out for deterministic testing):
// dismiss the card unless the pointer has since landed on it.
func (h *hoverController) graceFire() {
	if !h.overCard {
		h.dismiss()
	}
}

func (h *hoverController) cancelGrace() {
	if h.graceTimer != nil {
		h.graceTimer.Stop()
		h.graceTimer = nil
	}
}

// withinShown reports whether (row, col) falls inside the visible card's range.
func (h *hoverController) withinShown(row, col int) bool {
	return h.hasRange && h.shownRange.Contains(row, col)
}

// active reports whether the hover path should respond to pointer dwell: either
// a host provider is installed, or there are diagnostics to surface.
func (h *hoverController) active() bool {
	return h.provider != nil || h.e.hasDiagnostics()
}

func (h *hoverController) schedule(pos geo.Position) {
	h.cancelTimer()
	h.timer = time.AfterFunc(h.delay, func() {
		// Fire on the UI goroutine — the timer runs on its own.
		bellui.Do(func() { h.triggerAt(pos) })
	})
}

func (h *hoverController) cancelTimer() {
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
}

// triggerAt maps a pointer position to a document cell and shows the card there
// (the mouse-dwell path).
func (h *hoverController) triggerAt(pos geo.Position) {
	row, col, ok := h.e.positionAtPoint(pos)
	if !ok {
		return
	}
	h.triggerAtRowCol(row, col, false)
}

// invokeAtCursor shows the card for the symbol at the cursor (the keyboard
// path — see [Editor.ShowHoverAtCursor]).
func (h *hoverController) invokeAtCursor() {
	h.triggerAtRowCol(h.e.entry.CursorRow, h.e.entry.CursorColumn, true)
}

// triggerAtRowCol gathers the diagnostics and the host provider's content for
// (row, col) and shows a combined card. keyboard marks a card opened from the
// keyboard (dismissed on cursor move/edit rather than pointer move-away).
func (h *hoverController) triggerAtRowCol(row, col int, keyboard bool) {
	if !h.active() {
		return
	}
	// Don't contend with the interactive popover (e.g. an open completion list);
	// the signature companion is fine to show alongside.
	if p := h.e.interactivePopover(); p != nil && p != h.pop {
		return
	}

	diags := h.e.diagnosticsAt(row, col)
	var content *HoverContent
	if h.provider != nil {
		// A provider error is non-fatal — fall back to whatever diagnostics give.
		if c, err := h.provider.Hover(context.Background(), row, col); err == nil {
			content = c
		}
	}

	segs := hoverCardSegments(diags, content)
	if len(segs) == 0 {
		return
	}

	h.ensureUI()
	h.rich.Segments = segs
	h.rich.Refresh()
	h.resize()

	// The card keeps showing while the pointer stays within this range.
	// Diagnostics own the squiggle, so prefer their range; else the provider's,
	// else the single hovered cell.
	h.shownRange = hoverShownRange(diags, content, row, col)
	h.hasRange = true

	// Only outline an explicit provider range — diagnostics already render a
	// squiggle over their range, so a second box would be redundant.
	if len(diags) == 0 && content != nil {
		h.setOutline(content.Range)
	} else {
		h.clearOutline()
	}

	h.keyboardShown = keyboard
	h.e.showPopoverAt(h.pop, h.shownRange.StartRow, h.shownRange.StartCol)
}

// hoverCardSegments builds the card body as rich-text segments: a diagnostics
// block first, then the provider's markdown, then the provider's plain text,
// each present block separated by a divider. Rendering as segments (rather than
// composing a markdown string) lets diagnostic messages and the plain-text body
// show verbatim while the markdown body is still parsed (pre-rebrand tracker #92, pre-rebrand tracker #90). The result
// is empty when there is nothing to show.
func hoverCardSegments(diags []Diagnostic, content *HoverContent) []widget.RichTextSegment {
	var blocks [][]widget.RichTextSegment
	if d := diagnosticSegments(diags); len(d) > 0 {
		blocks = append(blocks, d)
	}
	if content != nil {
		if content.Markdown != "" {
			blocks = append(blocks, markupSegments(content.Markdown, MarkupMarkdown))
		}
		if content.PlainText != "" {
			blocks = append(blocks, markupSegments(content.PlainText, MarkupPlainText))
		}
	}

	var segs []widget.RichTextSegment
	for i, b := range blocks {
		if i > 0 {
			segs = append(segs, markupSegments("---", MarkupMarkdown)...) // divider
		}
		segs = append(segs, b...)
	}
	// Drop a trailing newline terminator (see diagnosticSegments) so a
	// diagnostics-only card has no blank line at the bottom.
	if n := len(segs); n > 0 {
		if ts, ok := segs[n-1].(*widget.TextSegment); ok && ts.Text == "\n" {
			segs = segs[:n-1]
		}
	}
	return segs
}

// diagnosticSegments renders each diagnostic as its message (verbatim plain text,
// since an LSP diagnostic message is plain text — pre-rebrand tracker #92) followed by its dimmed
// "(source)" annotation, one diagnostic per line.
//
// The message and source are inline so they share a line, and each diagnostic
// ends with an explicit newline. This matters for layout, not just aesthetics: an
// inline run that is *not* newline-terminated before the following block (the
// "---" divider or the provider content) is drawn on the same row as that block
// and overlaps it (a fyne RichText quirk — see hoverCardSegments' trailing-newline
// trim and the regression test).
func diagnosticSegments(diags []Diagnostic) []widget.RichTextSegment {
	var segs []widget.RichTextSegment
	for _, d := range diags {
		segs = append(segs, &widget.TextSegment{
			Text:  d.Message,
			Style: widget.RichTextStyle{Inline: true},
		})
		if d.Source != "" {
			segs = append(segs, &widget.TextSegment{
				Text: "  (" + d.Source + ")",
				Style: widget.RichTextStyle{
					Inline:    true,
					ColorName: theme.ColorNamePlaceHolder,
					TextStyle: bellui.TextStyle{Italic: true},
				},
			})
		}
		segs = append(segs, &widget.TextSegment{Text: "\n", Style: widget.RichTextStyle{Inline: true}})
	}
	return segs
}

// hoverShownRange picks the range that keeps the card alive while the pointer
// stays within it: the bounding union of every matched diagnostic's (widened)
// range plus the provider's explicit range, so the card persists across the whole
// hovered region even when diagnostics of different extents overlap (pre-rebrand tracker #58). With
// no diagnostics and no provider range it falls back to the single hovered cell.
func hoverShownRange(diags []Diagnostic, content *HoverContent, row, col int) TextRange {
	var u TextRange // empty; Union ignores empty operands
	for _, d := range diags {
		u = u.Union(pointWidened(d.Range))
	}
	if content != nil {
		u = u.Union(content.Range)
	}
	if u.IsEmpty() {
		return TextRange{StartRow: row, StartCol: col, EndRow: row, EndCol: col + 1}
	}
	return u
}

func (h *hoverController) ensureUI() {
	if h.pop != nil {
		return
	}
	h.rich = widget.NewRichTextFromMarkdown("")
	h.rich.Wrapping = bellui.TextWrapWord
	h.sizer = &fixedSizeLayout{}
	// Wrap the markdown in a vertical scroll (so a tall card scrolls instead of
	// being clipped) inside a Hoverable card wrapper. Being the topmost Hoverable
	// over its area, the wrapper intercepts pointer moves the text-capture layer
	// would otherwise treat as "moved away" — keeping the card alive so it can be
	// scrolled. It is deliberately not Tappable/Focusable, so taps fall through
	// to the Entry and focus is untouched.
	scroll := container.NewVScroll(h.rich)
	card := newHoverCard(scroll, h.onCardIn, h.onCardOut)
	h.content = container.New(h.sizer, card)
	h.pop = newPopover(h.content)
	h.pop.maxWidth = maxHoverWidth
	h.pop.maxHeight = maxHoverHeight
	h.pop.interactive = true // an interactive card; it shouldn't share with completion
	h.pop.onDismiss = func() {
		h.hasRange = false
		h.overCard = false
		h.keyboardShown = false
		h.cancelGrace()
		h.clearOutline()
	}
}

// resize sizes the card to its wrapped markdown, capped at maxHoverWidth.
func (h *hoverController) resize() {
	natural := naturalRichSize(h.rich) // unwrapped width; see naturalRichSize
	w := natural.Width
	if w > maxHoverWidth {
		w = maxHoverWidth
	}
	// Constrain the width so wrapped height is measured at the final width.
	h.rich.Resize(geo.NewSize(w, natural.Height))
	height := h.rich.MinSize().Height
	if height > maxHoverHeight {
		height = maxHoverHeight // taller content scrolls inside the card
	}
	h.sizer.size = geo.NewSize(w, height)
	h.content.Refresh()
}

func (h *hoverController) setOutline(rng TextRange) {
	if rng.IsEmpty() || h.e.hoverOutline == nil {
		h.clearOutline()
		return
	}
	h.e.hoverOutline.SpansList = []DecoratedSpan{{
		Range: rng,
		Kind:  SpanBox,
		Color: themeColor(theme.ColorNamePrimary),
	}}
	h.e.RefreshDecorators()
}

func (h *hoverController) clearOutline() {
	if h.e.hoverOutline != nil && h.e.hoverOutline.SpansList != nil {
		h.e.hoverOutline.SpansList = nil
		h.e.RefreshDecorators()
	}
}

// dismiss hides the card if shown and clears any outline.
func (h *hoverController) dismiss() {
	h.cancelGrace()
	if h.visible() {
		h.e.dismissPopoverObj(h.pop) // fires pop.onDismiss (resets the rest)
		return
	}
	h.hasRange = false
	h.overCard = false
	h.keyboardShown = false
	h.clearOutline()
}

// hoverCard wraps the card body in a Hoverable widget so the controller can keep
// the card alive while the pointer is over it (and the inner scroll handles the
// wheel). It implements only Hoverable — not Tappable/Focusable — so taps fall
// through to the Entry and keyboard focus is unaffected.
type hoverCard struct {
	widget.BaseWidget
	body  core.CanvasObject
	onIn  func()
	onOut func()
}

func newHoverCard(body core.CanvasObject, onIn, onOut func()) *hoverCard {
	c := &hoverCard{body: body, onIn: onIn, onOut: onOut}
	c.ExtendBaseWidget(c)
	return c
}

func (c *hoverCard) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(c.body)
}

func (c *hoverCard) HoverIn(*event.HoverEvent)    { c.onIn() }
func (c *hoverCard) HoverMoved(*event.HoverEvent) {}
func (c *hoverCard) HoverOut()                    { c.onOut() }

// hoverCapture is a transparent overlay above the text that receives pointer
// moves so the editor can detect hover dwell and move-away. It implements only
// Hoverable (and Cursorable, to keep the text I-beam) — taps, drags, and
// selection fall through to the Entry below.
type hoverCapture struct {
	widget.BaseWidget
	onMove func(geo.Position)
	onOut  func()
	cursor func() desktop.Cursor
}

func newHoverCapture(onMove func(geo.Position), onOut func(), cursor func() desktop.Cursor) *hoverCapture {
	h := &hoverCapture{onMove: onMove, onOut: onOut, cursor: cursor}
	h.ExtendBaseWidget(h)
	return h
}

func (h *hoverCapture) CreateRenderer() core.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (h *hoverCapture) HoverIn(e *event.HoverEvent)    { h.onMove(e.Position) }
func (h *hoverCapture) HoverMoved(e *event.HoverEvent) { h.onMove(e.Position) }
func (h *hoverCapture) HoverOut()                      { h.onOut() }

// Cursor delegates to the editor's cursor function: a pointer over a lit link,
// otherwise the text I-beam (this layer sits above the Entry and would otherwise
// show the default arrow). The driver re-queries it on every pointer move.
func (h *hoverCapture) Cursor() desktop.Cursor { return h.cursor() }

// hoverCapLayout hosts the hover-capture layer inset by the scroll bar
// strips: fyne places the Entry's vertical bar flush to the right edge and
// the horizontal bar along the bottom, and hover goes to the topmost
// hoverable — a full-size capture layer would starve the bar of the hover
// that expands it to its grabbable width (fycoder pre-rebrand tracker #151). Nothing is lost by
// the inset: with ReserveBarSpace the text never occupies the vertical
// strip, and the bottom strip is the bar's own hit area.
type hoverCapLayout struct{}

func (hoverCapLayout) Layout(objs []core.CanvasObject, size geo.Size) {
	bar := theme.ScrollBarSize()
	for _, o := range objs {
		o.Move(geo.Position{})
		o.Resize(geo.NewSize(size.Width-bar, size.Height-bar))
	}
}

func (hoverCapLayout) MinSize([]core.CanvasObject) geo.Size { return geo.Size{} }
