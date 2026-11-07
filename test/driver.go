package test

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/async"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/memcanvas"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/painter/software"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/internal/thread"
	"github.com/belluikit/bellui/storage/repository"
)

var (
	_ bellui.PositionMappingDriver = (*driver)(nil)
	_ bellui.OnScreenDriver        = (*driver)(nil)
)

// SoftwarePainter describes a simple type that can render canvases
type SoftwarePainter = memcanvas.Painter

type driver struct {
	device       device
	painter      SoftwarePainter
	windows      []bellui.Window
	windowsMutex sync.RWMutex
}

// doMutex serializes all Do/DoAndWait callbacks, matching the real drivers'
// guarantee that callbacks never run concurrently and establishing the
// happens-before edges that make background→UI flows testable under -race.
//
// It is package-level — shared across driver instances — rather than
// per-driver (pre-rebrand tracker #1049): tests in a package each create their own test app, so a
// goroutine leaked from (or still in flight at the end of) one test would
// otherwise hold the previous driver's mutex while the next test's work holds
// the new driver's, and nothing serializes the two. Tests are inherently
// serial per package, so over-serializing across instances costs nothing and
// matches the single-main-thread guarantee the real drivers provide globally.
var (
	doMutex sync.Mutex
	doOwner atomic.Uint64 // goroutine ID currently holding doMutex, for reentrancy
)

// Declare conformity with Driver
var _ bellui.Driver = (*driver)(nil)

// NewDriver sets up and registers a new dummy driver for test purpose
func NewDriver() bellui.Driver {
	drv := &driver{windowsMutex: sync.RWMutex{}}
	repository.Register("file", intRepo.NewFileRepository())

	httpHandler := intRepo.NewHTTPRepository()
	repository.Register("http", httpHandler)
	repository.Register("https", httpHandler)

	// make a single dummy window for rendering tests
	drv.CreateWindow("")

	return drv
}

// NewDriverWithPainter creates a new dummy driver that will pass the given
// painter to all canvases created
func NewDriverWithPainter(painter SoftwarePainter) bellui.Driver {
	return &driver{painter: painter}
}

// DoFromGoroutine on a test driver runs the function synchronously on the calling
// goroutine, but serialized under a driver-level mutex so that concurrent callers
// (a background worker using bellui.Do and a test reading via bellui.DoAndWait) are
// mutually exclusive with a happens-before edge between them, as on real drivers.
// The wait flag is ignored as execution is always synchronous; nested calls from
// within a running callback execute inline.
//
// The called-from-main check only applies once a main goroutine has been
// registered. Until then the mobile build reports every goroutine as main, and
// the check would hand each callback to a bare goroutine outside the mutex: Do
// callbacks then ran concurrently with each other and DoAndWait did not wait
// (#639).
func (d *driver) DoFromGoroutine(f func(), _ bool) {
	if !thread.MainGoroutineRegistered() {
		doSerialized(f)
		return
	}
	async.EnsureNotMain(func() { doSerialized(f) })
}

func doSerialized(f func()) {
	gid := thread.GoroutineID()
	if doOwner.Load() == gid {
		f() // nested bellui.Do[AndWait] from within a callback we are running
		return
	}

	doMutex.Lock()
	doOwner.Store(gid)
	defer func() {
		doOwner.Store(0)
		doMutex.Unlock()
	}()
	f()
}

func (d *driver) AbsolutePositionForObject(co core.CanvasObject) geo.Position {
	c := d.CanvasForObject(co)
	if c == nil {
		return geo.NewPos(0, 0)
	}

	tc := c.(*memcanvas.Canvas)
	pos := intdriver.AbsolutePositionForObject(co, tc.ObjectTrees())
	inset, _ := c.InteractiveArea()
	return pos.Subtract(inset)
}

// AbsolutePositionForPoint implements bellui.PositionMappingDriver.
func (d *driver) AbsolutePositionForPoint(co core.CanvasObject, pos geo.Position) geo.Position {
	c := d.CanvasForObject(co)
	if c == nil {
		return pos
	}
	inset, _ := c.InteractiveArea()
	return intdriver.AbsolutePositionForPoint(co, pos, c.(*memcanvas.Canvas).ObjectTrees()).Subtract(inset)
}

// PositionForAbsolutePoint implements bellui.PositionMappingDriver.
func (d *driver) PositionForAbsolutePoint(co core.CanvasObject, abs geo.Position) (geo.Position, bool) {
	c := d.CanvasForObject(co)
	if c == nil {
		return abs, false
	}
	inset, _ := c.InteractiveArea()
	return intdriver.PositionForAbsolutePoint(co, abs.Add(inset), c.(*memcanvas.Canvas).ObjectTrees())
}

func (d *driver) AllWindows() []bellui.Window {
	d.windowsMutex.RLock()
	defer d.windowsMutex.RUnlock()
	return d.windows
}

func (d *driver) CanvasForObject(core.CanvasObject) core.Canvas {
	d.windowsMutex.RLock()
	defer d.windowsMutex.RUnlock()
	if len(d.windows) == 0 {
		return nil // no window, so no canvas — as the mobile driver reports
	}
	// cheating: probably the last created window is meant
	return d.windows[len(d.windows)-1].Canvas()
}

// IsOnScreen implements bellui.OnScreenDriver: obj is reached from a
// window's content or overlays through visible objects and the renderers that
// exist. The test canvas keeps no render tree to answer from, as the real
// drivers do, so it walks. It builds no renderer: one that starts an animation
// would ask again from inside the walk.
func (d *driver) IsOnScreen(obj core.CanvasObject) bool {
	d.windowsMutex.RLock()
	windows := append([]bellui.Window(nil), d.windows...)
	d.windowsMutex.RUnlock()
	for _, w := range windows {
		c := w.Canvas()
		for _, root := range append([]core.CanvasObject{c.Content()}, c.Overlays().List()...) {
			if reachesVisible(root, obj) {
				return true
			}
		}
	}
	return false
}

// reachesVisible reports whether obj is o, or under it through visible objects.
func reachesVisible(o, obj core.CanvasObject) bool {
	if o == nil || !o.Visible() {
		return false
	}
	if o == obj {
		return true
	}
	var children []core.CanvasObject
	switch t := o.(type) {
	case *bellui.Container:
		children = t.Objects
	case core.Widget:
		if r, _ := cache.CachedRenderer(t); r != nil {
			children = r.Objects()
		}
	}
	for _, child := range children {
		if reachesVisible(child, obj) {
			return true
		}
	}
	return false
}

func (d *driver) CreateWindow(title string) bellui.Window {
	c := memcanvas.New()
	if d.painter != nil {
		c.SetPainter(d.painter)
	} else {
		c.SetPainter(software.NewPainter())
	}

	w := &window{canvas: c, driver: d, title: title}

	d.windowsMutex.Lock()
	d.windows = append(d.windows, w)
	d.windowsMutex.Unlock()
	return w
}

func (d *driver) Device() bellui.Device {
	return &d.device
}

// RenderedTextSize looks up how bit a string would be if drawn on screen
func (d *driver) RenderedTextSize(text string, size float32, style bellui.TextStyle, source bellui.Resource) (geo.Size, float32) {
	return painter.RenderedTextSize(text, size, style, source)
}

func (d *driver) Run() {
	// no-op
}

func (d *driver) StartAnimation(a *bellui.Animation) {
	// currently no animations in test app, we just initialise it and leave
	a.Tick(1.0)
}

func (d *driver) StopAnimation(a *bellui.Animation) {
	// currently no animations in test app, do nothing
}

func (d *driver) Quit() {
	// no-op
}

func (d *driver) Clipboard() bellui.Clipboard {
	return nil
}

func (d *driver) removeWindow(w *window) {
	d.windowsMutex.Lock()
	i := 0
	for _, win := range d.windows {
		if win == w {
			break
		}
		i++
	}

	copy(d.windows[i:], d.windows[i+1:])
	d.windows[len(d.windows)-1] = nil // Allow the garbage collector to reclaim the memory.
	d.windows = d.windows[:len(d.windows)-1]

	d.windowsMutex.Unlock()
}

func (d *driver) DoubleTapDelay() time.Duration {
	return 300 * time.Millisecond
}

func (d *driver) SetDisableScreenBlanking(_ bool) {
	// no-op for test
}
