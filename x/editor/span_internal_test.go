package editor

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

func TestUnderlineObjects(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}

	t.Run("solid is one horizontal line", func(t *testing.T) {
		objs := underlineObjects(10, 50, 20, 14, UnderlineSolid, red)
		if len(objs) != 1 {
			t.Fatalf("expected 1 object, got %d", len(objs))
		}
		ln, ok := objs[0].(*canvas.Line)
		if !ok {
			t.Fatalf("expected *canvas.Line, got %T", objs[0])
		}
		if ln.Position1.Y != ln.Position2.Y {
			t.Error("solid underline should be horizontal")
		}
		if ln.StrokeColor != color.Color(red) {
			t.Error("underline color not applied")
		}
	})

	t.Run("wavy oscillates over multiple segments", func(t *testing.T) {
		objs := underlineObjects(10, 70, 20, 14, UnderlineWavy, red)
		if len(objs) < 2 {
			t.Fatalf("a squiggle needs >1 segment, got %d", len(objs))
		}
		var minY, maxY float32
		for i, o := range objs {
			ln := o.(*canvas.Line)
			if i == 0 {
				minY, maxY = ln.Position1.Y, ln.Position1.Y
			}
			minY = min(minY, min(ln.Position1.Y, ln.Position2.Y))
			maxY = max(maxY, max(ln.Position1.Y, ln.Position2.Y))
		}
		if maxY <= minY {
			t.Error("wavy underline should oscillate vertically")
		}
	})

	t.Run("dotted is multiple marks", func(t *testing.T) {
		objs := underlineObjects(10, 40, 20, 14, UnderlineDotted, red)
		if len(objs) < 2 {
			t.Fatalf("expected multiple dots, got %d", len(objs))
		}
		if _, ok := objs[0].(*canvas.Rectangle); !ok {
			t.Fatalf("expected *canvas.Rectangle dots, got %T", objs[0])
		}
	})

	t.Run("empty span yields nothing", func(t *testing.T) {
		if objs := underlineObjects(50, 50, 20, 14, UnderlineSolid, red); objs != nil {
			t.Errorf("expected nil for zero-width span, got %d", len(objs))
		}
	})
}

func TestSpanObjects(t *testing.T) {
	rect := geo.NewRectangle(5, 10, 40, 14)
	blue := color.NRGBA{B: 0xff, A: 0x40}

	t.Run("background is a filled rect", func(t *testing.T) {
		objs := spanObjects(DecoratedSpan{Kind: SpanBackground, Color: blue}, rect)
		if len(objs) != 1 {
			t.Fatalf("expected 1 object, got %d", len(objs))
		}
		r, ok := objs[0].(*canvas.Rectangle)
		if !ok {
			t.Fatalf("expected *canvas.Rectangle, got %T", objs[0])
		}
		if r.FillColor != color.Color(blue) {
			t.Error("background fill color not applied")
		}
	})

	t.Run("opaque background forces full alpha", func(t *testing.T) {
		objs := spanObjects(DecoratedSpan{Kind: SpanBackground, Color: blue, Opaque: true}, rect)
		r := objs[0].(*canvas.Rectangle)
		if _, _, _, a := r.FillColor.RGBA(); a != 0xffff {
			t.Errorf("opaque background should be fully opaque, got alpha %d", a>>8)
		}
	})

	t.Run("box strokes without filling", func(t *testing.T) {
		objs := spanObjects(DecoratedSpan{Kind: SpanBox, Color: blue}, rect)
		r := objs[0].(*canvas.Rectangle)
		if r.StrokeWidth <= 0 || r.StrokeColor == nil {
			t.Error("box should have a stroke")
		}
	})

	t.Run("strike-through is a line", func(t *testing.T) {
		objs := spanObjects(DecoratedSpan{Kind: SpanStrikeThrough, Color: blue}, rect)
		if _, ok := objs[0].(*canvas.Line); !ok {
			t.Fatalf("expected *canvas.Line, got %T", objs[0])
		}
	})

	t.Run("nil color falls back to default", func(t *testing.T) {
		objs := spanObjects(DecoratedSpan{Kind: SpanBackground}, rect)
		r := objs[0].(*canvas.Rectangle)
		if r.FillColor != color.Color(defaultSpanColor) {
			t.Error("nil span color should fall back to defaultSpanColor")
		}
	})
}

func TestSearchMatchSpanDecorator(t *testing.T) {
	other := color.NRGBA{R: 1, G: 2, B: 3, A: 70}
	active := color.NRGBA{R: 9, G: 8, B: 7, A: 140}
	d := &SearchMatchSpanDecorator{
		Matches: []TextRange{
			{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 5},
			{StartRow: 2, StartCol: 4, EndRow: 2, EndCol: 9},
			{StartRow: 5, StartCol: 1, EndRow: 5, EndCol: 6},
		},
		Active:      1,
		Color:       other,
		ActiveColor: active,
	}

	spans := d.Spans()
	if len(spans) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(spans))
	}
	for i, s := range spans {
		if s.Kind != SpanBackground {
			t.Errorf("span %d kind = %v, want SpanBackground", i, s.Kind)
		}
		want := color.Color(other)
		if i == 1 {
			want = active
		}
		if s.Color != want {
			t.Errorf("span %d color = %v, want %v", i, s.Color, want)
		}
	}

	t.Run("inactive index highlights none as active", func(t *testing.T) {
		d.Active = -1
		for i, s := range d.Spans() {
			if s.Color != color.Color(other) {
				t.Errorf("span %d color = %v, want the inactive color", i, s.Color)
			}
		}
	})

	t.Run("nil colors fall back to defaults", func(t *testing.T) {
		d := &SearchMatchSpanDecorator{
			Matches: []TextRange{{StartRow: 0, StartCol: 0, EndRow: 0, EndCol: 1}},
			Active:  0,
		}
		spans := d.Spans()
		if _, _, _, a := spans[0].Color.RGBA(); a == 0 {
			t.Error("active span should have a non-transparent default color")
		}
	})
}

func TestApplyHighlighter_RichRunPassthrough(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}
	e := NewEditor()
	e.Highlighter = HighlighterFunc(func(line string, state int) ([]StyleRun, int) {
		return []StyleRun{{
			Text:            line,
			Bold:            true,
			Italic:          true,
			BackgroundColor: color.NRGBA{G: 0xff, A: 0x30},
			Underline:       true,
			UnderlineStyle:  UnderlineWavy,
			UnderlineColor:  red,
		}}, 0
	})
	e.applyHighlighter()

	runs, _ := e.entry.LineHighlighter("hello", 0)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	r := runs[0]
	if !r.Bold || !r.Italic || !r.Underline {
		t.Error("bold/italic/underline flags not passed through")
	}
	if r.BackgroundColor == nil {
		t.Error("background color not passed through")
	}
	if r.UnderlineStyle != widget.UnderlineWavy {
		t.Errorf("underline style not mapped: got %v", r.UnderlineStyle)
	}
	if r.UnderlineColor != color.Color(red) {
		t.Error("underline color not passed through")
	}
}
