package widget

import (
	"image/color"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	internalWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
)

type myWidget struct {
	DisableableWidget

	refreshed chan bool
}

func (m *myWidget) Refresh() {
	m.refreshed <- true
}

func (m *myWidget) CreateRenderer() core.WidgetRenderer {
	m.ExtendBaseWidget(m)
	return internalWidget.NewSimpleRenderer(&bellui.Container{})
}

func TestApplyThemeCalled(t *testing.T) {
	widget := &myWidget{refreshed: make(chan bool, 10)}

	test.NewTempWindow(t, widget)
	bellui.CurrentApp().Settings().SetTheme(test.NewTheme())

	func() {
		select {
		case <-widget.refreshed:
		case <-time.After(1 * time.Second):
			assert.Fail(t, "Timed out waiting for theme apply")
		}
	}()
}

func TestApplyThemeCalledChild(t *testing.T) {
	child := &myWidget{refreshed: make(chan bool, 10)}
	parent := &bellui.Container{Layout: layout.NewVBoxLayout(), Objects: []core.CanvasObject{child}}

	test.NewTempWindow(t, parent)
	bellui.CurrentApp().Settings().SetTheme(test.NewTheme())
	func() {
		select {
		case <-child.refreshed:
		case <-time.After(1 * time.Second):
			assert.Fail(t, "Timed out waiting for child theme apply")
		}
	}()
}

// themeRaceWidget is a minimal BaseWidget-backed widget used to exercise the
// #604 theme-cache race in isolation.
type themeRaceWidget struct {
	BaseWidget
}

func (w *themeRaceWidget) CreateRenderer() core.WidgetRenderer {
	w.ExtendBaseWidget(w)
	return internalWidget.NewSimpleRenderer(&bellui.Container{})
}

// TestBaseWidgetThemeRefreshRace reproduces the #604 data race: a layout pass
// reads BaseWidget.themeCache through Theme()/MinSize() on one goroutine while
// Refresh() clears the same field on another. With the plain-interface field
// this trips the race detector at widget.go:244; the atomic.Pointer field keeps
// it clean. Meaningful only under `go test -race`.
func TestBaseWidgetThemeRefreshRace(t *testing.T) {
	test.NewTempApp(t)

	w := &themeRaceWidget{}
	w.ExtendBaseWidget(w)
	// Warm the renderer on this goroutine so the concurrent loops below race on
	// the theme cache, not on first-time renderer creation.
	w.MinSize()
	w.Refresh()

	const iterations = 3000
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = w.Theme()
			_ = w.MinSize()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			w.Refresh()
		}
	}()
	wg.Wait()
}

func TestSimpleRenderer(t *testing.T) {
	test.NewTempApp(t)

	c := &bellui.Container{Layout: layout.NewStackLayout(), Objects: []core.CanvasObject{
		newTestWidget(canvas.NewRectangle(color.Gray{Y: 0x79})),
		newTestWidget(canvas.NewText("Hi", color.Black)),
	}}

	window := test.NewWindow(c)
	defer window.Close()

	test.AssertImageMatches(t, "simple_renderer.png", window.Canvas().Capture())
}

type testWidget struct {
	BaseWidget
	obj core.CanvasObject
}

func newTestWidget(o core.CanvasObject) core.Widget {
	t := &testWidget{obj: o}
	t.ExtendBaseWidget(t)
	return t
}

func (t *testWidget) CreateRenderer() core.WidgetRenderer {
	return NewSimpleRenderer(t.obj)
}

func waitForBinding() {
	time.Sleep(time.Millisecond * 100) // data resolves on background thread
}
