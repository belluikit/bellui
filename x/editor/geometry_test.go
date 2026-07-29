package editor_test

import (
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/editor"
)

// renderedEditor returns an editor laid out in a window so that row/column
// geometry is populated.
func renderedEditor(t *testing.T, text string) *editor.Editor {
	t.Helper()
	e := editor.NewEditor()
	e.SetText(text)
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))
	return e
}

func TestEditor_PositionAt(t *testing.T) {
	test.NewTempApp(t)
	e := renderedEditor(t, "abcdef\nxy")

	r0 := e.PositionAt(0, 0)
	r1 := e.PositionAt(0, 1)
	r2 := e.PositionAt(0, 2)
	if r0.IsZero() || r1.IsZero() {
		t.Fatal("expected non-zero rects for visible positions")
	}
	if !(r1.Pos.X > r0.Pos.X && r2.Pos.X > r1.Pos.X) {
		t.Errorf("X should increase across columns: %v %v %v", r0.Pos.X, r1.Pos.X, r2.Pos.X)
	}
	if d1, d2 := r1.Pos.X-r0.Pos.X, r2.Pos.X-r1.Pos.X; d1 < d2-0.5 || d1 > d2+0.5 {
		t.Errorf("monospace step should be uniform: %v vs %v", d1, d2)
	}
	if r0.Size.Width <= 0 || r0.Size.Height <= 0 {
		t.Errorf("caret cell should have positive size, got %v", r0.Size)
	}

	// Lower row sits below the first.
	if e.PositionAt(1, 0).Pos.Y <= r0.Pos.Y {
		t.Error("row 1 should be below row 0")
	}

	// Out of range yields the zero rect.
	for _, c := range []struct{ row, col int }{{-1, 0}, {0, -1}, {9, 0}, {0, 99}} {
		if got := e.PositionAt(c.row, c.col); !got.IsZero() {
			t.Errorf("PositionAt(%d,%d) should be zero, got %v", c.row, c.col, got)
		}
	}
}

func TestEditor_RangeRects(t *testing.T) {
	test.NewTempApp(t)
	e := renderedEditor(t, "abcdef\nghijkl\nmnopqr")

	// Single-row range → one rect with positive width.
	single := e.RangeRects(editor.TextRange{StartRow: 0, StartCol: 1, EndRow: 0, EndCol: 4})
	if len(single) != 1 {
		t.Fatalf("single-row range: expected 1 rect, got %d", len(single))
	}
	if single[0].Size.Width <= 0 {
		t.Errorf("expected positive width, got %v", single[0].Size.Width)
	}

	// Three-row range → one rect per visual row.
	multi := e.RangeRects(editor.TextRange{StartRow: 0, StartCol: 2, EndRow: 2, EndCol: 3})
	if len(multi) != 3 {
		t.Fatalf("three-row range: expected 3 rects, got %d", len(multi))
	}
	// Rows descend.
	if !(multi[0].Pos.Y < multi[1].Pos.Y && multi[1].Pos.Y < multi[2].Pos.Y) {
		t.Error("per-row rects should descend in Y")
	}

	// Empty range → nil.
	if got := e.RangeRects(editor.TextRange{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 2}); got != nil {
		t.Errorf("empty range should return nil, got %v", got)
	}
}

func TestEditor_StaticSpanDecorator_Renders(t *testing.T) {
	test.NewTempApp(t)
	e := editor.NewEditor()
	e.SpanDecorators = []editor.SpanDecorator{
		&editor.StaticSpanDecorator{SpansList: []editor.DecoratedSpan{
			{Range: editor.TextRange{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5},
				Kind: editor.SpanUnderline, UnderlineStyle: editor.UnderlineWavy},
		}},
	}
	e.SetText("error here")

	// Lay out; the overlay must build without panicking and the editor sizes.
	w := test.NewWindow(e)
	t.Cleanup(w.Close)
	w.Resize(geo.NewSize(400, 300))

	if e.Size().Width <= 0 || e.Size().Height <= 0 {
		t.Errorf("editor with span decorator failed to lay out: %v", e.Size())
	}
}

func TestTextRange_Union(t *testing.T) {
	type TR = editor.TextRange
	cases := []struct {
		name       string
		a, b, want TR
	}{
		{"disjoint same row", TR{0, 1, 0, 3}, TR{0, 5, 0, 8}, TR{0, 1, 0, 8}},
		{"nested keeps outer", TR{0, 0, 0, 10}, TR{0, 3, 0, 5}, TR{0, 0, 0, 10}},
		{"across rows", TR{1, 2, 1, 4}, TR{3, 0, 3, 6}, TR{1, 2, 3, 6}},
		{"empty left returns right", TR{0, 2, 0, 2}, TR{1, 0, 1, 4}, TR{1, 0, 1, 4}},
		{"empty right returns left", TR{1, 0, 1, 4}, TR{0, 2, 0, 2}, TR{1, 0, 1, 4}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.Union(c.b); got != c.want {
				t.Errorf("%+v.Union(%+v) = %+v, want %+v", c.a, c.b, got, c.want)
			}
		})
	}
	// Union of two empties is empty.
	if got := (TR{0, 2, 0, 2}).Union(TR{1, 1, 1, 1}); !got.IsEmpty() {
		t.Errorf("union of two empty ranges should be empty, got %+v", got)
	}
}
