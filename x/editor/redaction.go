package editor

import "github.com/belluikit/bellui/widget"

// RedactionMode selects whether redacted spans are hidden behind opaque bars or
// shown in the clear.
type RedactionMode int

const (
	// Masked hides each redacted span behind an opaque bar and bars the caret
	// from entering it. This is the zero value, so redactions are masked by
	// default.
	Masked RedactionMode = iota
	// Revealed shows redacted spans in the clear: bars are removed and the
	// caret-bar is lifted. Edit-protection is not implied by reveal — a host
	// that wants a revealed span to stay read-only also protects it via
	// [Editor.SetProtectedRanges].
	Revealed
)

// SetRedactions replaces the editor's redacted spans — secret regions to mask
// behind opaque bars. When the mode is [Masked] each span is covered by an
// opaque bar and the caret is barred from entering it (arrow navigation, clicks
// and taps skip over it). A masked span is atomic: edits inside it and inserts
// at either edge are vetoed silently, so nothing can shift the secret out from
// under its bar. When [Revealed] the spans are shown in the clear.
//
// Redaction (visual masking) and protection (edit-rejection via
// [Editor.SetProtectedRanges]) are orthogonal: they coincide for secrets but a
// host may reveal-but-keep-protected, or mask without separately protecting
// (masking already vetoes edits silently while masked).
//
// Spans are stored as (row, col) and anchor-tracked through edits (pre-rebrand tracker #130), like
// protected ranges: an insert or delete above a span shifts it (bar and
// caret-barring included) with no host re-publish; a programmatic
// [Editor.SetText] is a document swap and leaves spans as published. Passing
// nil or an empty slice clears all redactions.
func (e *Editor) SetRedactions(ranges []TextRange) {
	e.redaction.setRanges(ranges)
}

// SetRedactionMode flips between [Masked] and [Revealed]. It is a pure
// re-render plus a caret-bar recompute — it never mutates the document and never
// fires [Editor.OnChanged].
func (e *Editor) SetRedactionMode(m RedactionMode) {
	e.redaction.setMode(m)
}

// RedactionMode returns the current redaction mode.
func (e *Editor) RedactionMode() RedactionMode {
	return e.redaction.mode
}

// Redactions returns a copy of the current redaction spans, in the order they
// were last set, as anchor-tracking has shifted them through edits since
// (#130). It returns nil when none are set.
//
// This is the read-back for hosts that derive per-span state from the spans —
// a reveal-this-secret toggle, a re-locate of secrets before save: the host's
// own copy of what it passed to [Editor.SetRedactions] goes stale with every
// edit, while these are the spans the editor is actually drawing (#158). The
// symmetric accessor to [Editor.ProtectedRanges].
func (e *Editor) Redactions() []TextRange {
	if len(e.redaction.ranges) == 0 {
		return nil
	}
	return append([]TextRange{}, e.redaction.ranges...)
}

// redactionController owns the redacted spans and the mask/reveal mode. It is an
// internal [SpanDecorator] (drawing opaque bars when masked) and the owner of
// the Entry's CaretOpaque ProtectedRanges — the field reserved for caret-barring
// masked regions. The plain edit-rejection callback path (pre-rebrand tracker #63) goes through
// entry.BeforeChange instead; the two mechanisms are independent so redaction
// and protection compose.
type redactionController struct {
	e      *Editor
	ranges []TextRange
	mode   RedactionMode
}

func newRedactionController(e *Editor) *redactionController {
	return &redactionController{e: e}
}

// Spans implements [SpanDecorator]: one opaque bar per redaction when masked,
// nothing when revealed.
func (c *redactionController) Spans() []DecoratedSpan {
	if c.mode != Masked || len(c.ranges) == 0 {
		return nil
	}
	col := themeColor(ColorNameRedaction)
	spans := make([]DecoratedSpan, 0, len(c.ranges))
	for _, r := range c.ranges {
		spans = append(spans, DecoratedSpan{
			Range:  r,
			Kind:   SpanBackground,
			Opaque: true,
			Color:  col,
		})
	}
	return spans
}

func (c *redactionController) setRanges(ranges []TextRange) {
	if len(ranges) == 0 {
		c.ranges = nil
	} else {
		c.ranges = append([]TextRange{}, ranges...)
	}
	c.apply()
}

func (c *redactionController) setMode(m RedactionMode) {
	if m == c.mode {
		return
	}
	c.mode = m
	c.apply()
}

// apply pushes the recomputed caret-opaque ranges to the Entry and repaints the
// bars. It performs no document mutation and fires no OnChanged.
func (c *redactionController) apply() {
	c.reconcile()
	c.e.RefreshDecorators()
}

// allow is the redaction half of the Entry's BeforeChange gate, consulted after
// the protected controller. Each masked span is ATOMIC: the fork's CaretOpaque
// ProtectedRanges already vetoes edits strictly inside, but it permits boundary
// inserts (matching the protected-ranges design where edges stay appendable). A
// masked secret can't afford that — inserting at the leading edge shifts the
// secret out from under the opaque bar one keystroke at a time, revealing it
// (pre-rebrand tracker #108). So this vetoes any edit that touches a masked span's interior OR
// either edge, keeping the bar aligned to the secret. The veto is silent;
// redaction never fires OnProtectedEditAttempt. Returns true (allowing the edit)
// when revealed or when no spans are set.
func (c *redactionController) allow(sel widget.TextRange) bool {
	if c.mode != Masked || len(c.ranges) == 0 {
		return true
	}
	// Synchronise deferred edits (pre-rebrand tracker #129): sel indexes the live pre-edit content,
	// so the spans must be flattened against that same text.
	text := c.e.docText()
	for _, r := range c.ranges {
		s, e := rangeToOffsets(text, r)
		if editTouchesRangeAtomic(sel.Start, sel.End, s, e) {
			return false
		}
	}
	return true
}

// reconcile re-derives the Entry's CaretOpaque ProtectedRanges from the current
// redactions and mode against the current text. It is called on every text
// change, after anchor-tracking has shifted the spans themselves (pre-rebrand tracker #130), so the
// caret-bar offsets always match the spans' current homes.
func (c *redactionController) reconcile() {
	c.e.entry.ProtectedRanges = c.protectedRanges()
}

// protectedRanges derives the fork's CaretOpaque ranges for the masked spans
// against the current text. Revealed (or no spans) yields none, lifting the
// caret-bar.
func (c *redactionController) protectedRanges() []widget.ProtectedRange {
	if c.mode != Masked || len(c.ranges) == 0 {
		return nil
	}
	text := c.e.docText()
	out := make([]widget.ProtectedRange, 0, len(c.ranges))
	for _, r := range c.ranges {
		s, e := rangeToOffsets(text, r)
		if s >= e {
			continue // an empty span bars nothing
		}
		out = append(out, widget.ProtectedRange{Start: s, End: e, CaretOpaque: true})
	}
	return out
}
