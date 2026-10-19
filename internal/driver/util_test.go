package driver_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/driver"
	internal_widget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
	_ "github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

func TestAbsolutePositionForObject(t *testing.T) {
	t1r1c1 := widget.NewLabel("row 1 col 1")
	t1r1c2 := widget.NewLabel("row 1 col 2")
	t1r2c1 := widget.NewLabel("row 2 col 1")
	t1r2c2 := widget.NewLabel("row 2 col 2")
	t1r2c2.Hide()
	t1r1 := container.NewWithoutLayout(t1r1c1, t1r1c2)
	t1r2 := container.NewWithoutLayout(t1r2c1, t1r2c2)
	tree1 := container.NewWithoutLayout(t1r1, t1r2)

	t1r1c1.Move(geo.NewPos(111, 111))
	t1r1c2.Move(geo.NewPos(112, 112))
	t1r2c1.Move(geo.NewPos(121, 121))
	t1r2c2.Move(geo.NewPos(122, 122))
	t1r1.Move(geo.NewPos(11, 11))
	t1r2.Move(geo.NewPos(12, 12))
	tree1.Move(geo.NewPos(1, 1))

	t2r1c1 := widget.NewLabel("row 1 col 1")
	t2r1c2 := widget.NewLabel("row 1 col 2")
	t2r2c1 := widget.NewLabel("row 2 col 1")
	t2r2c2 := widget.NewLabel("row 2 col 2")
	t2r1 := container.NewWithoutLayout(t2r1c1, t2r1c2)
	t2r2 := container.NewWithoutLayout(t2r2c1, t2r2c2)
	tree2 := container.NewWithoutLayout(t2r1, t2r2)

	t2r1c1.Move(geo.NewPos(211, 211))
	t2r1c2.Move(geo.NewPos(212, 212))
	t2r2c1.Move(geo.NewPos(221, 221))
	t2r2c2.Move(geo.NewPos(222, 222))
	t2r1.Move(geo.NewPos(21, 21))
	t2r2.Move(geo.NewPos(22, 22))
	tree2.Move(geo.NewPos(2, 2))

	t3r1 := widget.NewLabel("row 1")
	t3r2 := widget.NewLabel("row 2")
	tree3 := container.NewWithoutLayout(t3r1, t3r2)

	t3r1.Move(geo.NewPos(31, 31))
	t3r2.Move(geo.NewPos(32, 32))
	tree3.Move(geo.NewPos(3, 3))

	trees := []core.CanvasObject{tree1, tree2, tree3}

	outsideTrees := widget.NewLabel("outside trees")
	outsideTrees.Move(geo.NewPos(10, 10))

	tests := map[string]struct {
		object core.CanvasObject
		want   geo.Position
	}{
		"tree 1: a cell": {
			object: t1r1c2,
			want:   geo.NewPos(124, 124), // 1 (root) + 11 (row 1) + 112 (cell 2)
		},
		"tree 1: a row": {
			object: t1r2,
			want:   geo.NewPos(13, 13), // 1 (root) + 12 (row 2)
		},
		"tree 1: root": {
			object: tree1,
			want:   geo.NewPos(1, 1),
		},
		"tree 1: a hidden element": {
			object: t1r2c2,
			want:   geo.NewPos(0, 0),
		},

		"tree 2: a row": {
			object: t2r2,
			want:   geo.NewPos(24, 24), // 2 (root) + 22 (row 2)
		},
		"tree 2: root": {
			object: tree2,
			want:   geo.NewPos(2, 2),
		},

		"tree 3: a row": {
			object: t3r2,
			want:   geo.NewPos(35, 35), // 3 (root) + 32 (row 2)
		},
		"tree 3: root": {
			object: tree3,
			want:   geo.NewPos(3, 3),
		},

		"an object not inside any tree": {
			object: outsideTrees,
			want:   geo.NewPos(0, 0),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, driver.AbsolutePositionForObject(tt.object, trees))
		})
	}
}

func TestFindObjectAtPositionMatching(t *testing.T) {
	col1cell1 := &objectTree{
		pos:  geo.NewPos(10, 10),
		size: geo.NewSize(15, 15),
	}
	col1cell2 := &objectTree{
		pos:  geo.NewPos(10, 35),
		size: geo.NewSize(15, 15),
	}
	col1cell3 := &objectTree{
		pos:  geo.NewPos(10, 60),
		size: geo.NewSize(15, 15),
	}
	col1 := &objectTree{
		children: []core.CanvasObject{col1cell1, col1cell2, col1cell3},
		pos:      geo.NewPos(10, 10),
		size:     geo.NewSize(35, 80),
	}
	col2cell1 := &objectTree{
		pos:  geo.NewPos(10, 10),
		size: geo.NewSize(15, 15),
	}
	col2cell2 := &objectTree{
		pos:  geo.NewPos(10, 35),
		size: geo.NewSize(15, 15),
	}
	col2cell3 := &objectTree{
		pos:  geo.NewPos(10, 60),
		size: geo.NewSize(15, 15),
	}
	col2 := &objectTree{
		children: []core.CanvasObject{col2cell1, col2cell2, col2cell3},
		pos:      geo.NewPos(55, 10),
		size:     geo.NewSize(35, 80),
	}
	colTree := &objectTree{
		children: []core.CanvasObject{col1, col2},
		pos:      geo.NewPos(10, 10),
		size:     geo.NewSize(100, 100),
	}
	row1cell1 := &objectTree{
		pos:  geo.NewPos(10, 10),
		size: geo.NewSize(15, 15),
	}
	row1cell2 := &objectTree{
		pos:  geo.NewPos(35, 10),
		size: geo.NewSize(15, 15),
	}
	row1cell3 := &objectTree{
		pos:  geo.NewPos(60, 10),
		size: geo.NewSize(15, 15),
	}
	row1 := &objectTree{
		children: []core.CanvasObject{row1cell1, row1cell2, row1cell3},
		pos:      geo.NewPos(10, 10),
		size:     geo.NewSize(80, 35),
	}
	row2cell1 := &objectTree{
		pos:  geo.NewPos(10, 10),
		size: geo.NewSize(15, 15),
	}
	row2cell2 := &objectTree{
		pos:  geo.NewPos(35, 10),
		size: geo.NewSize(15, 15),
	}
	row2cell3 := &objectTree{
		pos:  geo.NewPos(60, 10),
		size: geo.NewSize(15, 15),
	}
	row2 := &objectTree{
		children: []core.CanvasObject{row2cell1, row2cell2, row2cell3},
		pos:      geo.NewPos(10, 55),
		size:     geo.NewSize(80, 35),
	}
	rowTree := &objectTree{
		children: []core.CanvasObject{row1, row2},
		pos:      geo.NewPos(10, 10),
		size:     geo.NewSize(100, 100),
	}
	tree1 := &objectTree{
		pos:  geo.NewPos(100, 100),
		size: geo.NewSize(5, 5),
	}
	tree2 := &objectTree{
		pos:  geo.NewPos(0, 0),
		size: geo.NewSize(5, 5),
	}
	tree3 := &objectTree{
		pos:  geo.NewPos(50, 50),
		size: geo.NewSize(5, 5),
	}
	for name, tt := range map[string]struct {
		matcher    func(object core.CanvasObject) bool
		overlay    core.CanvasObject
		pos        geo.Position
		roots      []core.CanvasObject
		wantObject core.CanvasObject
		wantPos    geo.Position
		wantLayer  int
	}{
		"match in overlay and roots": {
			matcher:    func(o core.CanvasObject) bool { return o.Size().Width == 15 },
			overlay:    colTree,
			pos:        geo.NewPos(35, 60),
			roots:      []core.CanvasObject{rowTree},
			wantObject: col1cell2,
			wantPos:    geo.NewPos(5, 5),
			wantLayer:  0,
		},
		"match in root but overlay without match present": {
			matcher:    func(o core.CanvasObject) bool { return o.Size().Width == 15 },
			overlay:    tree1,
			pos:        geo.NewPos(35, 60),
			roots:      []core.CanvasObject{colTree, rowTree},
			wantObject: nil,
			wantPos:    geo.Position{},
			wantLayer:  0,
		},
		"match in multiple roots without overlay": {
			matcher:    func(o core.CanvasObject) bool { return o.Size().Width == 15 },
			overlay:    nil,
			pos:        geo.NewPos(83, 83),
			roots:      []core.CanvasObject{tree1, rowTree, tree2, colTree},
			wantObject: row2cell3,
			wantPos:    geo.NewPos(3, 8),
			wantLayer:  2,
		},
		"no match in roots without overlay": {
			matcher:    func(o core.CanvasObject) bool { return true },
			overlay:    nil,
			pos:        geo.NewPos(66, 66),
			roots:      []core.CanvasObject{tree1, tree2, tree3},
			wantObject: nil,
			wantPos:    geo.Position{},
			wantLayer:  3,
		},
		"no overlay and no roots": {
			matcher:    func(o core.CanvasObject) bool { return true },
			overlay:    nil,
			pos:        geo.NewPos(66, 66),
			roots:      nil,
			wantObject: nil,
			wantPos:    geo.Position{},
			wantLayer:  0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			o, p, l := driver.FindObjectAtPositionMatching(tt.pos, tt.matcher, []core.CanvasObject{tt.overlay}, tt.roots...)
			assert.Equal(t, tt.wantObject, o, "found object")
			assert.Equal(t, tt.wantPos, p, "position of found object")
			assert.Equal(t, tt.wantLayer, l, "layer of found object (0 - overlay, 1, 2, 3… - roots")
		})
	}
}

func TestFindAllObjectsAtPositionMatching(t *testing.T) {
	// Build a nested tree: outer (scrollable) > inner (scrollable) > leaf
	innerChild := &objectTree{
		pos:  geo.NewPos(5, 5),
		size: geo.NewSize(20, 20),
	}
	inner := &scrollableTree{
		objectTree: objectTree{
			children: []core.CanvasObject{innerChild},
			pos:      geo.NewPos(10, 10),
			size:     geo.NewSize(40, 40),
		},
	}
	outer := &scrollableTree{
		objectTree: objectTree{
			children: []core.CanvasObject{inner},
			pos:      geo.NewPos(0, 0),
			size:     geo.NewSize(100, 100),
		},
	}

	isScrollable := func(o core.CanvasObject) bool {
		_, ok := o.(event.Scrollable)
		return ok
	}

	t.Run("finds all scrollables deepest first", func(t *testing.T) {
		results := driver.FindAllObjectsAtPositionMatching(
			geo.NewPos(20, 20), isScrollable, nil, outer,
		)
		assert.Len(t, results, 2)
		assert.Equal(t, inner, results[0].Object, "deepest scrollable first")
		assert.Equal(t, outer, results[1].Object, "shallowest scrollable second")
	})

	t.Run("finds only outer when mouse outside inner", func(t *testing.T) {
		results := driver.FindAllObjectsAtPositionMatching(
			geo.NewPos(80, 80), isScrollable, nil, outer,
		)
		assert.Len(t, results, 1)
		assert.Equal(t, outer, results[0].Object)
	})

	t.Run("returns empty when mouse outside all", func(t *testing.T) {
		results := driver.FindAllObjectsAtPositionMatching(
			geo.NewPos(200, 200), isScrollable, nil, outer,
		)
		assert.Len(t, results, 0)
	})
}

func TestReverseWalkVisibleObjectTree(t *testing.T) {
	child1 := canvas.NewRectangle(color.White)
	child1.SetMinSize(geo.NewSize(100, 100))
	child2 := canvas.NewRectangle(color.Black)
	child2.Hide()
	child3 := canvas.NewRectangle(color.White)
	base := container.NewHBox(child1, child2, child3)

	var walked []core.CanvasObject
	driver.ReverseWalkVisibleObjectTree(
		base,
		func(object core.CanvasObject, position geo.Position, clippingPos geo.Position, clippingSize geo.Size) bool {
			walked = append(walked, object)
			return false
		},
		nil,
	)

	assert.Equal(t, []core.CanvasObject{base, child3, child1}, walked)
}

func TestReverseWalkVisibleObjectTree_Clip(t *testing.T) {
	rect := canvas.NewRectangle(color.White)
	rect.SetMinSize(geo.NewSize(100, 100))
	child := canvas.NewRectangle(color.Black)
	base := container.NewGridWithColumns(
		1,
		rect,
		internal_widget.NewScroll(child),
		container.NewGridWithColumns(
			2,
			canvas.NewCircle(color.White),
			canvas.NewCircle(color.White),
			canvas.NewCircle(color.White),
			&scrollable{},
		),
	)

	// We are not in a window. Resize the container to a default size.
	base.Resize(base.MinSize())

	var scClipPos, scrollableClipPos geo.Position
	var scClipSize, scrollableClipSize geo.Size

	driver.ReverseWalkVisibleObjectTree(base, func(object core.CanvasObject, position geo.Position, clippingPos geo.Position, clippingSize geo.Size) bool {
		if _, ok := object.(*internal_widget.Scroll); ok {
			scClipPos = clippingPos
			scClipSize = clippingSize
		} else if _, ok = object.(event.Scrollable); ok {
			scrollableClipPos = clippingPos
			scrollableClipSize = clippingSize
		}
		return false
	}, nil)

	// layout:
	// +-------------------------------+
	// | 0,0: rect 100x100             |
	// +-------------------------------+
	// |            padding            |
	// +-------------------------------+
	// | 0,104: scroller 100x100       |
	// +-------------------------------+
	// |            padding            |
	// +--------------+-+--------------+
	// | circle 48x48 |p| circle 48x48 |
	// +--------------+-+--------------+
	// |            padding            |
	// +--------------+-+--------------+
	// | circle 48x48 |p| scrollable   |
	// +--------------+-+--------------+
	assert.Equal(t, geo.NewPos(0, 104), scClipPos)
	assert.Equal(t, geo.NewSize(100, 100), scClipSize)
	assert.Equal(t, geo.NewPos(52, 260), scrollableClipPos)
	assert.Equal(t, geo.NewSize(48, 48), scrollableClipSize)
}

func TestWalkVisibleObjectTree(t *testing.T) {
	child1 := canvas.NewRectangle(color.White)
	child1.SetMinSize(geo.NewSize(100, 100))
	child2 := canvas.NewRectangle(color.Black)
	child2.Hide()
	child3 := canvas.NewRectangle(color.White)
	base := container.NewHBox(child1, child2, child3)

	var walked []core.CanvasObject
	driver.WalkVisibleObjectTree(base, func(object core.CanvasObject, position geo.Position, clippingPos geo.Position, clippingSize geo.Size) bool {
		walked = append(walked, object)
		return false
	}, nil)

	assert.Equal(t, []core.CanvasObject{base, child1, child3}, walked)
}

func TestWalkVisibleObjectTree_Clip(t *testing.T) {
	rect := canvas.NewRectangle(color.White)
	rect.SetMinSize(geo.NewSize(100, 100))
	child := canvas.NewRectangle(color.Black)
	base := container.NewGridWithColumns(
		1,
		rect,
		internal_widget.NewScroll(child),
		container.NewGridWithColumns(
			2,
			canvas.NewCircle(color.White),
			canvas.NewCircle(color.White),
			canvas.NewCircle(color.White),
			&scrollable{},
		),
	)

	// We are not in a window. Resize the container to a default size.
	base.Resize(base.MinSize())

	var scClipPos, scrollableClipPos geo.Position
	var scClipSize, scrollableClipSize geo.Size

	driver.WalkVisibleObjectTree(base, func(object core.CanvasObject, position geo.Position, clippingPos geo.Position, clippingSize geo.Size) bool {
		if _, ok := object.(*internal_widget.Scroll); ok {
			scClipPos = clippingPos
			scClipSize = clippingSize
		} else if _, ok = object.(event.Scrollable); ok {
			scrollableClipPos = clippingPos
			scrollableClipSize = clippingSize
		}
		return false
	}, nil)

	// layout:
	// +-------------------------------+
	// | 0,0: rect 100x100             |
	// +-------------------------------+
	// |            padding            |
	// +-------------------------------+
	// | 0,104: scroller 100x100       |
	// +-------------------------------+
	// |            padding            |
	// +--------------+-+--------------+
	// | circle 48x48 |p| circle 48x48 |
	// +--------------+-+--------------+
	// |            padding            |
	// +--------------+-+--------------+
	// | circle 48x48 |p| scrollable   |
	// +--------------+-+--------------+
	assert.Equal(t, geo.NewPos(0, 104), scClipPos)
	assert.Equal(t, geo.NewSize(100, 100), scClipSize)
	assert.Equal(t, geo.NewPos(52, 260), scrollableClipPos)
	assert.Equal(t, geo.NewSize(48, 48), scrollableClipSize)
}

func TestWalkWholeObjectTree(t *testing.T) {
	child1 := canvas.NewRectangle(color.White)
	child1.SetMinSize(geo.NewSize(100, 100))
	child2 := canvas.NewRectangle(color.Black)
	child2.Hide()
	child3 := canvas.NewRectangle(color.White)
	base := container.NewHBox(child1, child2, child3)

	var walked []core.CanvasObject
	driver.WalkCompleteObjectTree(base, func(object core.CanvasObject, position geo.Position, clippingPos geo.Position, clippingSize geo.Size) bool {
		walked = append(walked, object)
		return false
	}, nil)

	assert.Equal(t, []core.CanvasObject{base, child1, child2, child3}, walked)
}

var _ core.Widget = (*objectTree)(nil)

type objectTree struct {
	children []core.CanvasObject
	hidden   bool
	pos      geo.Position
	size     geo.Size
}

func (o *objectTree) Size() geo.Size {
	return o.size
}

func (o *objectTree) Resize(size geo.Size) {
	o.size = size
}

func (o *objectTree) Position() geo.Position {
	return o.pos
}

func (o *objectTree) Move(position geo.Position) {
	o.pos = position
}

func (o *objectTree) MinSize() geo.Size {
	return o.size
}

func (o objectTree) Visible() bool {
	return !o.hidden
}

func (o *objectTree) Show() {
	o.hidden = false
}

func (o *objectTree) Hide() {
	o.hidden = true
}

func (o *objectTree) Refresh() {
}

func (o *objectTree) CreateRenderer() core.WidgetRenderer {
	r := &objectTreeRenderer{}
	r.SetObjects(o.children)
	return r
}

var _ core.WidgetRenderer = (*objectTreeRenderer)(nil)

type objectTreeRenderer struct {
	internal_widget.BaseRenderer
}

func (o objectTreeRenderer) Layout(_ geo.Size) {
}

func (o objectTreeRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (o objectTreeRenderer) Refresh() {
}

type scrollable struct {
	pos  geo.Position
	size geo.Size
}

var (
	_ core.CanvasObject = (*scrollable)(nil)
	_ event.Scrollable  = (*scrollable)(nil)
)

func (s *scrollable) Hide() {
	panic("implement me")
}

func (s *scrollable) MinSize() geo.Size {
	return geo.NewSize(1, 1)
}

func (s *scrollable) Move(position geo.Position) {
	s.pos = position
}

func (s *scrollable) Position() geo.Position {
	return s.pos
}

func (s *scrollable) Refresh() {
	panic("implement me")
}

func (s *scrollable) Resize(size geo.Size) {
	s.size = size
}

func (s *scrollable) Scrolled(ev *event.ScrollEvent) {
	panic("implement me")
}

func (s *scrollable) Show() {
	panic("implement me")
}

func (s *scrollable) Size() geo.Size {
	return s.size
}

func (s *scrollable) Visible() bool {
	return true
}

// scrollableTree is a widget that also implements Scrollable, for testing nested scrollable hierarchies.
type scrollableTree struct {
	objectTree
}

var (
	_ core.Widget      = (*scrollableTree)(nil)
	_ event.Scrollable = (*scrollableTree)(nil)
)

func (s *scrollableTree) Scrolled(_ *event.ScrollEvent) {
}

// TestClipsOfMatchesSeparateChecks pins that ClipsOf, which answers from one
// renderer lookup (#492), agrees with IsClip, IsRoundedClip and IsPathClip.
func TestClipsOfMatchesSeparateChecks(t *testing.T) {
	test.NewTempApp(t)
	path := canvas.NewPath(func(p *canvas.Path) {
		p.MoveTo(0, 0)
		p.LineTo(10, 0)
		p.LineTo(0, 10)
		p.Close()
	})
	card := widget.NewCard("t", "", widget.NewLabel("x"))
	rounded := &roundedClipWidget{}
	rounded.ExtendBaseWidget(rounded)
	pathClip := container.NewPathClip(path, widget.NewLabel("x"))

	for name, obj := range map[string]core.CanvasObject{
		"scroll":          container.NewVScroll(widget.NewLabel("x")),
		"clip container":  &bellui.Container{ClipToBounds: true},
		"plain container": &bellui.Container{},
		"card":            card,
		"path clip":       pathClip,
		"rounded":         rounded,
		"clip":            container.NewClip(widget.NewLabel("x")),
		"label":           widget.NewLabel("x"),
		"rectangle":       canvas.NewRectangle(color.Black),
	} {
		if w, ok := obj.(core.Widget); ok {
			cache.Renderer(w) // the checks read only cached renderers
		}
		c := driver.ClipsOf(obj)
		rounded, radius := driver.IsRoundedClip(obj)
		p, isPath := driver.IsPathClip(obj)
		assert.Equal(t, driver.IsClip(obj), c.Rect, name)
		assert.Equal(t, rounded, c.Rounded, name)
		assert.Equal(t, radius, c.Radius, name)
		assert.Equal(t, isPath, c.Path != nil, name)
		assert.Equal(t, p, c.Path, name)
	}
	// The cases that matter are covered, not only the negative ones.
	assert.NotNil(t, driver.ClipsOf(pathClip).Path)
	assert.Equal(t, driver.Clips{Rounded: true, Radius: 8}, driver.ClipsOf(rounded))
}

// roundedClipWidget's renderer clips its children to 8-unit rounded corners.
type roundedClipWidget struct{ widget.BaseWidget }

func (w *roundedClipWidget) CreateRenderer() core.WidgetRenderer {
	return &roundedClipRenderer{}
}

type roundedClipRenderer struct{}

func (r *roundedClipRenderer) Destroy()                         {}
func (r *roundedClipRenderer) Layout(geo.Size)                  {}
func (r *roundedClipRenderer) MinSize() geo.Size                { return geo.Size{} }
func (r *roundedClipRenderer) Objects() []core.CanvasObject     { return nil }
func (r *roundedClipRenderer) Refresh()                         {}
func (r *roundedClipRenderer) RoundedClipCornerRadius() float32 { return 8 }
