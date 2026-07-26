package pool

import (
	"image/color"
	"sync"
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	lines, rects, circles, texts := p.Stats()
	if lines != 0 || rects != 0 || circles != 0 || texts != 0 {
		t.Errorf("new pool Stats = (%d,%d,%d,%d), want all zeros", lines, rects, circles, texts)
	}
}

func TestLine_NewAndReuse(t *testing.T) {
	p := New()
	p1 := geo.NewPos(0, 0)
	p2 := geo.NewPos(10, 10)
	c := color.White

	l1 := p.Line(p1, p2, c, 1)
	if l1 == nil {
		t.Fatal("Line() returned nil")
	}

	p.Reset()

	l2 := p.Line(p1, p2, c, 1)
	if l2 != l1 {
		t.Error("expected reused Line pointer after Reset, got a new one")
	}
}

func TestRect_NewAndReuse(t *testing.T) {
	p := New()
	pos := geo.NewPos(5, 5)
	size := geo.NewSize(20, 20)
	c := color.Black

	r1 := p.Rect(pos, size, c)
	if r1 == nil {
		t.Fatal("Rect() returned nil")
	}

	p.Reset()

	r2 := p.Rect(pos, size, c)
	if r2 != r1 {
		t.Error("expected reused Rect pointer after Reset, got a new one")
	}
}

func TestCircle_NewAndReuse(t *testing.T) {
	p := New()
	center := geo.NewPos(50, 50)
	c := color.NRGBA{R: 255, A: 255}

	c1 := p.Circle(center, 10, c)
	if c1 == nil {
		t.Fatal("Circle() returned nil")
	}

	p.Reset()

	c2 := p.Circle(center, 10, c)
	if c2 != c1 {
		t.Error("expected reused Circle pointer after Reset, got a new one")
	}
}

func TestText_NewAndReuse(t *testing.T) {
	p := New()
	c := color.White

	t1 := p.Text("hello", c, 14)
	if t1 == nil {
		t.Fatal("Text() returned nil")
	}

	p.Reset()

	t2 := p.Text("world", c, 12)
	if t2 != t1 {
		t.Error("expected reused Text pointer after Reset, got a new one")
	}
	if t2.Text != "world" {
		t.Errorf("reused Text.Text = %q, want %q", t2.Text, "world")
	}
	if t2.TextSize != 12 {
		t.Errorf("reused Text.TextSize = %v, want 12", t2.TextSize)
	}
}

func TestReset(t *testing.T) {
	p := New()
	c := color.White
	pos := geo.NewPos(0, 0)

	// Create some objects
	p.Line(pos, pos, c, 1)
	p.Line(pos, pos, c, 1)
	p.Rect(pos, geo.NewSize(10, 10), c)
	p.Circle(pos, 5, c)
	p.Text("x", c, 10)

	lines, rects, circles, texts := p.Stats()
	if lines != 2 || rects != 1 || circles != 1 || texts != 1 {
		t.Fatalf("Stats before Reset = (%d,%d,%d,%d), want (2,1,1,1)", lines, rects, circles, texts)
	}

	p.Reset()

	// Capacity should be preserved (Stats returns len of slices, not indices)
	lines2, rects2, circles2, texts2 := p.Stats()
	if lines2 != 2 || rects2 != 1 || circles2 != 1 || texts2 != 1 {
		t.Errorf("Stats after Reset = (%d,%d,%d,%d), want capacity preserved (2,1,1,1)", lines2, rects2, circles2, texts2)
	}
}

func TestConcurrent(t *testing.T) {
	p := New()
	c := color.White
	pos := geo.NewPos(0, 0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Line(pos, pos, c, 1)
			p.Rect(pos, geo.NewSize(5, 5), c)
			p.Circle(pos, 3, c)
			p.Text("t", c, 10)
		}()
	}
	wg.Wait()

	lines, rects, circles, texts := p.Stats()
	if lines != 100 {
		t.Errorf("concurrent Lines = %d, want 100", lines)
	}
	if rects != 100 {
		t.Errorf("concurrent Rects = %d, want 100", rects)
	}
	if circles != 100 {
		t.Errorf("concurrent Circles = %d, want 100", circles)
	}
	if texts != 100 {
		t.Errorf("concurrent Texts = %d, want 100", texts)
	}
}
