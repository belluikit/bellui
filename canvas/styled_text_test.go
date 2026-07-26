package canvas_test

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func TestStyledText_MinSize(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: "Hello"},
		{Text: " World"},
	}, color.Black)

	min := st.MinSize()
	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestStyledText_MinSize_MatchesSingleText(t *testing.T) {
	test.NewTempApp(t)

	// A single-run StyledText should measure the same as canvas.Text.
	text := "Hello World"
	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: text},
	}, color.Black)

	ct := canvas.NewText(text, color.Black)
	stMin := st.MinSize()
	ctMin := ct.MinSize()

	assert.InDelta(t, ctMin.Width, stMin.Width, 1)
	assert.InDelta(t, ctMin.Height, stMin.Height, 1)
}

func TestStyledText_MinSize_Empty(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText(nil, color.Black)
	min := st.MinSize()
	assert.Equal(t, geo.Size{}, min)
}

func TestStyledText_MinSize_MultipleRuns(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: "A"},
		{Text: "B"},
		{Text: "C"},
	}, color.Black)

	min := st.MinSize()
	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)

	// Width should be wider than a single character.
	single := canvas.NewStyledText([]canvas.TextRun{{Text: "A"}}, color.Black)
	singleMin := single.MinSize()
	assert.Greater(t, min.Width, singleMin.Width)
}

func TestStyledText_Move(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{{Text: "test"}}, color.Black)
	assert.Equal(t, geo.Position{}, st.Position())

	st.Move(geo.NewPos(10, 20))
	assert.Equal(t, geo.NewPos(10, 20), st.Position())
}

func TestStyledText_Resize(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{{Text: "test"}}, color.Black)
	st.Resize(geo.NewSize(200, 50))
	assert.Equal(t, geo.NewSize(200, 50), st.Size())
}

func TestStyledText_HideShow(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{{Text: "test"}}, color.Black)
	assert.True(t, st.Visible())

	st.Hide()
	assert.False(t, st.Visible())

	st.Show()
	assert.True(t, st.Visible())
}

func TestStyledText_DefaultTextSize(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{{Text: "test"}}, color.Black)
	// Should use theme text size by default.
	assert.Positive(t, st.TextSize)
}

func TestStyledText_RunColorFallback(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: "default color"},                           // nil Color → uses st.Color
		{Text: "red", Color: color.NRGBA{R: 255, A: 255}}, // explicit
	}, color.Black)

	// Just verify it doesn't panic when rendered.
	w := test.NewWindow(st)
	defer w.Close()
	w.Resize(geo.NewSize(300, 50))
	assert.NotNil(t, w.Canvas().Capture())
}

func TestStyledText_RunSizeFallback(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: "small", TextSize: 10},
		{Text: "default"}, // 0 → uses st.TextSize
		{Text: "large", TextSize: 24},
	}, color.Black)

	min := st.MinSize()
	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestStyledText_Alignment(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{{Text: "test"}}, color.Black)

	for _, align := range []bellui.TextAlign{
		bellui.TextAlignLeading,
		bellui.TextAlignCenter,
		bellui.TextAlignTrailing,
	} {
		st.Alignment = align
		w := test.NewWindow(st)
		w.Resize(geo.NewSize(300, 50))
		assert.NotNil(t, w.Canvas().Capture())
		w.Close()
	}
}

func TestStyledText_SetMinSize_NoOp(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{{Text: "Hello"}}, color.Black)
	originalMin := st.MinSize()
	st.SetMinSize(geo.NewSize(999, 999))
	// SetMinSize is a no-op for StyledText.
	assert.Equal(t, originalMin, st.MinSize())
}

func TestStyledText_RefreshNoPanic(t *testing.T) {
	st := canvas.NewStyledText([]canvas.TextRun{{Text: "test"}}, color.Black)
	assert.NotPanics(t, func() { st.Refresh() })
}

func TestStyledText_BoldRun(t *testing.T) {
	test.NewTempApp(t)

	normal := canvas.NewStyledText([]canvas.TextRun{
		{Text: "Hello"},
	}, color.Black)
	bold := canvas.NewStyledText([]canvas.TextRun{
		{Text: "Hello", TextStyle: bellui.TextStyle{Bold: true}},
	}, color.Black)

	// Bold text is typically wider.
	assert.Greater(t, bold.MinSize().Width, normal.MinSize().Width)
}

func TestStyledText_MixedSizes(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: "Small", TextSize: 10},
		{Text: "Large", TextSize: 30},
	}, color.Black)

	min := st.MinSize()
	// Height should be at least as tall as the largest run.
	singleLarge := canvas.NewStyledText([]canvas.TextRun{
		{Text: "Large", TextSize: 30},
	}, color.Black)
	assert.GreaterOrEqual(t, min.Height, singleLarge.MinSize().Height)
}

func TestStyledText_EmptyRuns(t *testing.T) {
	test.NewTempApp(t)

	st := canvas.NewStyledText([]canvas.TextRun{
		{Text: ""},
		{Text: "Hello"},
		{Text: ""},
	}, color.Black)

	single := canvas.NewStyledText([]canvas.TextRun{
		{Text: "Hello"},
	}, color.Black)

	// Empty runs should not affect the size.
	assert.InDelta(t, single.MinSize().Width, st.MinSize().Width, 1)
}
