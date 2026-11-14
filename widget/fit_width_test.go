package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/layout"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/bellui/widget"
)

// An unstretched wrapping or truncating label beside a button keeps its text
// on one line with the button right after it while the row has room, and
// wraps or truncates with the button still beside it when it has not
// (#1195). It used to collapse to one character wide.
func TestLabel_FitWidthBesideAButton(t *testing.T) {
	test.NewTempApp(t)
	layouts := map[string]func() core.Layout{
		"FlexHBox": func() core.Layout { return layout.NewFlexHBoxLayout([]float32{0, 0}) },
		"HBox":     layout.NewHBoxLayout,
	}
	for name, newLayout := range layouts {
		for _, truncate := range []bool{false, true} {
			lbl := widget.NewLabel("Your code: K7Q-42X · Friends: 2")
			if truncate {
				lbl.Truncation = bellui.TextTruncateEllipsis
			} else {
				lbl.Wrapping = bellui.TextWrapWord
			}
			btn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), nil)
			row := &bellui.Container{Layout: newLayout(), Objects: []core.CanvasObject{lbl, btn}}
			w := test.NewTempWindow(t, row)
			w.SetPadded(false)

			oneLine := lbl.IntrinsicSize()
			pad := theme.Padding()
			for _, c := range []struct {
				width float32
				fits  bool
			}{{400, true}, {120, false}} {
				// Wrapped text's height settles on the second layout pass, as
				// it does for a stretched label.
				w.Resize(geo.NewSize(c.width, 300))
				w.Resize(geo.NewSize(c.width, 301))
				msg := name
				if truncate {
					msg += " truncating"
				}
				if c.fits {
					assert.InDelta(t, oneLine.Width, lbl.Size().Width, 0.01, "%s: one line at %v", msg, c.width)
					assert.InDelta(t, oneLine.Height, lbl.MinSize().Height, 0.01, "%s: one line high at %v", msg, c.width)
				} else {
					assert.Less(t, lbl.Size().Width, oneLine.Width, "%s: narrower than one line at %v", msg, c.width)
					assert.Greater(t, lbl.Size().Width, lbl.MinSize().Width, "%s: wider than one character at %v", msg, c.width)
					if !truncate {
						assert.Greater(t, lbl.MinSize().Height, oneLine.Height, "%s: wraps at %v", msg, c.width)
					}
				}
				assert.InDelta(t, lbl.Size().Width+pad, btn.Position().X, 0.01, "%s: button right after the label at %v", msg, c.width)
				assert.LessOrEqual(t, btn.Position().X+btn.Size().Width, c.width+0.01, "%s: button on screen at %v", msg, c.width)
			}
		}
	}
}

// Editable text is not a FitWidther: an entry beside other widgets would grow
// as it is typed in.
func TestEntry_IsNotAFitWidther(t *testing.T) {
	var e core.CanvasObject = widget.NewEntry()
	_, ok := e.(core.FitWidther)
	assert.False(t, ok)
	var l core.CanvasObject = widget.NewLabel("")
	_, ok = l.(core.FitWidther)
	assert.True(t, ok)
}
