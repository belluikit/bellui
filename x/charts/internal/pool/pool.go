// Package pool provides canvas object pooling to reduce GC pressure
// when rendering charts with many data points.
package pool

import (
	"image/color"
	"sync"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// ObjectPool manages reusable canvas objects.
type ObjectPool struct {
	lines     []*canvas.Line
	rects     []*canvas.Rectangle
	circles   []*canvas.Circle
	texts     []*canvas.Text
	mu        sync.Mutex
	lineIdx   int
	rectIdx   int
	circleIdx int
	textIdx   int
}

// New creates an ObjectPool.
func New() *ObjectPool {
	return &ObjectPool{}
}

// Reset returns all objects to the pool for reuse. Call at the start of each render cycle.
func (p *ObjectPool) Reset() {
	p.mu.Lock()
	p.lineIdx = 0
	p.rectIdx = 0
	p.circleIdx = 0
	p.textIdx = 0
	p.mu.Unlock()
}

// Line returns a reusable Line object.
func (p *ObjectPool) Line(p1, p2 geo.Position, c color.Color, width float32) *canvas.Line {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.lineIdx < len(p.lines) {
		l := p.lines[p.lineIdx]
		p.lineIdx++
		l.Position1 = p1
		l.Position2 = p2
		l.StrokeColor = c
		l.StrokeWidth = width
		l.Hidden = false
		return l
	}

	l := &canvas.Line{
		Position1:   p1,
		Position2:   p2,
		StrokeColor: c,
		StrokeWidth: width,
	}
	p.lines = append(p.lines, l)
	p.lineIdx++
	return l
}

// Rect returns a reusable Rectangle object.
func (p *ObjectPool) Rect(pos geo.Position, size geo.Size, fill color.Color) *canvas.Rectangle {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.rectIdx < len(p.rects) {
		r := p.rects[p.rectIdx]
		p.rectIdx++
		r.FillColor = fill
		r.StrokeColor = nil
		r.StrokeWidth = 0
		r.CornerRadius = 0
		r.Move(pos)
		r.Resize(size)
		r.Hidden = false
		return r
	}

	r := canvas.NewRectangle(fill)
	r.Move(pos)
	r.Resize(size)
	p.rects = append(p.rects, r)
	p.rectIdx++
	return r
}

// Circle returns a reusable Circle object.
func (p *ObjectPool) Circle(center geo.Position, radius float32, fill color.Color) *canvas.Circle {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.circleIdx < len(p.circles) {
		c := p.circles[p.circleIdx]
		p.circleIdx++
		c.FillColor = fill
		c.StrokeColor = nil
		c.StrokeWidth = 0
		c.Position1 = geo.NewPos(center.X-radius, center.Y-radius)
		c.Position2 = geo.NewPos(center.X+radius, center.Y+radius)
		c.Hidden = false
		return c
	}

	c := canvas.NewCircle(fill)
	c.Position1 = geo.NewPos(center.X-radius, center.Y-radius)
	c.Position2 = geo.NewPos(center.X+radius, center.Y+radius)
	p.circles = append(p.circles, c)
	p.circleIdx++
	return c
}

// Text returns a reusable Text object.
func (p *ObjectPool) Text(text string, c color.Color, size float32) *canvas.Text {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.textIdx < len(p.texts) {
		t := p.texts[p.textIdx]
		p.textIdx++
		t.Text = text
		t.Color = c
		t.TextSize = size
		t.Hidden = false
		return t
	}

	t := canvas.NewText(text, c)
	t.TextSize = size
	p.texts = append(p.texts, t)
	p.textIdx++
	return t
}

// Stats returns the current pool utilization.
func (p *ObjectPool) Stats() (lines, rects, circles, texts int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.lines), len(p.rects), len(p.circles), len(p.texts)
}
