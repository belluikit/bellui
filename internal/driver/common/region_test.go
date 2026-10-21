//go:build !nopartialrepaint && (partialrepaint || (!android && !ios && !js && !wasm && !mobile))

package common

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image/color"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// twinSizeable is fakeSizeable for two canvases showing the same content: c,
// whose region walks from what objects report, and ref, whose region walks
// everything every frame, as every region did before #599.
type twinSizeable struct {
	SizeableCanvas
	size   geo.Size
	c, ref *Canvas
}

func (f *twinSizeable) Size() geo.Size { return f.size }

func (f *twinSizeable) MarkPlacementDirty(o core.CanvasObject) { f.c.MarkPlacementDirty(o) }

func (f *twinSizeable) MarkMinSizeDirty(core.CanvasObject) {}

func (f *twinSizeable) SetDirty() {}

func (f *twinSizeable) PixelCoordinateForPosition(p geo.Position) (int, int) {
	return int(p.X), int(p.Y) // a scale of 1, for icons rendered at their size
}

func (f *twinSizeable) Scale() float32 { return 1 }

func (f *twinSizeable) Refresh(o core.CanvasObject) {
	for _, c := range []*Canvas{f.c, f.ref} {
		c.noteRefreshed(o) // as FreeDirtyTextures does
	}
}

// newTwinCanvases returns the twin canvases for content: see twinSizeable.
func newTwinCanvases(content core.CanvasObject, size geo.Size) (c, ref *Canvas) {
	c, ref = &Canvas{}, &Canvas{}
	impl := &twinSizeable{size: size, c: c, ref: ref}
	for _, cv := range []*Canvas{c, ref} {
		cv.Initialize(impl, func() {})
		cv.SetContentTreeAndFocusMgr(content)
	}
	a := bellui.CurrentApp()
	bellui.SetCurrentApp(&regionApp{App: a, d: &regionDriver{Driver: a.Driver(), c: impl}})
	return c, ref
}

// frame finds both canvases' regions and paints them, as a driver's frame
// does, and fails t where c's region misses what ref's walk of everything
// found: a node whose bounds the two recorded differently, or ref's region
// reaching outside c's. It reports whether c walked less than ref.
func frame(t *testing.T, c, ref *Canvas, step string) (fewer bool) {
	t.Helper()
	ref.placeAll.Store(true)
	rPos, rSize, rSet := ref.PartialDirtyRegion()
	cPos, cSize, cSet := c.PartialDirtyRegion()
	fewer = c.RegionVisits() < ref.RegionVisits()
	for _, cv := range []*Canvas{c, ref} {
		cv.WalkContentTreesCulled(cv.CullOffClip, nil, nil) // the paint walk
	}

	ref.nodeMap.Range(func(obj core.CanvasObject, rn *RenderCacheNode) bool {
		cn, ok := c.nodeMap.Load(obj)
		if !rn.lastPaintSet {
			if ok && cn.lastPaintSet {
				t.Errorf("%s: %T is recorded as drawn at %v %v, and is not", step, obj, cn.lastPaintPos, cn.lastPaintSize)
			}
			return true
		}
		if !ok || !cn.lastPaintSet {
			t.Errorf("%s: %T is drawn at %v %v, and not recorded", step, obj, rn.lastPaintPos, rn.lastPaintSize)
		} else if cn.lastPaintPos != rn.lastPaintPos || cn.lastPaintSize != rn.lastPaintSize {
			t.Errorf("%s: %T is drawn at %v %v, and recorded at %v %v", step, obj,
				rn.lastPaintPos, rn.lastPaintSize, cn.lastPaintPos, cn.lastPaintSize)
		}
		return true
	})
	c.nodeMap.Range(func(obj core.CanvasObject, cn *RenderCacheNode) bool {
		if _, ok := ref.nodeMap.Load(obj); !ok && cn.lastPaintSet {
			t.Errorf("%s: %T left the tree, and is recorded as drawn at %v %v", step, obj, cn.lastPaintPos, cn.lastPaintSize)
		}
		return true
	})

	if !rSet || rSize.IsZero() {
		return fewer
	}
	if !cSet {
		t.Errorf("%s: region %v %v found nothing", step, rPos, rSize)
		return fewer
	}
	const eps = 0.01
	if rPos.X < cPos.X-eps || rPos.Y < cPos.Y-eps ||
		rPos.X+rSize.Width > cPos.X+cSize.Width+eps || rPos.Y+rSize.Height > cPos.Y+cSize.Height+eps {
		t.Errorf("%s: region %v %v reaches outside %v %v", step, rPos, rSize, cPos, cSize)
	}
	return fewer
}

// ownObject is a CanvasObject an app implements itself: it reports nothing.
type ownObject struct {
	pos    geo.Position
	size   geo.Size
	hidden bool
}

func (o *ownObject) MinSize() geo.Size      { return geo.NewSize(1, 1) }
func (o *ownObject) Move(p geo.Position)    { o.pos = p }
func (o *ownObject) Position() geo.Position { return o.pos }
func (o *ownObject) Resize(s geo.Size)      { o.size = s }
func (o *ownObject) Size() geo.Size         { return o.size }
func (o *ownObject) Hide()                  { o.hidden = true }
func (o *ownObject) Visible() bool          { return !o.hidden }
func (o *ownObject) Show()                  { o.hidden = false }
func (o *ownObject) Refresh()               {}
func (o *ownObject) String() string         { return fmt.Sprintf("own%p", o) }

// ownWidget is a widget an app implements without BaseWidget: neither it nor
// its own layout of its children reports anything.
type ownWidget struct {
	ownObject
	children []core.CanvasObject
}

func (w *ownWidget) CreateRenderer() core.WidgetRenderer { return &ownRenderer{w} }

type ownRenderer struct{ w *ownWidget }

func (r *ownRenderer) Destroy()                     {}
func (r *ownRenderer) Layout(geo.Size)              {}
func (r *ownRenderer) MinSize() geo.Size            { return geo.NewSize(1, 1) }
func (r *ownRenderer) Objects() []core.CanvasObject { return r.w.children }
func (r *ownRenderer) Refresh()                     {}

// TestPartialDirtyRegion_FromReported drives a tree through changes of every
// kind and checks, frame by frame, that the region found by walking from what
// objects report misses nothing a walk of everything finds (#599).
func TestPartialDirtyRegion_FromReported(t *testing.T) {
	test.NewTempApp(t)

	rect := func(x, y, w, h float32) *canvas.Rectangle {
		r := canvas.NewRectangle(color.White)
		r.Move(geo.NewPos(x, y))
		r.Resize(geo.NewSize(w, h))
		return r
	}
	for _, tc := range []struct {
		name   string
		build  func() (core.CanvasObject, func())
		frames int
	}{
		{"move", func() (core.CanvasObject, func()) {
			r := rect(10, 10, 20, 20)
			return container.NewWithoutLayout(r, rect(100, 100, 20, 20)), func() { r.Move(geo.NewPos(50, 60)) }
		}, 1},
		{"resize", func() (core.CanvasObject, func()) {
			r := rect(10, 10, 20, 20)
			return container.NewWithoutLayout(r), func() { r.Resize(geo.NewSize(80, 5)) }
		}, 1},
		{"move container", func() (core.CanvasObject, func()) {
			inner := container.NewWithoutLayout(rect(0, 0, 10, 10), rect(30, 30, 10, 10))
			inner.Resize(geo.NewSize(50, 50))
			return container.NewWithoutLayout(inner), func() { inner.Move(geo.NewPos(200, 100)) }
		}, 1},
		{"hide only child", func() (core.CanvasObject, func()) {
			r := rect(10, 10, 20, 20)
			inner := container.NewWithoutLayout(r)
			inner.Resize(geo.NewSize(50, 50))
			return container.NewWithoutLayout(inner), r.Hide
		}, 2},
		{"show hidden from the start", func() (core.CanvasObject, func()) {
			r := rect(10, 10, 20, 20)
			r.Hide()
			return container.NewWithoutLayout(r, rect(100, 100, 5, 5)), r.Show
		}, 2},
		{"show widget hidden from the start", func() (core.CanvasObject, func()) {
			l := widget.NewLabel("shown")
			l.Resize(geo.NewSize(80, 30))
			l.Hide()
			return container.NewWithoutLayout(l), l.Show
		}, 2},
		{"remove all", func() (core.CanvasObject, func()) {
			inner := container.NewWithoutLayout(rect(0, 0, 10, 10), rect(20, 0, 10, 10))
			inner.Resize(geo.NewSize(50, 50))
			return container.NewWithoutLayout(inner), inner.RemoveAll
		}, 2},
		{"add", func() (core.CanvasObject, func()) {
			inner := container.NewWithoutLayout(rect(0, 0, 10, 10))
			inner.Resize(geo.NewSize(50, 50))
			return container.NewWithoutLayout(inner), func() { inner.Add(rect(60, 60, 10, 10)) }
		}, 1},
		{"objects replaced and refreshed", func() (core.CanvasObject, func()) {
			inner := container.NewWithoutLayout(rect(0, 0, 10, 10))
			inner.Resize(geo.NewSize(50, 50))
			return container.NewWithoutLayout(inner), func() {
				inner.Objects = []core.CanvasObject{rect(70, 0, 10, 10)}
				inner.Refresh()
			}
		}, 1},
		{"transform", func() (core.CanvasObject, func()) {
			r := rect(10, 10, 20, 20)
			return container.NewWithoutLayout(r), func() { r.SetTransform(geo.NewScale(3, 3)) }
		}, 1},
		{"label text", func() (core.CanvasObject, func()) {
			l := widget.NewLabel("short")
			l.Resize(geo.NewSize(200, 30))
			return container.NewWithoutLayout(l), func() { l.SetText("a much longer text than before") }
		}, 1},
		{"tree branch opened", func() (core.CanvasObject, func()) {
			// Its content grows, through a Resize that is not BaseWidget's.
			tree := widget.NewTreeWithStrings(map[string][]string{"": {"a", "b"}, "a": {"c", "d", "e"}})
			tree.Resize(geo.NewSize(200, 60))
			return container.NewWithoutLayout(tree), func() { tree.ToggleBranch("a") }
		}, 2},
		{"tree view branch opened", func() (core.CanvasObject, func()) {
			tree := widget.NewTreeViewWithNodes([]widget.TreeViewNode{{ID: "a", Text: "a"}, {ID: "b", Text: "b"}})
			tree.SetChildren("a", []widget.TreeViewNode{{ID: "c", Text: "c"}, {ID: "d", Text: "d"}})
			tree.Resize(geo.NewSize(200, 60))
			return container.NewWithoutLayout(tree), func() { tree.ToggleBranch("a") }
		}, 2},
		{"own object moved", func() (core.CanvasObject, func()) {
			o := &ownObject{pos: geo.NewPos(10, 10), size: geo.NewSize(20, 20)}
			return container.NewWithoutLayout(o, rect(100, 100, 5, 5)), func() { o.Move(geo.NewPos(150, 150)) }
		}, 1},
		{"own object hidden", func() (core.CanvasObject, func()) {
			o := &ownObject{pos: geo.NewPos(10, 10), size: geo.NewSize(20, 20)}
			return container.NewWithoutLayout(o, rect(100, 100, 5, 5)), o.Hide
		}, 1},
		{"own object shown, hidden from the start", func() (core.CanvasObject, func()) {
			o := &ownObject{pos: geo.NewPos(10, 10), size: geo.NewSize(20, 20), hidden: true}
			return container.NewWithoutLayout(o, rect(100, 100, 5, 5)), o.Show
		}, 1},
		{"own widget lays out its children", func() (core.CanvasObject, func()) {
			r := rect(0, 0, 10, 10)
			w := &ownWidget{ownObject: ownObject{size: geo.NewSize(100, 100)}, children: []core.CanvasObject{r}}
			// Not through Rectangle.Move, which reports: set as the app's
			// own layout might, through an object that does not.
			o := &ownObject{pos: geo.NewPos(5, 5), size: geo.NewSize(5, 5)}
			w.children = append(w.children, o)
			return container.NewWithoutLayout(w), func() { o.Move(geo.NewPos(60, 60)) }
		}, 1},
		{"own widget moved", func() (core.CanvasObject, func()) {
			w := &ownWidget{ownObject: ownObject{size: geo.NewSize(100, 100)}, children: []core.CanvasObject{rect(0, 0, 10, 10)}}
			return container.NewWithoutLayout(w), func() { w.Move(geo.NewPos(150, 150)) }
		}, 1},
		{"own object at the root", func() (core.CanvasObject, func()) {
			o := &ownObject{size: geo.NewSize(400, 400)}
			return o, func() { o.Resize(geo.NewSize(300, 300)) }
		}, 1},
		{"clip-owner scrolled off and back", func() (core.CanvasObject, func()) {
			owner := container.NewWithoutLayout(rect(-10, -10, 50, 50))
			owner.ClipToBounds = true
			owner.Resize(geo.NewSize(30, 30))
			owner.Move(geo.NewPos(40, 40))
			clip := container.NewWithoutLayout(owner)
			clip.ClipToBounds = true
			clip.Resize(geo.NewSize(200, 200))
			off := true
			return container.NewWithoutLayout(clip), func() {
				if off {
					owner.Move(geo.NewPos(300, 300))
				} else {
					owner.Move(geo.NewPos(40, 40))
				}
				off = !off
			}
		}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, change := tc.build()
			content.Resize(geo.NewSize(400, 400))
			c, ref := newTwinCanvases(content, geo.NewSize(400, 400))
			frame(t, c, ref, "first frame")
			frame(t, c, ref, "unchanged")
			for i := range tc.frames {
				change()
				frame(t, c, ref, fmt.Sprintf("change %d", i+1))
				frame(t, c, ref, fmt.Sprintf("after change %d", i+1))
			}
		})
	}
}

// TestPartialDirtyRegion_FromReportedRandom is TestPartialDirtyRegion_FromReported
// for random changes to a random tree.
func TestPartialDirtyRegion_FromReportedRandom(t *testing.T) {
	test.NewTempApp(t)

	frames, fewer := 0, 0
	defer func() {
		// Showing or transforming an object makes a frame walk everything:
		// most frames must not, or this tests nothing new.
		t.Logf("%d of %d frames walked from reported nodes", fewer, frames)
		assert.Greater(t, fewer, frames/2)
	}()
	for seed := int64(1); seed <= 40; seed++ {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rnd := rand.New(rand.NewSource(seed))
			pos := func() geo.Position { return geo.NewPos(float32(rnd.Intn(500)-50), float32(rnd.Intn(500)-50)) }
			size := func() geo.Size { return geo.NewSize(float32(1+rnd.Intn(120)), float32(1+rnd.Intn(120))) }

			var objects []core.CanvasObject
			var containers []*bellui.Container
			parents := map[core.CanvasObject]*bellui.Container{}
			leaf := func() core.CanvasObject {
				var o core.CanvasObject
				switch rnd.Intn(5) {
				case 0, 1:
					o = canvas.NewRectangle(color.White)
				case 2:
					o = canvas.NewText(fmt.Sprint("text ", rnd.Intn(1000)), color.White)
				case 3:
					o = widget.NewLabel(fmt.Sprint("label ", rnd.Intn(1000)))
				default:
					o = &ownObject{}
				}
				o.Move(pos())
				o.Resize(size())
				if rnd.Intn(5) == 0 {
					o.Hide()
				}
				objects = append(objects, o)
				return o
			}
			root := container.NewWithoutLayout()
			containers = append(containers, root)
			for range 4 {
				inner := container.NewWithoutLayout()
				inner.ClipToBounds = rnd.Intn(2) == 0
				inner.Move(pos())
				inner.Resize(size().AddWidthHeight(100, 100))
				root.Add(inner)
				parents[inner] = root
				containers = append(containers, inner)
				objects = append(objects, inner)
				for range 4 {
					o := leaf()
					inner.Add(o)
					parents[o] = inner
				}
			}
			root.Resize(geo.NewSize(400, 400))
			c, ref := newTwinCanvases(root, geo.NewSize(400, 400))
			frame(t, c, ref, "first frame")

			for step := range 40 {
				var did []string
				for range 1 + rnd.Intn(3) {
					o := objects[rnd.Intn(len(objects))]
					switch op := rnd.Intn(9); op {
					case 0, 1:
						o.Move(pos())
						did = append(did, fmt.Sprintf("move %T", o))
					case 2:
						o.Resize(size())
						did = append(did, fmt.Sprintf("resize %T", o))
					case 3:
						o.Hide()
						did = append(did, fmt.Sprintf("hide %T", o))
					case 4:
						o.Show()
						did = append(did, fmt.Sprintf("show %T", o))
					case 5:
						if p := parents[o]; p != nil {
							p.Remove(o)
							delete(parents, o)
							did = append(did, fmt.Sprintf("remove %T", o))
						}
					case 6:
						p := containers[rnd.Intn(len(containers))]
						n := leaf()
						p.Add(n)
						parents[n] = p
						did = append(did, fmt.Sprintf("add %T", n))
					case 7:
						if r, ok := o.(*canvas.Rectangle); ok {
							r.SetTransform(geo.NewScale(1+rnd.Float32(), 1+rnd.Float32()))
							did = append(did, "transform")
						}
					default:
						if txt, ok := o.(*canvas.Text); ok {
							txt.Text = fmt.Sprint("changed ", rnd.Intn(100000))
							txt.Refresh()
							did = append(did, "text")
						}
					}
				}
				frames++
				if frame(t, c, ref, fmt.Sprintf("step %d %v", step, did)) {
					fewer++
				}
				if t.Failed() {
					return
				}
			}
		})
	}
}

// TestPartialDirtyRegion_NodeAddedByAnotherWalk pins that a node another walk
// adds, such as the min-size pass finding children a layout added without
// reporting them, joins the next region.
func TestPartialDirtyRegion_NodeAddedByAnotherWalk(t *testing.T) {
	test.NewTempApp(t)

	inner := container.NewWithoutLayout(canvas.NewRectangle(color.White))
	inner.Resize(geo.NewSize(200, 200))
	content := container.NewWithoutLayout(inner)
	content.Resize(geo.NewSize(400, 400))
	c, ref := newTwinCanvases(content, geo.NewSize(400, 400))
	frame(t, c, ref, "first frame")

	added := canvas.NewRectangle(color.White)
	added.Move(geo.NewPos(120, 120))
	added.Resize(geo.NewSize(30, 30))
	inner.Objects = append(inner.Objects, added) // as a renderer's layout might
	for _, cv := range []*Canvas{c, ref} {
		cv.WalkTrees(nil, nil) // the min-size pass
	}
	frame(t, c, ref, "added")
}

// TestPartialDirtyRegion_WalksOnlyWhatChanged pins the point of #599: a frame
// where one object among many moved or was refreshed visits that object, not
// the tree.
func TestPartialDirtyRegion_WalksOnlyWhatChanged(t *testing.T) {
	test.NewTempApp(t)

	cards := make([]core.CanvasObject, 200)
	for i := range cards {
		cards[i] = container.NewVBox(widget.NewLabel(fmt.Sprint(i)), widget.NewButton("Open", func() {}))
	}
	mark := canvas.NewRectangle(color.White)
	mark.SetMinSize(geo.NewSquareSize(5))
	scroll := container.NewVScroll(container.NewGridWrap(geo.NewSize(180, 100), cards...))
	content := container.NewBorder(container.NewHBox(mark), nil, nil, nil, scroll)
	content.Resize(geo.NewSize(400, 800))
	c := newPartialCanvas(content, geo.NewSize(400, 800))
	c.PartialDirtyRegion()
	full := c.RegionVisits()

	_, _, set := c.PartialDirtyRegion()
	assert.False(t, set)
	assert.Zero(t, c.RegionVisits(), "an unchanged frame visits nothing")

	mark.Move(geo.NewPos(3, 0))
	_, _, set = c.PartialDirtyRegion()
	assert.True(t, set)
	assert.Equal(t, 1, c.RegionVisits(), "a moved object is visited alone (of %d)", full)

	mark.FillColor = color.Black
	mark.Refresh()
	_, _, set = c.PartialDirtyRegion()
	assert.True(t, set)
	assert.Equal(t, 1, c.RegionVisits(), "a refreshed object is visited alone (of %d)", full)

	grid := scroll.Content.(*bellui.Container)
	grid.Layout.Layout(grid.Objects, grid.Size()) // as a relayout does: every card moved to where it is
	_, _, set = c.PartialDirtyRegion()
	assert.False(t, set)
	assert.Zero(t, c.RegionVisits(), "a relayout that moves nothing visits nothing (of %d)", full)
}

// TestPartialDirtyRegion_OverlayIsNotARoot pins that a reported overlay node
// is left to the walk of the overlays, which covers it: walked as a root, it
// was taken for content, and an overlay that does not report made every
// region after it walk everything.
func TestPartialDirtyRegion_OverlayIsNotARoot(t *testing.T) {
	test.NewTempApp(t)

	objs := make([]core.CanvasObject, 100)
	for i := range objs {
		r := canvas.NewRectangle(color.White)
		r.Move(geo.NewPos(float32(i), 0))
		r.Resize(geo.NewSize(5, 5))
		objs[i] = r
	}
	mark := objs[0].(*canvas.Rectangle)
	content := container.NewWithoutLayout(objs...)
	content.Resize(geo.NewSize(400, 400))
	c := newPartialCanvas(content, geo.NewSize(400, 400))
	ov := &ownObject{pos: geo.NewPos(200, 200), size: geo.NewSize(30, 30)}
	c.Overlays().Add(ov)
	c.WalkTrees(nil, nil) // the overlay's node, as the min-size pass makes it
	c.PartialDirtyRegion()
	full := c.RegionVisits()

	refresh(t, c, ov)
	pos, size, set := c.PartialDirtyRegion()
	assert.True(t, set)
	assert.Equal(t, 1, c.RegionVisits(), "the overlay is walked alone (of %d)", full)
	assert.True(t, pos.X <= 200 && pos.Y <= 200 && pos.X+size.Width >= 230 && pos.Y+size.Height >= 230,
		"the region %v %v does not cover the refreshed overlay", pos, size)

	mark.Move(geo.NewPos(3, 3))
	c.PartialDirtyRegion()
	assert.Equal(t, 2, c.RegionVisits(), "a move after it visits the object and the overlay (of %d)", full)
}

// TestReportsPlacement_ListedTypesReport pins that every type reportsPlacement
// names tells its canvas when it is moved, resized or hidden: one that did not
// would have its changes missed by the region, which walks only from what
// reports. The list is read from reportsPlacement's source, so a type added
// there is checked here too.
func TestReportsPlacement_ListedTypesReport(t *testing.T) {
	test.NewTempApp(t)

	examples := map[string]core.CanvasObject{
		"bellui.Container":      container.NewWithoutLayout(),
		"canvas.Arc":            &canvas.Arc{},
		"canvas.BackdropBlur":   &canvas.BackdropBlur{},
		"canvas.BoxShadow":      &canvas.BoxShadow{},
		"canvas.Circle":         &canvas.Circle{},
		"canvas.GLView":         &canvas.GLView{},
		"canvas.Image":          &canvas.Image{},
		"canvas.Line":           &canvas.Line{},
		"canvas.LinearGradient": &canvas.LinearGradient{},
		"canvas.Path":           &canvas.Path{},
		"canvas.Polygon":        &canvas.Polygon{},
		"canvas.RadialGradient": &canvas.RadialGradient{},
		"canvas.Raster":         &canvas.Raster{},
		"canvas.Rectangle":      &canvas.Rectangle{},
		"canvas.Ripple":         &canvas.Ripple{},
		"canvas.Scene3D":        &canvas.Scene3D{},
		"canvas.StyledText":     &canvas.StyledText{},
		"canvas.Text":           &canvas.Text{},
		"canvas.TextOnPath":     &canvas.TextOnPath{},
	}
	// Spacer is listed, and reports nothing: it draws nothing, so where it is
	// changes no pixel.
	silent := map[string]bool{"layout.Spacer": true}

	listed := reportsPlacementCases(t)
	require.NotEmpty(t, listed)
	for _, name := range listed {
		if silent[name] {
			continue
		}
		obj, ok := examples[name]
		if !assert.True(t, ok, "%s is listed by reportsPlacement, and not checked here", name) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			require.True(t, reportsPlacement(obj))
			obj.Move(geo.NewPos(1, 1))
			obj.Resize(geo.NewSize(10, 10))
			content := container.NewWithoutLayout(obj)
			content.Resize(geo.NewSize(100, 100))
			c := newPartialCanvas(content, geo.NewSize(100, 100))
			c.PartialDirtyRegion()
			node, ok := c.NodeFor(obj)
			require.True(t, ok)
			for _, change := range []struct {
				name string
				do   func()
			}{
				{"move", func() { obj.Move(geo.NewPos(5, 5)) }},
				{"resize", func() { obj.Resize(geo.NewSize(20, 20)) }},
				{"hide", obj.Hide},
			} {
				c.takePlaced()
				change.do()
				assert.Contains(t, c.takePlaced(), node, "%s does not report a %s", name, change.name)
			}
		})
	}
}

// reportsPlacementCases returns the types reportsPlacement's type switch
// names, as package.Type.
func reportsPlacementCases(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "region.go", nil, 0)
	require.NoError(t, err)
	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "reportsPlacement" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if cc, ok := n.(*ast.CaseClause); ok {
				for _, e := range cc.List {
					if star, ok := e.(*ast.StarExpr); ok {
						if sel, ok := star.X.(*ast.SelectorExpr); ok {
							names = append(names, sel.X.(*ast.Ident).Name+"."+sel.Sel.Name)
						}
					}
				}
			}
			return true
		})
		return false
	})
	return names
}
