package embedded

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"

	"github.com/belluikit/bellui/driver/software"
)

type touchCanvas struct {
	software.WindowlessCanvas
	common.TouchHandler
}

func newTouchCanvas() *touchCanvas {
	ret := &touchCanvas{
		WindowlessCanvas: software.NewCanvas(),
	}
	ret.InitTouchHandler(&embeddedFinder{canvas: ret})
	ret.LongPressCallback = func(wid event.LongPressable, ev *event.PointEvent) {
		wid.LongPressed(ev)
	}
	return ret
}

// embeddedFinder adapts the embedded canvas to the ObjectFinder interface.
type embeddedFinder struct {
	canvas *touchCanvas
}

func (f *embeddedFinder) FindObjectAtPositionMatching(pos geo.Position, test func(core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	return intdriver.FindObjectAtPositionMatching(pos, test, nil, nil, f.canvas.Content())
}

func (f *embeddedFinder) CanvasForTouch() core.Canvas {
	return f.canvas.WindowlessCanvas
}
