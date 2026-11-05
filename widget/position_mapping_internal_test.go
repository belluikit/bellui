package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// shiftBox is a widget with a transform, holding one object where it is put.
type shiftBox struct {
	BaseWidget
	m   geo.Matrix
	obj core.CanvasObject
}

func newShiftBox(m geo.Matrix, obj core.CanvasObject) *shiftBox {
	b := &shiftBox{m: m, obj: obj}
	b.ExtendBaseWidget(b)
	return b
}

func (b *shiftBox) Transform() geo.Matrix     { return b.m }
func (b *shiftBox) SetTransform(m geo.Matrix) { b.m = m }

func (b *shiftBox) CreateRenderer() core.WidgetRenderer {
	return NewSimpleRenderer(&bellui.Container{Objects: []core.CanvasObject{b.obj}})
}

// A Select moved by a transform opens its list below where it is drawn, not
// below where layout put it (#1003).
func TestSelect_PopUpFollowsTransform(t *testing.T) {
	test.NewTempApp(t)
	sel := NewSelect([]string{"a", "b"}, nil)
	box := newShiftBox(geo.NewTranslation(40, 30), sel)
	w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{box}})
	w.Resize(geo.NewSize(300, 300))
	box.Resize(geo.NewSize(150, 40))
	sel.Move(geo.NewPos(10, 5))
	sel.Resize(geo.NewSize(120, 30))

	laidOut := bellui.CurrentApp().Driver().AbsolutePositionForObject(sel)
	assert.Equal(t, laidOut.AddXY(40, 30), bellui.AbsolutePositionForPoint(sel, geo.Position{}))

	border := sel.Theme().Size(theme.SizeNameInputBorder)
	assert.Equal(t, laidOut.AddXY(40, 30+30-border), sel.popUpPos())
}
