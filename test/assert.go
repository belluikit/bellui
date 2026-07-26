package test

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/stretchr/testify/assert"
)

// AssertWidgetSize asserts that a widget's current size matches the expected size.
// Uses approximate comparison (within 1 pixel) to handle float rounding.
func AssertWidgetSize(t *testing.T, obj core.CanvasObject, expectedW, expectedH float32) bool {
	t.Helper()
	s := obj.Size()
	ok := approxEq(s.Width, expectedW) && approxEq(s.Height, expectedH)
	if !ok {
		t.Errorf("expected size (%.0f, %.0f), got (%.0f, %.0f)", expectedW, expectedH, s.Width, s.Height)
	}
	return ok
}

// AssertWidgetPosition asserts that a widget's current position matches expected values.
func AssertWidgetPosition(t *testing.T, obj core.CanvasObject, expectedX, expectedY float32) bool {
	t.Helper()
	p := obj.Position()
	ok := approxEq(p.X, expectedX) && approxEq(p.Y, expectedY)
	if !ok {
		t.Errorf("expected position (%.0f, %.0f), got (%.0f, %.0f)", expectedX, expectedY, p.X, p.Y)
	}
	return ok
}

// AssertMinSizeAtLeast asserts that a widget's MinSize is at least the given dimensions.
// Useful for checking that a widget has reasonable minimum size without exact values.
func AssertMinSizeAtLeast(t *testing.T, obj core.CanvasObject, minW, minH float32) bool {
	t.Helper()
	s := obj.MinSize()
	ok := s.Width >= minW && s.Height >= minH
	if !ok {
		t.Errorf("expected MinSize >= (%.0f, %.0f), got (%.0f, %.0f)", minW, minH, s.Width, s.Height)
	}
	return ok
}

// AssertVisible asserts that the object is visible.
func AssertVisible(t *testing.T, obj core.CanvasObject) bool {
	t.Helper()
	return assert.True(t, obj.Visible(), "expected widget to be visible")
}

// AssertHidden asserts that the object is hidden.
func AssertHidden(t *testing.T, obj core.CanvasObject) bool {
	t.Helper()
	return assert.False(t, obj.Visible(), "expected widget to be hidden")
}

// AssertEnabled asserts that a Disableable widget is enabled.
func AssertEnabled(t *testing.T, obj core.Disableable) bool {
	t.Helper()
	return assert.False(t, obj.Disabled(), "expected widget to be enabled")
}

// AssertDisabled asserts that a Disableable widget is disabled.
func AssertDisabled(t *testing.T, obj core.Disableable) bool {
	t.Helper()
	return assert.True(t, obj.Disabled(), "expected widget to be disabled")
}

// AssertFitsWithin asserts that a widget's size fits within the given bounds.
// Useful for layout tests without exact size matching.
func AssertFitsWithin(t *testing.T, obj core.CanvasObject, maxW, maxH float32) bool {
	t.Helper()
	s := obj.Size()
	ok := s.Width <= maxW && s.Height <= maxH
	if !ok {
		t.Errorf("expected size <= (%.0f, %.0f), got (%.0f, %.0f)", maxW, maxH, s.Width, s.Height)
	}
	return ok
}

func approxEq(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1.5
}
