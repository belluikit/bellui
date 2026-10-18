package cache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	ret := m.Run()
	testClearAll()
	os.Exit(ret)
}

func TestCacheClean(t *testing.T) {
	destroyedRenderersCnt := 0
	testClearAll()
	tm := &timeMock{}

	for k := 0; k < 2; k++ {
		tm.setTime(10, 10+k*10)
		for i := 0; i < 20; i++ {
			SetSvg(fmt.Sprintf("%d%d", k, i), nil, nil, i, i+1)
			Renderer(&dummyWidget{onDestroy: func() {
				destroyedRenderersCnt++
			}})
			SetCanvasForObject(&dummyWidget{}, &dummyCanvas{}, nil)
		}
	}

	t.Run("no_expired_objects", func(t *testing.T) {
		lastClean.Store(tm.createTime(10, 20))
		Clean(false)
		assert.Equal(t, svgs.Len(), 40)
		assert.Equal(t, 40, renderers.Len())
		assert.Equal(t, 40, canvases.Len())
		assert.Zero(t, destroyedRenderersCnt)
		assert.Equal(t, tm.now, getLastClean())

		tm.setTime(10, 30)
		Clean(true)
		assert.Equal(t, svgs.Len(), 40)
		assert.Equal(t, 40, renderers.Len())
		assert.Equal(t, 40, canvases.Len())
		assert.Zero(t, destroyedRenderersCnt)
		assert.Equal(t, tm.now, getLastClean())
	})

	t.Run("do_not_clean_too_fast", func(t *testing.T) {
		lastClean.Store(tm.createTime(10, 30))
		// when no canvas refresh and has been transcurred less than
		// cleanTaskInterval duration, no clean task should occur.
		tm.setTime(10, 42)
		Clean(false)
		assert.Less(t, getLastClean().UnixNano(), tm.now.UnixNano())

		Clean(true)
		assert.Equal(t, tm.now, getLastClean())

		// when canvas refresh the clean task is only executed if it has been
		// transcurred more than 10 seconds since the lastClean.
		tm.setTime(10, 45)
		Clean(true)
		assert.Less(t, getLastClean().UnixNano(), tm.now.UnixNano())

		tm.setTime(10, 53)
		Clean(true)
		assert.Equal(t, tm.now, getLastClean())

		assert.Equal(t, svgs.Len(), 40)
		assert.Equal(t, 40, renderers.Len())
		assert.Equal(t, 40, canvases.Len())
		assert.Zero(t, destroyedRenderersCnt)
	})

	t.Run("clean_no_canvas_refresh", func(t *testing.T) {
		lastClean.Store(tm.createTime(10, 11))
		tm.setTime(11, 12)
		Clean(false)
		assert.Equal(t, svgs.Len(), 20)
		assert.Equal(t, renderers.Len(), 40)
		assert.Equal(t, canvases.Len(), 40)
		assert.Zero(t, destroyedRenderersCnt)

		tm.setTime(11, 42)
		Clean(false)
		assert.Equal(t, svgs.Len(), 0)
		assert.Equal(t, renderers.Len(), 40)
		assert.Equal(t, canvases.Len(), 40)
		assert.Zero(t, destroyedRenderersCnt)
	})

	t.Run("clean_canvas_refresh", func(t *testing.T) {
		lastClean.Store(tm.createTime(10, 11))
		tm.setTime(11, 11)
		Clean(true)
		assert.Equal(t, svgs.Len(), 0)
		assert.Equal(t, 20, renderers.Len())
		assert.Equal(t, 20, canvases.Len())
		assert.Equal(t, 20, destroyedRenderersCnt)

		tm.setTime(11, 22)
		Clean(true)
		assert.Equal(t, svgs.Len(), 0)
		assert.Equal(t, 0, renderers.Len())
		assert.Equal(t, 0, canvases.Len())
		assert.Equal(t, 40, destroyedRenderersCnt)
	})

	t.Run("skipped_clean_with_canvas_refresh", func(t *testing.T) {
		testClearAll()
		lastClean.Store(tm.createTime(13, 10))
		tm.setTime(13, 10)
		assert.False(t, skippedCleanWithCanvasRefresh.Load())
		Clean(true)
		assert.Equal(t, tm.now, getLastClean())

		Renderer(&dummyWidget{})

		tm.setTime(13, 15)
		Clean(true)
		assert.True(t, skippedCleanWithCanvasRefresh.Load())
		assert.Less(t, getLastClean().UnixNano(), tm.now.UnixNano())
		assert.Equal(t, 1, renderers.Len())

		tm.setTime(14, 21)
		Clean(false)
		assert.False(t, skippedCleanWithCanvasRefresh.Load())
		assert.Equal(t, tm.now, getLastClean())
		assert.Equal(t, 0, renderers.Len())
	})
}

func TestCleanCanvas(t *testing.T) {
	destroyedRenderersCnt := 0
	testClearAll()

	dcanvas1 := &dummyCanvas{}
	dcanvas2 := &dummyCanvas{}

	for i := 0; i < 20; i++ {
		dwidget := &dummyWidget{onDestroy: func() {
			destroyedRenderersCnt++
		}}
		Renderer(dwidget)
		SetCanvasForObject(dwidget, dcanvas1, nil)
	}

	for i := 0; i < 22; i++ {
		dwidget := &dummyWidget{onDestroy: func() {
			destroyedRenderersCnt++
		}}
		Renderer(dwidget)
		SetCanvasForObject(dwidget, dcanvas2, nil)
	}

	assert.Equal(t, 42, renderers.Len())
	assert.Equal(t, 42, canvases.Len())

	CleanCanvas(dcanvas1)
	assert.Equal(t, 22, renderers.Len())
	assert.Equal(t, 22, canvases.Len())
	assert.Equal(t, 20, destroyedRenderersCnt)
	canvases.Range(func(_ core.CanvasObject, cinfo *canvasInfo) bool {
		assert.Equal(t, dcanvas2, cinfo.canvas)
		return true
	})

	CleanCanvas(dcanvas2)
	assert.Equal(t, 0, renderers.Len())
	assert.Equal(t, 0, canvases.Len())
	assert.Equal(t, 42, destroyedRenderersCnt)
}

func Test_expiringCache(t *testing.T) {
	tm := &timeMock{}
	tm.setTime(10, 10)

	c := &expiringCache{}
	assert.True(t, c.isExpired(tm.now))

	c.setAlive()

	tm.setTime(10, 20)
	assert.False(t, c.isExpired(tm.now))

	tm.setTime(10, 11)
	tm.now = tm.now.Add(ValidDuration)
	assert.True(t, c.isExpired(tm.now))
}

// After a long idle, the paint that comes before Clean keeps the widgets on
// screen alive with the sample taken before the idle. Clean must not destroy
// their renderers; a widget nothing touches still expires (#458).
func TestCleanKeepsWidgetsTouchedAfterIdle(t *testing.T) {
	testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)
	lastClean.Store(tm.now)
	destroyed := 0
	shown := &dummyWidget{onDestroy: func() { destroyed++ }}
	Renderer(shown)

	// Two minutes without a frame, then a paint touches the widget.
	tm.now = tm.createTime(12, 0)
	Renderer(shown)
	Clean(true)
	assert.Zero(t, destroyed)
	assert.Equal(t, 1, renderers.Len())

	tm.setTime(13, 30)
	Clean(true)
	assert.Equal(t, 1, destroyed)
	assert.Zero(t, renderers.Len())
}

type dummyCanvas struct {
	core.Canvas
}

type dummyWidget struct {
	core.Widget
	onDestroy func()
}

func (w *dummyWidget) CreateRenderer() core.WidgetRenderer {
	return &dummyWidgetRenderer{widget: w}
}

type dummyWidgetRenderer struct {
	widget  *dummyWidget
	objects []core.CanvasObject
}

func (r *dummyWidgetRenderer) Destroy() {
	if r.widget.onDestroy != nil {
		r.widget.onDestroy()
	}
}

func (r *dummyWidgetRenderer) Layout(size geo.Size) {
}

func (r *dummyWidgetRenderer) MinSize() geo.Size {
	return geo.NewSize(0, 0)
}

func (r *dummyWidgetRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *dummyWidgetRenderer) Refresh() {
}

type timeMock struct {
	now time.Time
}

func (t *timeMock) createTime(min, sec int) time.Time {
	return time.Date(2021, time.June, 15, 2, min, sec, 0, time.UTC)
}

// setTime moves the clock and takes the per-frame sample at that time, as if a
// frame had been cleaned then.
func (t *timeMock) setTime(min, sec int) {
	t.now = time.Date(2021, time.June, 15, 2, min, sec, 0, time.UTC)
	timeNow = func() time.Time {
		return t.now
	}
	cachedNow.Store(t.now.UnixNano())
}

func testClearAll() {
	skippedCleanWithCanvasRefresh.Store(false)
	lastClean.Store(time.Time{})
	canvases.Clear()
	svgs.Clear()
	textTextures.Clear()
	objectTextures.Clear()
	renderers.Clear()
	timeNow = time.Now
	cachedNow.Store(time.Now().UnixNano())
}
