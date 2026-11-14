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

// A wrapping or truncating label in Center or in a column that does not
// stretch it keeps its text on one line while it fits, and wraps or truncates
// at the container's width when it does not (#1198). It used to collapse to one
// character wide.
func TestLabel_FitWidthInCenterAndColumns(t *testing.T) {
	test.NewTempApp(t)
	layouts := map[string]func() core.Layout{
		"Center":      layout.NewCenterLayout,
		"CrossStart":  func() core.Layout { return &layout.FlexVBoxLayout{CrossAlign: layout.CrossStart} },
		"CrossCenter": func() core.Layout { return &layout.FlexVBoxLayout{CrossAlign: layout.CrossCenter} },
		"CrossEnd":    func() core.Layout { return &layout.FlexVBoxLayout{CrossAlign: layout.CrossEnd} },
	}
	for name, newLayout := range layouts {
		for _, truncate := range []bool{false, true} {
			lbl := widget.NewLabel("Your code: K7Q-42X · Friends: 2")
			msg := name
			if truncate {
				lbl.Truncation = bellui.TextTruncateEllipsis
				msg += " truncating"
			} else {
				lbl.Wrapping = bellui.TextWrapWord
			}
			box := &bellui.Container{Layout: newLayout(), Objects: []core.CanvasObject{lbl}}
			w := test.NewTempWindow(t, box)
			w.SetPadded(false)

			oneLine := lbl.IntrinsicSize()
			for _, width := range []float32{400, 120} {
				w.Resize(geo.NewSize(width, 300))
				w.Resize(geo.NewSize(width, 301))
				if width > oneLine.Width {
					assert.InDelta(t, oneLine.Width, lbl.Size().Width, 0.01, "%s: one line at %v", msg, width)
					assert.InDelta(t, oneLine.Height, lbl.Size().Height, 0.01, "%s: one line high at %v", msg, width)
					continue
				}
				assert.InDelta(t, width, lbl.Size().Width, 0.01, "%s: the container's width at %v", msg, width)
				assert.InDelta(t, lbl.MinSize().Height, lbl.Size().Height, 0.01, "%s: as high as its lines at %v", msg, width)
				if !truncate {
					assert.Greater(t, lbl.Size().Height, oneLine.Height, "%s: wraps at %v", msg, width)
				}
			}
		}
	}
}

// A wrapping label in a RowWrap flow of chips keeps its text on one line
// beside them while it fits, and wraps at the flow's width on a row of its
// own when it does not; the chip rows keep their own height (#1199).
func TestLabel_FitWidthInRowWrap(t *testing.T) {
	test.NewTempApp(t)
	lbl := widget.NewLabel("Filters apply to every playlist you open")
	lbl.Wrapping = bellui.TextWrapWord
	before := widget.NewButton("Rock", nil)
	after := widget.NewButton("Jazz", nil)
	flow := &bellui.Container{Layout: layout.NewRowWrapLayout(), Objects: []core.CanvasObject{before, lbl, after}}
	w := test.NewTempWindow(t, flow)
	w.SetPadded(false)

	oneLine := lbl.IntrinsicSize()
	w.Resize(geo.NewSize(1000, 300))
	w.Resize(geo.NewSize(1000, 301))
	assert.InDelta(t, oneLine.Width, lbl.Size().Width, 0.01, "one line")
	assert.InDelta(t, 0, lbl.Position().Y, 0.01, "beside the chip")

	w.Resize(geo.NewSize(150, 300))
	w.Resize(geo.NewSize(150, 301))
	assert.InDelta(t, 150, lbl.Size().Width, 0.01, "the flow's width")
	assert.Greater(t, lbl.Size().Height, oneLine.Height, "wraps")
	assert.InDelta(t, before.MinSize().Height+theme.Padding(), lbl.Position().Y, 0.01, "on the row after the chip, which keeps its own height")
	assert.InDelta(t, lbl.Position().Y+lbl.Size().Height+theme.Padding(), after.Position().Y, 0.01, "the next chip below it")
}

// A wrapping or truncating label in a FormLayout's label column keeps its
// text on one line while the field beside it keeps half the width, and wraps
// or truncates at half the width when it does not (#1202). It used to collapse
// to one character wide.
func TestLabel_FitWidthInFormLabelColumn(t *testing.T) {
	test.NewTempApp(t)
	for _, truncate := range []bool{false, true} {
		lbl := widget.NewLabel("Your code: K7Q-42X · Friends: 2")
		msg := "wrapping"
		if truncate {
			lbl.Truncation = bellui.TextTruncateEllipsis
			msg = "truncating"
		} else {
			lbl.Wrapping = bellui.TextWrapWord
		}
		field := widget.NewEntry()
		form := &bellui.Container{Layout: layout.NewFormLayout(), Objects: []core.CanvasObject{lbl, field, widget.NewLabel("Name"), widget.NewEntry()}}
		w := test.NewTempWindow(t, form)
		w.SetPadded(false)

		oneLine := lbl.IntrinsicSize()
		pad := theme.Padding()
		for _, width := range []float32{500, 200} {
			w.Resize(geo.NewSize(width, 300))
			w.Resize(geo.NewSize(width, 301))
			half := (width - pad) / 2
			if oneLine.Width <= half {
				assert.InDelta(t, oneLine.Width, lbl.Size().Width, 0.01, "%s: one line at %v", msg, width)
				assert.InDelta(t, oneLine.Height, lbl.MinSize().Height, 0.01, "%s: one line high at %v", msg, width)
			} else {
				assert.InDelta(t, half, lbl.Size().Width, 0.01, "%s: half the width at %v", msg, width)
				if !truncate {
					assert.Greater(t, lbl.MinSize().Height, oneLine.Height, "%s: wraps at %v", msg, width)
				}
			}
			assert.InDelta(t, lbl.Size().Width+pad, field.Position().X, 0.01, "%s: field after the label at %v", msg, width)
			assert.GreaterOrEqual(t, field.Size().Width, half-0.01, "%s: field keeps half at %v", msg, width)
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
