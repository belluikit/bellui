package canvas_test

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"os"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/stretchr/testify/assert"
)

func TestText_FontSource(t *testing.T) {
	// Discarding log output for tests
	// The following method logs an error:
	// test.NewWindow(text).Canvas().Capture()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	text := canvas.NewText("Test", color.NRGBA{0, 0, 0, 0xff})
	c := test.NewWindow(text).Canvas()

	text.FontSource = test.Theme().Font(bellui.TextStyle{Bold: true})
	img1 := c.Capture()
	text.FontSource = test.Theme().Font(bellui.TextStyle{Italic: true})
	img2 := c.Capture()
	assert.NotEqual(t, img1.(*image.NRGBA).Pix, img2.(*image.NRGBA).Pix)

	text.FontSource = bellui.NewStaticResource("corrupt", []byte{})
	assert.NotPanics(t, func() {
		test.NewWindow(text).Canvas().Capture()
	})
}

func TestText_MinSize(t *testing.T) {
	text := canvas.NewText("Test", color.NRGBA{0, 0, 0, 0xff})
	min := text.MinSize()

	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)

	text = canvas.NewText("Test2", color.NRGBA{0, 0, 0, 0xff})
	min2 := text.MinSize()
	assert.Greater(t, min2.Width, min.Width)
}

func TestText_MinSize_NoMultiLine(t *testing.T) {
	text := canvas.NewText("Break", color.NRGBA{0, 0, 0, 0xff})
	min := text.MinSize()

	text = canvas.NewText("Bre\nak", color.NRGBA{0, 0, 0, 0xff})
	min2 := text.MinSize()
	assert.Greater(t, min2.Width, min.Width)
	assert.True(t, min2.Height == min.Height)
}

func TestText_Layout(t *testing.T) {
	test.NewTempApp(t)

	for name, tt := range map[string]struct {
		text  string
		align bellui.TextAlign
		size  geo.Size
	}{
		"short_leading_small": {
			text:  "abc",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(1, 1),
		},
		"short_leading_large": {
			text:  "abc",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(500, 101),
		},
		"long_leading_small": {
			text:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(1, 1),
		},
		"long_leading_large": {
			text:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(500, 101),
		},
		"short_center_small": {
			text:  "abc",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(1, 1),
		},
		"short_center_large": {
			text:  "abc",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(500, 101),
		},
		"long_center_small": {
			text:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(1, 1),
		},
		"long_center_large": {
			text:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(500, 101),
		},
		"short_trailing_small": {
			text:  "abc",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(1, 1),
		},
		"short_trailing_large": {
			text:  "abc",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(500, 101),
		},
		"long_trailing_small": {
			text:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(1, 1),
		},
		"long_trailing_large": {
			text:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(500, 101),
		},
	} {
		t.Run(name, func(t *testing.T) {
			text := canvas.NewText(tt.text, theme.Color(theme.ColorNameForeground))
			text.Alignment = tt.align
			text.Resize(tt.size)

			test.AssertObjectRendersToImage(t, "text/layout_"+name+".png", text)
		})
	}
}

func TestText_SetRotation(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("Test", color.NRGBA{0, 0, 0, 0xff})
	assert.Equal(t, float32(0), text.Rotation())

	text.SetRotation(90)
	assert.Equal(t, float32(90), text.Rotation())

	text.SetRotation(-45)
	assert.Equal(t, float32(-45), text.Rotation())

	text.SetRotation(0)
	assert.Equal(t, float32(0), text.Rotation())
}

func TestText_SetRotation_ClearsOnSetTransform(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("Test", color.NRGBA{0, 0, 0, 0xff})
	text.SetRotation(45)
	assert.Equal(t, float32(45), text.Rotation())

	text.SetTransform(geo.Identity())
	assert.Equal(t, float32(0), text.Rotation())
}

func TestText_SetRotation_Transform(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("Test", color.NRGBA{0, 0, 0, 0xff})
	min := text.MinSize()

	text.SetRotation(90)
	m := text.Transform()

	// Rotation center should be the text's visual center.
	cx, cy := min.Width/2, min.Height/2

	// The center should map to itself under rotation.
	rx, ry := m.TransformPoint(cx, cy)
	assert.InDelta(t, float64(cx), float64(rx), 0.01)
	assert.InDelta(t, float64(cy), float64(ry), 0.01)

	// A 90-degree rotation should map (cx+1, cy) → (cx, cy+1).
	rx, ry = m.TransformPoint(cx+1, cy)
	assert.InDelta(t, float64(cx), float64(rx), 0.01)
	assert.InDelta(t, float64(cy+1), float64(ry), 0.01)
}

func TestText_SetRotation_UpdatesOnTextChange(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("A", color.NRGBA{0, 0, 0, 0xff})
	text.SetRotation(90)

	m1 := text.Transform()
	// Center of "A"
	s1 := text.MinSize()
	cx1, cy1 := s1.Width/2, s1.Height/2

	// Change text content — rotation center should update.
	text.Text = "ABCDEFGHIJ"
	m2 := text.Transform()
	s2 := text.MinSize()
	cx2, cy2 := s2.Width/2, s2.Height/2

	// Different text → different center → different matrix.
	assert.NotEqual(t, cx1, cx2)

	// But both matrices should rotate around their respective centers.
	rx, ry := m1.TransformPoint(cx1, cy1)
	assert.InDelta(t, float64(cx1), float64(rx), 0.01)
	assert.InDelta(t, float64(cy1), float64(ry), 0.01)

	rx, ry = m2.TransformPoint(cx2, cy2)
	assert.InDelta(t, float64(cx2), float64(rx), 0.01)
	assert.InDelta(t, float64(cy2), float64(ry), 0.01)
}

func TestText_MinSize_UnchangedByRotation(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("Hello World", color.NRGBA{0, 0, 0, 0xff})
	min0 := text.MinSize()

	text.SetRotation(45)
	min45 := text.MinSize()

	// MinSize should NOT change with rotation — transforms are visual only.
	assert.Equal(t, min0, min45)
}

func TestText_SubPixel(t *testing.T) {
	text := canvas.NewText("Test", color.NRGBA{0, 0, 0, 0xff})
	assert.False(t, text.SubPixel)

	text.SubPixel = true
	assert.True(t, text.SubPixel)
}

func TestText_RotationMatrix_Angles(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("XY", color.NRGBA{0, 0, 0, 0xff})
	s := text.MinSize()
	cx, cy := float64(s.Width/2), float64(s.Height/2)

	for _, deg := range []float32{0, 30, 45, 90, 180, 270, -90, -45} {
		t.Run(fmt.Sprintf("%gdeg", deg), func(t *testing.T) {
			text.SetRotation(deg)

			if deg == 0 {
				// Zero rotation should yield identity.
				m := text.Transform()
				assert.True(t, m.IsIdentity())
				return
			}

			m := text.Transform()
			// Center maps to itself.
			rx, ry := m.TransformPoint(float32(cx), float32(cy))
			assert.InDelta(t, cx, float64(rx), 0.01)
			assert.InDelta(t, cy, float64(ry), 0.01)

			// Origin maps to expected rotated position.
			rad := float64(deg) * math.Pi / 180
			expectX := cx + (-cx)*math.Cos(rad) - (-cy)*math.Sin(rad)
			expectY := cy + (-cx)*math.Sin(rad) + (-cy)*math.Cos(rad)
			rx, ry = m.TransformPoint(0, 0)
			assert.InDelta(t, expectX, float64(rx), 0.01)
			assert.InDelta(t, expectY, float64(ry), 0.01)
		})
	}
}

func TestText_CarriageReturn(t *testing.T) {
	test.NewTempApp(t)

	for name, tt := range map[string]struct {
		text  string
		align bellui.TextAlign
		size  geo.Size
	}{
		"short_leading_small": {
			text:  "\ra\rb\rc\r",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(1, 1),
		},
		"short_leading_large": {
			text:  "\ra\rb\rc\r",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(500, 101),
		},
		"long_leading_small": {
			text:  "\ra\rb\rc\rd\re\rf\rg\rh\ri\rj\rk\rl\rm\rn\ro\rp\rq\rr\rs\rt\ru\rv\rw\rx\ry\rz\rA\rB\rC\rD\rE\rF\rG\rH\rI\rJ\rK\rL\rM\rN\rO\rP\rQ\rR\rS\rT\rU\rV\rW\rX\rY\rZ\r",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(1, 1),
		},
		"long_leading_large": {
			text:  "\ra\rb\rc\rd\re\rf\rg\rh\ri\rj\rk\rl\rm\rn\ro\rp\rq\rr\rs\rt\ru\rv\rw\rx\ry\rz\rA\rB\rC\rD\rE\rF\rG\rH\rI\rJ\rK\rL\rM\rN\rO\rP\rQ\rR\rS\rT\rU\rV\rW\rX\rY\rZ\r",
			align: bellui.TextAlignLeading,
			size:  geo.NewSize(500, 101),
		},
		"short_center_small": {
			text:  "\ra\rb\rc\r",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(1, 1),
		},
		"short_center_large": {
			text:  "\ra\rb\rc\r",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(500, 101),
		},
		"long_center_small": {
			text:  "\ra\rb\rc\rd\re\rf\rg\rh\ri\rj\rk\rl\rm\rn\ro\rp\rq\rr\rs\rt\ru\rv\rw\rx\ry\rz\rA\rB\rC\rD\rE\rF\rG\rH\rI\rJ\rK\rL\rM\rN\rO\rP\rQ\rR\rS\rT\rU\rV\rW\rX\rY\rZ\r",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(1, 1),
		},
		"long_center_large": {
			text:  "\ra\rb\rc\rd\re\rf\rg\rh\ri\rj\rk\rl\rm\rn\ro\rp\rq\rr\rs\rt\ru\rv\rw\rx\ry\rz\rA\rB\rC\rD\rE\rF\rG\rH\rI\rJ\rK\rL\rM\rN\rO\rP\rQ\rR\rS\rT\rU\rV\rW\rX\rY\rZ\r",
			align: bellui.TextAlignCenter,
			size:  geo.NewSize(500, 101),
		},
		"short_trailing_small": {
			text:  "\ra\rb\rc\r",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(1, 1),
		},
		"short_trailing_large": {
			text:  "\ra\rb\rc\r",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(500, 101),
		},
		"long_trailing_small": {
			text:  "\ra\rb\rc\rd\re\rf\rg\rh\ri\rj\rk\rl\rm\rn\ro\rp\rq\rr\rs\rt\ru\rv\rw\rx\ry\rz\rA\rB\rC\rD\rE\rF\rG\rH\rI\rJ\rK\rL\rM\rN\rO\rP\rQ\rR\rS\rT\rU\rV\rW\rX\rY\rZ\r",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(1, 1),
		},
		"long_trailing_large": {
			text:  "\ra\rb\rc\rd\re\rf\rg\rh\ri\rj\rk\rl\rm\rn\ro\rp\rq\rr\rs\rt\ru\rv\rw\rx\ry\rz\rA\rB\rC\rD\rE\rF\rG\rH\rI\rJ\rK\rL\rM\rN\rO\rP\rQ\rR\rS\rT\rU\rV\rW\rX\rY\rZ\r",
			align: bellui.TextAlignTrailing,
			size:  geo.NewSize(500, 101),
		},
	} {
		t.Run(name, func(t *testing.T) {
			text := canvas.NewText(tt.text, theme.Color(theme.ColorNameForeground))
			text.Alignment = tt.align
			text.Resize(tt.size)

			test.AssertObjectRendersToImage(t, "text/layout_"+name+".png", text)
		})
	}
}

func TestMeasureTextFull(t *testing.T) {
	test.NewTempApp(t)

	size, baseline := bellui.MeasureTextFull("Hello", 14, bellui.TextStyle{}, nil)
	assert.Positive(t, size.Width)
	assert.Positive(t, size.Height)
	assert.Positive(t, baseline)
	assert.LessOrEqual(t, baseline, size.Height)
}

func TestMeasureTextFull_MatchesMeasureText(t *testing.T) {
	test.NewTempApp(t)

	s1 := bellui.MeasureText("Test", 14, bellui.TextStyle{})
	s2, _ := bellui.MeasureTextFull("Test", 14, bellui.TextStyle{}, nil)
	assert.Equal(t, s1, s2)
}

func TestMeasureText_Standalone(t *testing.T) {
	// MeasureText should work via the registered fallback even without a running app.
	// The painter package registers its RenderedTextSize via init(), which has already
	// run because the test package imports it.
	//
	// We don't call test.NewTempApp(t), so there is no current app.
	size := bellui.MeasureText("Standalone", 14, bellui.TextStyle{})
	assert.Positive(t, size.Width)
	assert.Positive(t, size.Height)
}
