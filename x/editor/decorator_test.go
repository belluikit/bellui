package editor

import (
	"testing"

	"github.com/belluikit/bellui/geo"
)

func TestCurrentLineDecoratorSkipsOtherRows(t *testing.T) {
	d := NewCurrentLineDecorator(3) // 1-based: line 3
	pos := geo.NewPos(0, 0)
	size := geo.NewSize(100, 14)

	// Wrong line — should return nil.
	if objs := d.Decorate(1, pos, size); objs != nil {
		t.Errorf("line 1: expected nil, got %d objects", len(objs))
	}

	// Correct line — should return background rectangle.
	if objs := d.Decorate(3, pos, size); len(objs) != 1 {
		t.Errorf("line 3: expected 1 object, got %d", len(objs))
	}

	// Disabled (0 = no highlight).
	d.Row = 0
	if objs := d.Decorate(3, pos, size); objs != nil {
		t.Errorf("disabled: expected nil, got %d objects", len(objs))
	}
}

func TestSearchHighlightDecorator(t *testing.T) {
	d := NewSearchHighlightDecorator()
	d.Rows[1] = true // 1-based: line 1
	d.Rows[5] = true // 1-based: line 5

	pos := geo.NewPos(0, 0)
	size := geo.NewSize(100, 14)

	// Line 0 does not exist (1-based) — should return nil.
	if objs := d.Decorate(0, pos, size); objs != nil {
		t.Errorf("line 0: expected nil, got %d objects", len(objs))
	}
	if objs := d.Decorate(1, pos, size); len(objs) != 1 {
		t.Errorf("line 1: expected 1 object, got %d", len(objs))
	}
	if objs := d.Decorate(5, pos, size); len(objs) != 1 {
		t.Errorf("line 5: expected 1 object, got %d", len(objs))
	}
}
