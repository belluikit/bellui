package widget_test

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"

	"github.com/stretchr/testify/assert"
)

// choiceGroups is a horizontal and a vertical RadioGroup and CheckGroup.
func choiceGroups() (radioH, radioV *widget.RadioGroup, checkH, checkV *widget.CheckGroup, all core.CanvasObject) {
	radioH = widget.NewRadioGroup([]string{"Start", "Center", "End"}, nil)
	radioH.Horizontal = true
	radioH.SetSelected("Center")
	radioV = widget.NewRadioGroup([]string{"Start", "Center", "End"}, nil)
	radioV.SetSelected("Center")
	checkH = widget.NewCheckGroup([]string{"One", "Two"}, nil)
	checkH.Horizontal = true
	checkH.SetSelected([]string{"One"})
	checkV = widget.NewCheckGroup([]string{"One", "Two"}, nil)
	checkV.SetSelected([]string{"Two"})
	return radioH, radioV, checkH, checkV, container.NewVBox(radioH, radioV, checkH, checkV)
}

// Right-to-left, a radio item puts its circle right of its label, as a Check
// does, and horizontal groups run from the right edge, first option
// rightmost. A radio item's circle used to stay on the left while its label
// moved, onto the next option (#1392). Left-to-right is unchanged.
func TestChoiceGroups_Direction(t *testing.T) {
	for _, c := range []struct {
		name string
		dir  bellui.Direction
	}{{"ltr", bellui.DirectionLTR}, {"rtl", bellui.DirectionRTL}} {
		t.Run(c.name, func(t *testing.T) {
			test.NewTempApp(t)
			bellui.SetLayoutDirection(c.dir)
			defer bellui.SetLayoutDirection(bellui.DirectionAuto)
			radioH, _, checkH, _, all := choiceGroups()
			w := test.NewTempWindow(t, all)
			w.Resize(geo.NewSize(400, 330))
			test.AssertRendersToImage(t, "choice_groups/"+c.name+".png", w.Canvas())

			// The first option's item, against the last's.
			first := func(o core.CanvasObject) (float32, float32) {
				objs := test.TempWidgetRenderer(t, o.(core.Widget)).Objects()
				return objs[0].Position().X, objs[len(objs)-1].Position().X
			}
			for _, g := range []core.CanvasObject{radioH, checkH} {
				x0, xn := first(g)
				if c.dir == bellui.DirectionRTL {
					assert.Greater(t, x0, xn, "%T: right-to-left, the first option is rightmost", g)
				} else {
					assert.Less(t, x0, xn, "%T: left-to-right, the first option is leftmost", g)
				}
			}
		})
	}
}
