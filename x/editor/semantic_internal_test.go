package editor

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// ovRuneLen is the fork's measure of a StyleRun: only its rune length is used.
func ovRuneLen(r widget.StyleRun) int { return len([]rune(r.Text)) }

func TestSemanticOverlayRunsForRow(t *testing.T) {
	test.NewTempApp(t) // themeColor needs a running app to resolve defaults

	typeCol := themeColor(ColorNameSyntaxType)
	annCol := themeColor(ColorNameSyntaxAnnotation)

	t.Run("empty when no tokens on row", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 1, Col: 0, Length: 3, Style: "type"}})
		if got := o.runsForRow(0); got != nil {
			t.Fatalf("row 0 has no tokens, want nil, got %v", got)
		}
	})

	t.Run("leading spacer then colored token", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 0, Col: 2, Length: 3, Style: "type"}})
		runs := o.runsForRow(0)
		if len(runs) != 2 {
			t.Fatalf("want spacer+token, got %d runs: %v", len(runs), runs)
		}
		if ovRuneLen(runs[0]) != 2 || runs[0].ForegroundColor != nil {
			t.Errorf("run[0] should be an unstyled 2-rune spacer, got len=%d color=%v", ovRuneLen(runs[0]), runs[0].ForegroundColor)
		}
		if ovRuneLen(runs[1]) != 3 || runs[1].ForegroundColor != typeCol {
			t.Errorf("run[1] should be a 3-rune token in type color, got len=%d color=%v", ovRuneLen(runs[1]), runs[1].ForegroundColor)
		}
	})

	t.Run("token at column 0 has no leading spacer", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 4, Style: "function"}})
		runs := o.runsForRow(0)
		if len(runs) != 1 || ovRuneLen(runs[0]) != 4 {
			t.Fatalf("want a single 4-rune token, got %v", runs)
		}
	})

	t.Run("two tokens tile spacer/token/spacer/token", func(t *testing.T) {
		o := newSemanticOverlay()
		// deliberately unsorted input — setTokens sorts by column
		o.setTokens([]SemanticToken{
			{Row: 0, Col: 8, Length: 2, Style: "annotation"},
			{Row: 0, Col: 1, Length: 3, Style: "type"},
		})
		runs := o.runsForRow(0)
		if len(runs) != 4 {
			t.Fatalf("want 4 runs, got %d: %v", len(runs), runs)
		}
		if ovRuneLen(runs[0]) != 1 || ovRuneLen(runs[1]) != 3 || ovRuneLen(runs[2]) != 4 || ovRuneLen(runs[3]) != 2 {
			t.Fatalf("unexpected tiling lengths: %d/%d/%d/%d", ovRuneLen(runs[0]), ovRuneLen(runs[1]), ovRuneLen(runs[2]), ovRuneLen(runs[3]))
		}
		if runs[1].ForegroundColor != typeCol {
			t.Errorf("first token should be type color")
		}
		if runs[3].ForegroundColor != annCol {
			t.Errorf("second token should be annotation color")
		}
	})

	t.Run("unmapped style leaves the range unstyled", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "notARealType"}})
		if got := o.runsForRow(0); got != nil {
			t.Fatalf("unmapped token should not tile, got %v", got)
		}
	})

	t.Run("overlapping token is dropped to keep tiling monotonic", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{
			{Row: 0, Col: 0, Length: 5, Style: "type"},
			{Row: 0, Col: 2, Length: 5, Style: "function"}, // overlaps the first
		})
		runs := o.runsForRow(0)
		if len(runs) != 1 || ovRuneLen(runs[0]) != 5 || runs[0].ForegroundColor != typeCol {
			t.Fatalf("want only the first token, got %v", runs)
		}
	})

	t.Run("non-positive length and negative coords are ignored", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{
			{Row: 0, Col: 0, Length: 0, Style: "type"},
			{Row: 0, Col: -1, Length: 3, Style: "type"},
			{Row: -1, Col: 0, Length: 3, Style: "type"},
		})
		if got := o.runsForRow(0); got != nil {
			t.Fatalf("all tokens invalid, want nil, got %v", got)
		}
	})
}

func TestEditorSemanticTokensAPI(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("type Foo = int\n@bar baz")

	if e.entry.StyleOverlayForRow == nil {
		t.Fatal("StyleOverlayForRow should be wired unconditionally")
	}
	if got := e.SemanticTokens(); got != nil {
		t.Fatalf("no tokens published yet, want nil, got %v", got)
	}
	// With no tokens, the overlay contributes nothing.
	if runs := e.entry.StyleOverlayForRow(0); runs != nil {
		t.Fatalf("empty overlay should return nil, got %v", runs)
	}

	tokens := []SemanticToken{
		{Row: 0, Col: 5, Length: 3, Style: "type"},       // "Foo"
		{Row: 1, Col: 0, Length: 4, Style: "annotation"}, // "@bar"
	}
	e.SetSemanticTokens(tokens)

	if got := e.SemanticTokens(); len(got) != 2 {
		t.Fatalf("SemanticTokens should round-trip, got %v", got)
	}
	// getter returns a copy — mutating it must not affect the editor
	e.SemanticTokens()[0].Style = "mutated"
	if e.SemanticTokens()[0].Style != "type" {
		t.Error("SemanticTokens must return a defensive copy")
	}

	row0 := e.entry.StyleOverlayForRow(0)
	if len(row0) != 2 || ovRuneLen(row0[0]) != 5 || ovRuneLen(row0[1]) != 3 {
		t.Fatalf("row 0 overlay tiling wrong: %v", row0)
	}
	if row0[1].ForegroundColor != themeColor(ColorNameSyntaxType) {
		t.Error("row 0 token should be type color")
	}
	row1 := e.entry.StyleOverlayForRow(1)
	if len(row1) != 1 || row1[0].ForegroundColor != themeColor(ColorNameSyntaxAnnotation) {
		t.Fatalf("row 1 overlay wrong: %v", row1)
	}

	// Clearing removes the overlay.
	e.SetSemanticTokens(nil)
	if e.SemanticTokens() != nil || e.entry.StyleOverlayForRow(0) != nil {
		t.Error("SetSemanticTokens(nil) should clear the overlay")
	}
}

func TestEditorSetSemanticStyles(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("keyword here")
	e.SetSemanticTokens([]SemanticToken{{Row: 0, Col: 0, Length: 7, Style: "keyword"}})

	// Default maps "keyword" -> keyword color.
	if got := e.entry.StyleOverlayForRow(0); len(got) != 1 || got[0].ForegroundColor != themeColor(ColorNameSyntaxKeyword) {
		t.Fatalf("default keyword mapping not applied: %v", got)
	}

	// Remap "keyword" to the string color; the change takes effect immediately.
	e.SetSemanticStyles(map[string]bellui.ThemeColorName{"keyword": ColorNameSyntaxString})
	if got := e.entry.StyleOverlayForRow(0); len(got) != 1 || got[0].ForegroundColor != themeColor(ColorNameSyntaxString) {
		t.Fatalf("custom style map not applied: %v", got)
	}

	// nil restores defaults.
	e.SetSemanticStyles(nil)
	if got := e.entry.StyleOverlayForRow(0); got[0].ForegroundColor != themeColor(ColorNameSyntaxKeyword) {
		t.Fatalf("nil should restore default styles: %v", got)
	}
}

// TestDefaultSemanticStylesProperty covers the standard LSP token types
// "property" and "enumMember" (#192), which a protolsp .pxf document emits for
// assignment/map-entry keys and enum-value idents respectively.
func TestDefaultSemanticStylesProperty(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("field VALUE")
	e.SetSemanticTokens([]SemanticToken{
		{Row: 0, Col: 0, Length: 5, Style: "property"},
		{Row: 0, Col: 6, Length: 5, Style: "enumMember"},
	})

	// Runs: colored "field", an uncolored spacer for the space, colored "VALUE".
	got := e.entry.StyleOverlayForRow(0)
	if len(got) != 3 {
		t.Fatalf("want 3 runs, got %d: %v", len(got), got)
	}
	if got[0].ForegroundColor != themeColor(ColorNameSyntaxFunction) {
		t.Errorf("property: want function color, got %v", got[0].ForegroundColor)
	}
	if got[2].ForegroundColor != themeColor(ColorNameSyntaxBuiltin) {
		t.Errorf("enumMember: want builtin color, got %v", got[2].ForegroundColor)
	}
}

// TestDefaultSemanticStylesStandardSet covers the remaining standard LSP token
// types filled in alongside property/enumMember (#192): variable, parameter,
// event, and regexp.
func TestDefaultSemanticStylesStandardSet(t *testing.T) {
	test.NewTempApp(t)
	styles := DefaultSemanticStyles()
	cases := map[string]bellui.ThemeColorName{
		"variable":  theme.ColorNameForeground,
		"parameter": theme.ColorNameForeground,
		"event":     ColorNameSyntaxFunction,
		"regexp":    ColorNameSyntaxString,
	}
	for tok, want := range cases {
		if got := styles[tok]; got != want {
			t.Errorf("%q: want %q, got %q", tok, want, got)
		}
	}
}

func TestSemanticOverlayModifiers(t *testing.T) {
	test.NewTempApp(t)
	typeCol := themeColor(ColorNameSyntaxType)

	t.Run("modifier composes bold over the type color", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "type", Modifiers: []string{"declaration"}}})
		runs := o.runsForRow(0)
		if len(runs) != 1 {
			t.Fatalf("want 1 run, got %v", runs)
		}
		if runs[0].ForegroundColor != typeCol || !runs[0].Bold {
			t.Errorf("want type color + bold, got color=%v bold=%v", runs[0].ForegroundColor, runs[0].Bold)
		}
	})

	t.Run("modifier styles a token whose type is unmapped", func(t *testing.T) {
		o := newSemanticOverlay()
		// no color (unmapped Style) but a mapped modifier -> bold-only run
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "notARealType", Modifiers: []string{"declaration"}}})
		runs := o.runsForRow(0)
		if len(runs) != 1 || runs[0].ForegroundColor != nil || !runs[0].Bold {
			t.Fatalf("want a bold-only run with no recolor, got %v", runs)
		}
	})

	t.Run("multiple modifiers compose", func(t *testing.T) {
		o := newSemanticOverlay()
		// declaration -> bold, deprecated -> strike-through (defaults)
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "type", Modifiers: []string{"declaration", "deprecated"}}})
		runs := o.runsForRow(0)
		if len(runs) != 1 || !runs[0].Bold || !runs[0].StrikeThrough {
			t.Fatalf("want bold+strikethrough, got %v", runs)
		}
	})

	t.Run("deprecated defaults to strike-through", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "type", Modifiers: []string{"deprecated"}}})
		runs := o.runsForRow(0)
		if len(runs) != 1 || !runs[0].StrikeThrough || runs[0].Italic {
			t.Fatalf("deprecated should strike (not italic), got %v", runs)
		}
	})

	t.Run("unmapped modifier is ignored", func(t *testing.T) {
		o := newSemanticOverlay()
		o.setTokens([]SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "notAType", Modifiers: []string{"nope"}}})
		if got := o.runsForRow(0); got != nil {
			t.Fatalf("no color and no mapped modifier -> nil, got %v", got)
		}
	})
}

func TestEditorSetSemanticModifierStyles(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("readonly thing")
	e.SetSemanticTokens([]SemanticToken{{Row: 0, Col: 0, Length: 8, Style: "type", Modifiers: []string{"readonly"}}})

	// "readonly" is not in the default modifier map -> no extra styling.
	if got := e.entry.StyleOverlayForRow(0); got[0].Italic || got[0].Bold {
		t.Fatalf("readonly should be unstyled by default, got %v", got[0])
	}

	// Map readonly -> italic; it applies immediately.
	e.SetSemanticModifierStyles(map[string]SemanticModifierStyle{"readonly": {Italic: true}})
	if got := e.entry.StyleOverlayForRow(0); !got[0].Italic {
		t.Fatalf("custom readonly->italic not applied: %v", got[0])
	}

	// nil restores defaults (readonly unmapped again).
	e.SetSemanticModifierStyles(nil)
	if got := e.entry.StyleOverlayForRow(0); got[0].Italic {
		t.Fatalf("nil should restore defaults (readonly unmapped): %v", got[0])
	}
}

func TestShiftSemanticTokens(t *testing.T) {
	tok := func(row, col, ln int) SemanticToken {
		return SemanticToken{Row: row, Col: col, Length: ln, Style: "type"}
	}

	t.Run("no change returns not-dirty", func(t *testing.T) {
		in := []SemanticToken{tok(0, 0, 3)}
		_, dirty := shiftSemanticTokens(in, "abc", "abc")
		if dirty {
			t.Error("identical text should not be dirty")
		}
	})

	t.Run("inserted line above shifts the row down", func(t *testing.T) {
		// "foo\nWidget" -> insert "new\n" at the top -> "new\nfoo\nWidget"
		old, nw := "foo\nWidget", "new\nfoo\nWidget"
		out, dirty := shiftSemanticTokens([]SemanticToken{tok(1, 0, 6)}, old, nw)
		if !dirty || len(out) != 1 {
			t.Fatalf("want 1 shifted token, got %v (dirty=%v)", out, dirty)
		}
		if out[0].Row != 2 || out[0].Col != 0 || out[0].Length != 6 {
			t.Errorf("want row 2 col 0 len 6, got %+v", out[0])
		}
	})

	t.Run("same-line insert before token shifts the column", func(t *testing.T) {
		// "abWidget" with token "Widget" at col 2; insert "XY" at col 0.
		out, dirty := shiftSemanticTokens([]SemanticToken{tok(0, 2, 6)}, "abWidget", "XYabWidget")
		if !dirty || out[0].Col != 4 || out[0].Row != 0 || out[0].Length != 6 {
			t.Errorf("want col 4 len 6, got %+v (dirty=%v)", out[0], dirty)
		}
	})

	t.Run("edit after the token leaves it alone", func(t *testing.T) {
		out, dirty := shiftSemanticTokens([]SemanticToken{tok(0, 0, 3)}, "abc def", "abc XYZdef")
		if dirty || out[0].Row != 0 || out[0].Col != 0 || out[0].Length != 3 {
			t.Errorf("token before the edit should be untouched, got %+v (dirty=%v)", out[0], dirty)
		}
	})

	t.Run("deletion swallowing the token drops it", func(t *testing.T) {
		// delete "Widget" entirely
		out, dirty := shiftSemanticTokens([]SemanticToken{tok(0, 0, 6)}, "Widget", "")
		if !dirty || len(out) != 0 {
			t.Errorf("swallowed token should be dropped, got %v (dirty=%v)", out, dirty)
		}
	})

	t.Run("newline inserted inside the token drops it (row split)", func(t *testing.T) {
		// "Widget" token [0,6); insert "\n" at col 3 -> "Wid\nget"
		out, dirty := shiftSemanticTokens([]SemanticToken{tok(0, 0, 6)}, "Widget", "Wid\nget")
		if !dirty || len(out) != 0 {
			t.Errorf("token split across rows should be dropped, got %v (dirty=%v)", out, dirty)
		}
	})

	t.Run("invalid tokens pass through untouched", func(t *testing.T) {
		bad := SemanticToken{Row: 0, Col: 0, Length: 0, Style: "type"}
		out, _ := shiftSemanticTokens([]SemanticToken{bad}, "abc", "Xabc")
		if len(out) != 1 || out[0].Length != 0 || out[0].Row != 0 || out[0].Col != 0 {
			t.Errorf("invalid token should pass through, got %v", out)
		}
	})
}

func TestSemanticTokensFollowLiveEdit(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("foo\nWidget here")
	e.SetSemanticTokens([]SemanticToken{{Row: 1, Col: 0, Length: 6, Style: "type"}})

	// Before the edit, row 1 carries the overlay.
	if got := e.entry.StyleOverlayForRow(1); len(got) != 1 {
		t.Fatalf("precondition: row 1 should have the overlay, got %v", got)
	}

	// A genuine edit that inserts a line at the top must carry the token to row 2.
	e.entry.CursorRow, e.entry.CursorColumn = 0, 0
	e.entry.InsertText("new\n")

	if got := e.SemanticTokens(); len(got) != 1 || got[0].Row != 2 {
		t.Fatalf("token should have followed the inserted line to row 2, got %v", got)
	}
	if got := e.entry.StyleOverlayForRow(2); len(got) != 1 || got[0].ForegroundColor != themeColor(ColorNameSyntaxType) {
		t.Fatalf("overlay should now paint row 2, got %v", got)
	}
	if got := e.entry.StyleOverlayForRow(1); got != nil {
		t.Fatalf("row 1 should no longer carry the overlay, got %v", got)
	}
}

func TestApplySemanticTokenEdits(t *testing.T) {
	mk := func(col int) SemanticToken { return SemanticToken{Row: 0, Col: col, Length: 1, Style: "type"} }
	cols := func(ts []SemanticToken) []int {
		out := make([]int, len(ts))
		for i, tok := range ts {
			out[i] = tok.Col
		}
		return out
	}
	eq := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	cur := []SemanticToken{mk(0), mk(1), mk(2), mk(3)}

	t.Run("insert in the middle", func(t *testing.T) {
		got := applySemanticTokenEdits(cur, []SemanticTokenEdit{{Start: 2, DeleteCount: 0, Insert: []SemanticToken{mk(9)}}})
		if !eq(cols(got), []int{0, 1, 9, 2, 3}) {
			t.Fatalf("got cols %v", cols(got))
		}
	})

	t.Run("delete a range", func(t *testing.T) {
		got := applySemanticTokenEdits(cur, []SemanticTokenEdit{{Start: 1, DeleteCount: 2}})
		if !eq(cols(got), []int{0, 3}) {
			t.Fatalf("got cols %v", cols(got))
		}
	})

	t.Run("replace (delete + insert)", func(t *testing.T) {
		got := applySemanticTokenEdits(cur, []SemanticTokenEdit{{Start: 1, DeleteCount: 1, Insert: []SemanticToken{mk(7), mk(8)}}})
		if !eq(cols(got), []int{0, 7, 8, 2, 3}) {
			t.Fatalf("got cols %v", cols(got))
		}
	})

	t.Run("two ordered non-overlapping edits apply as one pass", func(t *testing.T) {
		got := applySemanticTokenEdits(cur, []SemanticTokenEdit{
			{Start: 0, DeleteCount: 1, Insert: []SemanticToken{mk(5)}}, // replace tok0
			{Start: 3, DeleteCount: 1},                                 // delete tok3
		})
		if !eq(cols(got), []int{5, 1, 2}) {
			t.Fatalf("got cols %v", cols(got))
		}
	})

	t.Run("out-of-range and overlap are clamped, not panicking", func(t *testing.T) {
		got := applySemanticTokenEdits(cur, []SemanticTokenEdit{{Start: 99, DeleteCount: 99, Insert: []SemanticToken{mk(4)}}})
		if !eq(cols(got), []int{0, 1, 2, 3, 4}) { // clamped append at the end
			t.Fatalf("got cols %v", cols(got))
		}
	})

	t.Run("empty edits returns the list unchanged", func(t *testing.T) {
		got := applySemanticTokenEdits(cur, nil)
		if !eq(cols(got), []int{0, 1, 2, 3}) {
			t.Fatalf("got cols %v", cols(got))
		}
	})
}

func TestEditorApplySemanticTokenEdits(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("type A\ntype B")
	e.SetSemanticTokens([]SemanticToken{
		{Row: 0, Col: 5, Length: 1, Style: "type"},
		{Row: 1, Col: 5, Length: 1, Style: "type"},
	})

	// Insert a token on row 0 ahead of the existing ones, delete the row-1 token.
	e.ApplySemanticTokenEdits([]SemanticTokenEdit{
		{Start: 0, DeleteCount: 0, Insert: []SemanticToken{{Row: 0, Col: 0, Length: 4, Style: "keyword"}}},
		{Start: 1, DeleteCount: 1}, // drop the original row-1 token (now at index 1)
	})

	got := e.SemanticTokens()
	if len(got) != 2 {
		t.Fatalf("want 2 tokens after delta, got %v", got)
	}
	// Row 0 now carries two tokens: keyword @0 (no leading spacer), then a spacer
	// and the type @5 — three runs.
	if runs := e.entry.StyleOverlayForRow(0); len(runs) != 3 {
		t.Fatalf("row 0 should tile 2 tokens (kw,spacer,type), got %d runs: %v", len(runs), runs)
	}
	if runs := e.entry.StyleOverlayForRow(1); runs != nil {
		t.Fatalf("row 1 token should be gone, got %v", runs)
	}

	// Empty edit list is a no-op.
	before := e.SemanticTokens()
	e.ApplySemanticTokenEdits(nil)
	if len(e.SemanticTokens()) != len(before) {
		t.Fatal("empty delta should be a no-op")
	}
}

// TestDeltaAppliesToUnshiftedBase is the #188 coherence check: an anchor edit
// (#179) that drops a token from the rendered overlay must not re-index the base
// a subsequent index-based delta (#186) applies against.
func TestDeltaAppliesToUnshiftedBase(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("AAA\nBBB")
	e.SetSemanticTokens([]SemanticToken{
		{Row: 0, Col: 0, Length: 3, Style: "type"},     // A, index 0
		{Row: 1, Col: 0, Length: 3, Style: "function"}, // B, index 1
	})

	// Delete the whole first line: anchor-tracking swallows A (dropped) and
	// shifts B up to row 0 — the rendered set is now just [B], re-indexed.
	if !e.entry.ApplyEdits([]widget.TextEdit{{Start: 0, End: 4}}) {
		t.Fatal("delete-line edit should apply")
	}
	if got := e.SemanticTokens(); len(got) != 1 || got[0].Style != "function" {
		t.Fatalf("precondition: rendered set should be just B after the drop, got %v", got)
	}

	// The server re-analyzes and sends a delta against its last full result
	// [A, B]: replace index 1 (B) with a new token C. This must target B in the
	// un-shifted base — not whatever index 1 is in the shrunken rendered set.
	e.ApplySemanticTokenEdits([]SemanticTokenEdit{
		{Start: 1, DeleteCount: 1, Insert: []SemanticToken{{Row: 0, Col: 0, Length: 3, Style: "keyword"}}},
	})

	got := e.SemanticTokens()
	styles := map[string]bool{}
	for _, tok := range got {
		styles[tok.Style] = true
	}
	if styles["function"] {
		t.Errorf("delta indexed the shifted rendered set, not the base: B (function) should be gone, got %v", got)
	}
	if !styles["type"] || !styles["keyword"] {
		t.Errorf("delta should have applied to base [A,B] -> [A(type), C(keyword)], got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("want 2 tokens after base-delta, got %d: %v", len(got), got)
	}
}

// TestFullPublishResetsBase confirms a full SetSemanticTokens re-bases, so a
// later delta indexes the newly published set, not a stale base.
func TestFullPublishResetsBase(t *testing.T) {
	test.NewTempApp(t)
	e := NewEditor()
	e.SetText("x y z")
	e.SetSemanticTokens([]SemanticToken{{Row: 0, Col: 0, Length: 1, Style: "type"}})
	// Re-publish a different single-token base.
	e.SetSemanticTokens([]SemanticToken{{Row: 0, Col: 4, Length: 1, Style: "function"}})
	// Delete index 0 of the *new* base.
	e.ApplySemanticTokenEdits([]SemanticTokenEdit{{Start: 0, DeleteCount: 1}})
	if got := e.SemanticTokens(); len(got) != 0 {
		t.Fatalf("delta should have emptied the re-based set, got %v", got)
	}
}
