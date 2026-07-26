package inspector

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

func rowsMap(rows []detailRow) map[string]string {
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Label] = r.Value
	}
	return m
}

func TestDescribe_BasicAndAccessibility(t *testing.T) {
	test.NewTempApp(t)

	lbl := widget.NewLabel("hi")
	lbl.Move(geo.NewPos(3, 4))
	lbl.Resize(geo.NewSize(50, 20))
	n := &node{
		obj:      lbl,
		typeName: "*widget.Label",
		kind:     kindWidget,
		absPos:   geo.NewPos(3, 4),
		size:     geo.NewSize(50, 20),
		visible:  true,
	}

	got := rowsMap(describe(n))
	assert.Equal(t, "*widget.Label", got["Type"])
	assert.Equal(t, "widget", got["Kind"])
	assert.Equal(t, "true", got["Visible"])
	assert.Equal(t, "50 × 20", got["Size"])
	assert.Contains(t, got, "MinSize")
	assert.Equal(t, "(3,4) 50×20", got["Abs bounds"])
	assert.Equal(t, "Label", got["A11y role"]) // Label implements core.Accessible
}

func TestDescribe_ScrollOffset(t *testing.T) {
	test.NewTempApp(t)

	sc := container.NewScroll(widget.NewLabel("x"))
	sc.Resize(geo.NewSize(50, 50))
	sc.Offset = geo.NewPos(7, 9)

	n := &node{obj: sc, typeName: "*container.Scroll", kind: kindWidget, size: geo.NewSize(50, 50), visible: true}
	got := rowsMap(describe(n))
	assert.Equal(t, "(7, 9)", got["Scroll offset"])
	assert.Contains(t, got, "Scroll content")
}

func TestDescribe_DirectionScope(t *testing.T) {
	test.NewTempApp(t)

	ds := container.NewDirectionScope(widget.NewLabel("x"), bellui.DirectionRTL)
	n := &node{obj: ds, typeName: "*container.DirectionScope", kind: kindWidget, visible: true}
	got := rowsMap(describe(n))
	assert.Equal(t, "RTL", got["Direction scope"])
}

func TestDescribe_NilNode(t *testing.T) {
	assert.Nil(t, describe(nil))
}
