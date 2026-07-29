package editor

import (
	"sort"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// SemanticToken is a single absolute-positioned foreground styling range from a
// whole-document analysis — typically decoded from an LSP semantic-tokens stream
// against its legend. It is the unit fed to [Editor.SetSemanticTokens].
//
// Coordinates are rune-based, matching the rest of the editor: Row and Col are
// zero-based, and Length counts runes on that row. A token never spans rows.
// Style names the token's visual class (the token-type name from the host's
// legend, e.g. "type", "function", "annotation"); it is resolved to a color
// through the editor's semantic style map (see [Editor.SetSemanticStyles] and
// [DefaultSemanticStyles]). A Style with no mapping leaves the run unstyled, so
// the line [Highlighter]'s color shows through.
type SemanticToken struct {
	// Row is the zero-based line the token sits on.
	Row int
	// Col is the zero-based rune column where the token starts.
	Col int
	// Length is the token's length in runes (on Row). Non-positive is ignored.
	Length int
	// Style is the token-type name resolved via the editor's semantic style map.
	Style string
	// Modifiers are the token's modifier names (as decoded from the legend's
	// modifier set, e.g. "declaration", "deprecated", "readonly"). Each is
	// resolved to a text style via the editor's modifier style map (see
	// [Editor.SetSemanticModifierStyles]) and composed onto the run — so a
	// declaration can read bold and a deprecated symbol strike-through, on top
	// of the type color. Unmapped modifiers are ignored.
	Modifiers []string
}

// SemanticModifierStyle is the text styling a semantic-token modifier
// contributes (see [Editor.SetSemanticModifierStyles]). The flags only ever
// switch styling on, so several modifiers on one token compose (bold + italic);
// they never clear the token-type color or the line highlighter's own bold /
// background / squiggle.
type SemanticModifierStyle struct {
	// Bold renders the token in bold weight.
	Bold bool
	// Italic renders the token in italic.
	Italic bool
	// Underline draws an underline (UnderlineStyle selects its shape).
	Underline bool
	// UnderlineStyle is the underline shape when Underline is set.
	UnderlineStyle UnderlineStyle
	// StrikeThrough draws a line through the token — the conventional rendering
	// for a deprecated symbol. The strike takes the token's foreground color.
	StrikeThrough bool
}

// DefaultSemanticModifierStyles returns the built-in mapping from LSP semantic
// token-modifier names to text styles: declarations and definitions read bold,
// deprecated symbols strike-through. A host may pass a customized copy to
// [Editor.SetSemanticModifierStyles]; modifiers absent from the map contribute
// no styling.
func DefaultSemanticModifierStyles() map[string]SemanticModifierStyle {
	return map[string]SemanticModifierStyle{
		"declaration": {Bold: true},
		"definition":  {Bold: true},
		"deprecated":  {StrikeThrough: true},
	}
}

// DefaultSemanticStyles returns the built-in mapping from LSP semantic
// token-type names to fycoder theme color names. It covers the standard LSP
// token types; a host may pass a customized copy to [Editor.SetSemanticStyles].
// Token types absent from the map render unstyled (the line highlighter's color
// is kept).
//
// "property" and "enumMember" reuse the function and builtin colors so the
// overlay agrees with the pxf highlighter, which colors field/assignment keys
// as functions and enum values as builtins (see highlight/pxf). "variable" and
// "parameter" map to the plain theme foreground (the Monokai convention),
// "regexp" to the string color, and "event" to the function color.
func DefaultSemanticStyles() map[string]bellui.ThemeColorName {
	return map[string]bellui.ThemeColorName{
		"namespace":     ColorNameSyntaxType,
		"type":          ColorNameSyntaxType,
		"class":         ColorNameSyntaxType,
		"enum":          ColorNameSyntaxType,
		"interface":     ColorNameSyntaxType,
		"struct":        ColorNameSyntaxType,
		"typeParameter": ColorNameSyntaxType,
		"function":      ColorNameSyntaxFunction,
		"method":        ColorNameSyntaxFunction,
		"event":         ColorNameSyntaxFunction,
		"property":      ColorNameSyntaxFunction,
		"enumMember":    ColorNameSyntaxBuiltin,
		"keyword":       ColorNameSyntaxKeyword,
		"modifier":      ColorNameSyntaxKeyword,
		"comment":       ColorNameSyntaxComment,
		"string":        ColorNameSyntaxString,
		"regexp":        ColorNameSyntaxString,
		"number":        ColorNameSyntaxNumber,
		"operator":      ColorNameSyntaxOperator,
		"decorator":     ColorNameSyntaxAnnotation,
		"annotation":    ColorNameSyntaxAnnotation,
		"macro":         ColorNameSyntaxAnnotation,
		"variable":      theme.ColorNameForeground,
		"parameter":     theme.ColorNameForeground,
	}
}

// semanticOverlay holds the published whole-document foreground overlay and
// builds the per-row [widget.StyleRun] tiling the fork composites over the line
// highlighter (fork StyleOverlayForRow, trendvidia/fyne#1079). It is pull-based:
// the fork calls runsForRow only for the rows it materialises, so cost is
// O(tokens on the visible rows), not O(document).
type semanticOverlay struct {
	// server is the last set published by the host (full or after a delta),
	// exactly as received — never anchor-shifted. Deltas (#186) apply against it,
	// so an index-based edit computed by the server against its last result still
	// lands on the right token even after local edits have moved/dropped tokens in
	// the rendered set (#188).
	server []SemanticToken
	// tokens is the rendered set: the server set plus the anchor shifts applied
	// since it was published (#179). It drives byRow and is what the getter
	// returns.
	tokens    []SemanticToken         // rendered set (byRow is derived from it)
	byRow     map[int][]SemanticToken // rendered tokens per row, sorted by Col
	styles    map[string]bellui.ThemeColorName
	modStyles map[string]SemanticModifierStyle
}

// publish sets both the un-shifted delta base (server) and the rendered set to
// tokens — used whenever the host hands over a fresh full or delta-derived set,
// which reflects the current document and so starts with no pending anchor
// shifts.
func (o *semanticOverlay) publish(tokens []SemanticToken) {
	o.server = append([]SemanticToken(nil), tokens...)
	o.setTokens(tokens)
}

func newSemanticOverlay() *semanticOverlay {
	return &semanticOverlay{
		styles:    DefaultSemanticStyles(),
		modStyles: DefaultSemanticModifierStyles(),
	}
}

// setTokens replaces the rendered set (o.tokens) and its per-row buckets (sorted
// by column) for O(1) per-row lookup at render time. It does not touch the
// delta base (o.server); anchor shifts go through here so they move the rendered
// tokens without re-indexing the base a future delta applies against.
func (o *semanticOverlay) setTokens(tokens []SemanticToken) {
	o.tokens = append(o.tokens[:0:0], tokens...)
	if len(tokens) == 0 {
		o.byRow = nil
		return
	}
	byRow := make(map[int][]SemanticToken)
	for _, t := range tokens {
		if t.Length <= 0 || t.Row < 0 || t.Col < 0 {
			continue
		}
		byRow[t.Row] = append(byRow[t.Row], t)
	}
	for _, ts := range byRow {
		sort.SliceStable(ts, func(i, j int) bool { return ts[i].Col < ts[j].Col })
	}
	o.byRow = byRow
}

// shift re-anchors the published tokens through the edit that turned oldText
// into newText, so the overlay follows the text between analyze passes (#179).
// Returns whether anything changed. It reuses the same splice recovery as the
// fold/protected/redaction anchor tracking (see anchor.go).
func (o *semanticOverlay) shift(oldText, newText string) bool {
	if len(o.tokens) == 0 {
		return false
	}
	shifted, dirty := shiftSemanticTokens(o.tokens, oldText, newText)
	if dirty {
		o.setTokens(shifted)
	}
	return dirty
}

// shiftSemanticTokens maps each token's (Row, Col, Length) through the splice
// turning oldText into newText. A token before the edit is untouched, one after
// it shifts, and one the edit reaches into resizes; a token the edit deletes or
// splits across a row boundary is dropped (it re-appears on the next analyze
// publish). dirty reports whether anything differs from the input.
func shiftSemanticTokens(tokens []SemanticToken, oldText, newText string) ([]SemanticToken, bool) {
	es, eeOld, eeNew, changed := spliceBounds([]rune(oldText), []rune(newText))
	if !changed {
		return tokens, false
	}
	out := make([]SemanticToken, 0, len(tokens))
	dirty := false
	for _, t := range tokens {
		if t.Length <= 0 || t.Row < 0 || t.Col < 0 {
			out = append(out, t) // invalid token: left as published
			continue
		}
		s := rowColToOffset(oldText, t.Row, t.Col)
		ns, ne, keep := shiftSpan(s, s+t.Length, es, eeOld, eeNew)
		if !keep || ne <= ns {
			// The deletion swallowed the token, or resized it to nothing (an edit
			// flush with both edges); drop it until the next analyze publish.
			dirty = true
			continue
		}
		nr, nc := offsetToRowCol(newText, ns)
		if er, _ := offsetToRowCol(newText, ne); er != nr {
			dirty = true // an edit split the token across a row boundary
			continue
		}
		if nr != t.Row || nc != t.Col || ne-ns != t.Length {
			dirty = true
		}
		t.Row, t.Col, t.Length = nr, nc, ne-ns
		out = append(out, t)
	}
	return out, dirty
}

// runsForRow builds the overlay tiling for one display row. The returned runs
// tile the row from rune 0 by rune length, using zero-valued spacer runs for
// gaps (and unmapped tokens), so the fork composites only the mapped token
// ranges as foreground. Returns nil when the row carries no styled overlay.
//
// This is wired to the fork's Entry.StyleOverlayForRow; the fork only reads each
// run's rune length (it never changes the text), so spacer/token content is a
// length carrier built with [strings.Repeat].
func (o *semanticOverlay) runsForRow(row int) []widget.StyleRun {
	toks := o.byRow[row]
	if len(toks) == 0 {
		return nil
	}
	var runs []widget.StyleRun
	cursor := 0 // runes already tiled from the row start
	for _, t := range toks {
		if t.Col < cursor {
			continue // overlaps an earlier token — keep the tiling monotonic
		}
		// Resolve the token-type color and any modifier styling. A bare
		// ForegroundColor merges over the lexer run without touching its
		// weight/underline/background (fork mergeStyleRun), so semantic color
		// overrides lexical while a diagnostic squiggle or bold keyword survives
		// underneath. Modifier flags only ever switch styling on, so they compose
		// over the type color and the lexer's own attributes.
		run := widget.StyleRun{}
		styled := false
		if name, ok := o.styles[t.Style]; ok {
			run.ForegroundColor = themeColor(name)
			styled = true
		}
		for _, m := range t.Modifiers {
			ms, ok := o.modStyles[m]
			if !ok {
				continue
			}
			if ms.Bold {
				run.Bold = true
			}
			if ms.Italic {
				run.Italic = true
			}
			if ms.Underline {
				run.Underline = true
				run.UnderlineStyle = ms.UnderlineStyle.toWidget()
			}
			if ms.StrikeThrough {
				run.StrikeThrough = true
			}
			styled = true
		}
		if !styled {
			continue // neither type nor modifiers styled this token
		}
		if t.Col > cursor {
			runs = append(runs, widget.StyleRun{Text: strings.Repeat(" ", t.Col-cursor)})
		}
		run.Text = strings.Repeat(" ", t.Length)
		runs = append(runs, run)
		cursor = t.Col + t.Length
	}
	return runs
}

// SetSemanticTokens replaces the editor's whole-document foreground overlay: a
// set of absolute (row, col, length, type) styled ranges — typically decoded
// from an LSP semantic-tokens stream against its legend — composited over the
// line [Highlighter]'s output. A token's foreground overrides the lexer's color
// for the covered runes while leaving the lexer's weight, background and
// underline (e.g. a diagnostic squiggle) intact.
//
// The overlay is pure composition: it never re-runs the line highlighter and
// never disturbs its incremental cache, so this is cheap to call on every
// analyze-completion notification. Token types with no entry in the semantic
// style map (see [Editor.SetSemanticStyles]) render unstyled, so the lexer
// color shows through. Coordinates are document coordinates and are used as
// published: after an edit the tokens are stale until the host republishes,
// matching the LSP semantic-tokens model. Passing nil or an empty slice clears
// the overlay.
//
// This set also becomes the base a subsequent [Editor.ApplySemanticTokenEdits]
// delta applies against; that base is held as published (never anchor-shifted),
// so a full publish here resets both the base and the rendered overlay.
func (e *Editor) SetSemanticTokens(tokens []SemanticToken) {
	e.semantic.publish(tokens)
	e.entry.Refresh()
}

// SemanticTokenEdit is a splice on the editor's current semantic-token list: it
// removes DeleteCount tokens starting at index Start, then inserts Insert in
// their place. It is the decoded-token analogue of an LSP
// semanticTokens/full/delta edit — an incremental update to the last published
// set (see [Editor.ApplySemanticTokenEdits]).
type SemanticTokenEdit struct {
	// Start is the index into the current token list where the splice begins.
	Start int
	// DeleteCount is how many existing tokens the splice removes at Start.
	DeleteCount int
	// Insert is the tokens to put in their place (may be empty for a pure
	// deletion).
	Insert []SemanticToken
}

// ApplySemanticTokenEdits updates the overlay incrementally, applying delta
// edits to the current token set (the slice last passed to
// [Editor.SetSemanticTokens] or produced by a prior delta) instead of replacing
// it wholesale — the decoded-token counterpart of an LSP
// semanticTokens/full/delta response, so a host need not re-materialize and
// re-send the full set on every analyze.
//
// Edits must be sorted by Start ascending and non-overlapping, exactly as LSP
// guarantees; they are applied as one pass. Out of range or overlapping edits
// are clamped defensively rather than panicking. No edits is a no-op.
//
// Edits index the **delta base** — the set as last published by
// [Editor.SetSemanticTokens] or a prior delta — not the rendered overlay. The
// base is held exactly as published and is never anchor-shifted, so local edits
// that move or drop tokens in the rendered overlay (#179) do not re-index it: a
// delta the server computed against its last result still lands on the right
// tokens. The host remains responsible for only sending a delta whose base
// matches that last published set (LSP resultId coordination). The delta result
// reflects the current document, so it resets the rendered overlay just like a
// full publish.
func (e *Editor) ApplySemanticTokenEdits(edits []SemanticTokenEdit) {
	if len(edits) == 0 {
		return
	}
	e.semantic.publish(applySemanticTokenEdits(e.semantic.server, edits))
	e.entry.Refresh()
}

// applySemanticTokenEdits returns cur with edits applied as a single splice
// pass. Edits are expected sorted by Start ascending and non-overlapping;
// indices are clamped to keep the pass monotonic and in-bounds so malformed
// input can't panic.
func applySemanticTokenEdits(cur []SemanticToken, edits []SemanticTokenEdit) []SemanticToken {
	n := len(cur)
	out := make([]SemanticToken, 0, n)
	prev := 0 // first index of cur not yet copied
	for _, ed := range edits {
		start := ed.Start
		if start < prev {
			start = prev // overlap or unsorted: clamp to keep the pass monotonic
		}
		if start > n {
			start = n
		}
		out = append(out, cur[prev:start]...)
		out = append(out, ed.Insert...)
		end := start
		if ed.DeleteCount > 0 {
			end = start + ed.DeleteCount
			if end > n {
				end = n
			}
		}
		prev = end
	}
	out = append(out, cur[prev:]...)
	return out
}

// SemanticTokens returns a copy of the token set last passed to
// [Editor.SetSemanticTokens]; nil when none are set.
func (e *Editor) SemanticTokens() []SemanticToken {
	if len(e.semantic.tokens) == 0 {
		return nil
	}
	return append([]SemanticToken(nil), e.semantic.tokens...)
}

// SetSemanticStyles replaces the mapping from semantic token-type names to
// fycoder theme color names used to color the overlay. Start from
// [DefaultSemanticStyles] and adjust, or supply a fully custom map. A nil map
// restores the defaults. Call before or after [Editor.SetSemanticTokens]; the
// change takes effect on the next repaint.
func (e *Editor) SetSemanticStyles(styles map[string]bellui.ThemeColorName) {
	if styles == nil {
		styles = DefaultSemanticStyles()
	}
	e.semantic.styles = styles
	e.entry.Refresh()
}

// SetSemanticModifierStyles replaces the mapping from semantic token-modifier
// names to text styles (bold/italic/underline) composed onto the overlay. Start
// from [DefaultSemanticModifierStyles] and adjust, or supply a fully custom map.
// A nil map restores the defaults. The change takes effect on the next repaint.
func (e *Editor) SetSemanticModifierStyles(styles map[string]SemanticModifierStyle) {
	if styles == nil {
		styles = DefaultSemanticModifierStyles()
	}
	e.semantic.modStyles = styles
	e.entry.Refresh()
}
