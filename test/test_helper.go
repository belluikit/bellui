//go:build !tamago && !noos

package test

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/painter/software"
	"github.com/belluikit/bellui/internal/test"
)

// resolveMasterFilename, on mobile, prefers a platform-specific master under
// <dir>/mobile/<base> when one exists, so widgets that render differently for
// touch (e.g. the always-wide scrollbar) don't diverge the shared desktop
// masters. Falls back to the shared master when no mobile variant is present.
func resolveMasterFilename(wd, name string) string {
	if !onMobileTestBuild() {
		return name
	}
	dir, base := filepath.Split(name)
	mobileName := filepath.Join(dir, "mobile", base)
	if _, err := os.Stat(filepath.Join(wd, "testdata", mobileName)); err == nil {
		return mobileName
	}
	return name
}

// AssertCanvasTappableAt asserts that the canvas is tappable at the given position.
func AssertCanvasTappableAt(t *testing.T, c core.Canvas, pos geo.Position) bool {
	if o, _ := findTappable(c, pos); o == nil {
		t.Errorf("No tappable found at %#v", pos)
		return false
	}
	return true
}

// AssertObjectRendersToImage asserts that the given `CanvasObject` renders the same image as the one stored in the master file.
// The theme used is the standard test theme which may look different to how it shows on your device.
// The master filename is relative to the `testdata` directory which is relative to the test.
// The test `t` fails if the given image is not equal to the loaded master image.
// In this case the given image is written into a file in `testdata/failed/<masterFilename>` (relative to the test).
// This path is also reported, thus the file can be used as new master.
//
// Caveat: the canvas is sized to exactly the object's own bounds (MinSize maxed with Size),
// so anything the object's renderer draws outside those bounds is cropped by the canvas image
// and will not appear in the comparison. A widget that overflows its bounds therefore renders
// to a clean, cropped master — looking as if its content were correctly clipped even when it is not.
// To verify overflow or clipping behaviour, render the object inside a larger canvas (place it in a
// sized container and use [AssertRendersToImage]), and use bellui's clipping API
// (e.g. container.NewClip) to opt a subtree into clipping.
//
// Since: 1.0
func AssertObjectRendersToImage(t *testing.T, masterFilename string, o core.CanvasObject, msgAndArgs ...any) bool {
	c := NewCanvasWithPainter(software.NewPainter())
	c.SetPadded(false)
	size := o.MinSize().Max(o.Size())
	c.SetContent(o)
	c.Resize(size) // ensure we are large enough for current size

	return AssertRendersToImage(t, masterFilename, c, msgAndArgs...)
}

// AssertObjectRendersToMarkup asserts that the given `CanvasObject` renders the same markup as the one stored in the master file.
// The master filename is relative to the `testdata` directory which is relative to the test.
// The test `t` fails if the rendered markup is not equal to the loaded master markup.
// In this case the rendered markup is written into a file in `testdata/failed/<masterFilename>` (relative to the test).
// This path is also reported, thus the file can be used as new master.
//
// Be aware, that the indentation has to use tab characters ('\t') instead of spaces.
// Every element starts on a new line indented one more than its parent.
// Closing elements stand on their own line, too, using the same indentation as the opening element.
// The only exception to this are text elements which do not contain line breaks unless the text includes them.
//
// Caveat: as with [AssertObjectRendersToImage], the canvas is sized to exactly the object's own
// bounds, so markup for content drawn outside those bounds is not captured. Render the object inside
// a larger canvas (via [AssertRendersToMarkup]) to observe overflow.
//
// Since: 1.0
func AssertObjectRendersToMarkup(t *testing.T, masterFilename string, o core.CanvasObject, msgAndArgs ...any) bool {
	c := NewCanvas()
	c.SetPadded(false)
	size := o.MinSize().Max(o.Size())
	c.SetContent(o)
	c.Resize(size) // ensure we are large enough for current size

	return AssertRendersToMarkup(t, masterFilename, c, msgAndArgs...)
}

// AssertImageMatches asserts that the given image is the same as the one stored in the master file.
// The master filename is relative to the `testdata` directory which is relative to the test.
// The test `t` fails if the given image is not equal to the loaded master image.
// In this case the given image is written into a file in `testdata/failed/<masterFilename>` (relative to the test).
// This path is also reported, thus the file can be used as new master.
func AssertImageMatches(t *testing.T, masterFilename string, img image.Image, msgAndArgs ...any) bool {
	return test.AssertImageMatches(t, masterFilename, img, msgAndArgs...)
}

// AssertRendersToImage asserts that the given canvas renders the same image as the one stored in the master file.
// The master filename is relative to the `testdata` directory which is relative to the test.
// The test `t` fails if the given image is not equal to the loaded master image.
// In this case the given image is written into a file in `testdata/failed/<masterFilename>` (relative to the test).
// This path is also reported, thus the file can be used as new master.
//
// Since: 1.0
func AssertRendersToImage(t *testing.T, masterFilename string, c core.Canvas, msgAndArgs ...any) bool {
	return AssertImageMatches(t, masterFilename, c.Capture(), msgAndArgs...)
}

// AssertRendersToMarkup asserts that the given canvas renders the same markup as the one stored in the master file.
// The master filename is relative to the `testdata` directory which is relative to the test.
// The test `t` fails if the rendered markup is not equal to the loaded master markup.
// In this case the rendered markup is written into a file in `testdata/failed/<masterFilename>` (relative to the test).
// This path is also reported, thus the file can be used as new master.
//
// Be aware, that the indentation has to use tab characters ('\t') instead of spaces.
// Every element starts on a new line indented one more than its parent.
// Closing elements stand on their own line, too, using the same indentation as the opening element.
// The only exception to this are text elements which do not contain line breaks unless the text includes them.
//
// Since: 1.0
func AssertRendersToMarkup(t *testing.T, masterFilename string, c core.Canvas, msgAndArgs ...any) bool {
	wd, err := os.Getwd()
	require.NoError(t, err)

	got := snapshot(c)
	masterFilename = resolveMasterFilename(wd, masterFilename)
	masterPath := filepath.Join(wd, "testdata", masterFilename)
	failedPath := filepath.Join(wd, "testdata/failed", masterFilename)

	// Auto-update mode.
	if os.Getenv("UPDATE_TESTDATA") != "" {
		require.NoError(t, writeMarkup(masterPath, got))
		t.Logf("Updated master: %s", masterPath)
		return true
	}

	_, err = os.Stat(masterPath)
	if os.IsNotExist(err) {
		require.NoError(t, writeMarkup(failedPath, got))
		t.Errorf("Master not found at %s. Markup written to %s might be used as master.\n"+
			"Run with UPDATE_TESTDATA=1 to create it automatically.", masterPath, failedPath)
		return false
	}

	raw, err := os.ReadFile(masterPath)
	require.NoError(t, err)
	master := strings.ReplaceAll(string(raw), "\r", "")

	var msg string
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprintf(msgAndArgs[0].(string)+"\n", msgAndArgs[1:]...)
	}
	if !assert.Equal(t, master, got, "%sMarkup did not match master. Actual markup written to file://%s.\n"+
		"Run with UPDATE_TESTDATA=1 to update the master.", msg, failedPath) {
		require.NoError(t, writeMarkup(failedPath, got))
		return false
	}
	return true
}

// themeApplyTimeout bounds how long the test theme helpers wait for an async
// theme change to propagate. Theme application normally completes synchronously,
// but a stuck settings listener could otherwise spin the wait forever; the bound
// makes such a stall fail fast with a clear message instead of hanging (pre-rebrand tracker #37).
const themeApplyTimeout = 5 * time.Second

// themeApplied polls until the app reports want as its applied theme, returning
// true once it does or false if timeout elapses first. The bound is what stops a
// stalled settings apply from spinning forever.
func themeApplied(a *app, want bellui.Theme, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for a.lastAppliedTheme() != want {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

// waitForTheme blocks until the app reports want as its applied theme, or fails
// the test if that has not happened within themeApplyTimeout. Safe to call from
// a t.Cleanup or deferred restore as well as the main test body.
func waitForTheme(t *testing.T, a *app, want bellui.Theme) {
	t.Helper()
	if !themeApplied(a, want, themeApplyTimeout) {
		t.Errorf("theme was not applied within %v (settings apply likely stalled)", themeApplyTimeout)
	}
}

// ApplyTheme sets the given theme and waits for it to be applied to the current app.
// The previous theme is automatically restored when the test completes.
func ApplyTheme(t *testing.T, theme bellui.Theme) {
	require.IsType(t, &app{}, bellui.CurrentApp())
	a := bellui.CurrentApp().(*app)
	previous := a.Settings().Theme()
	a.Settings().SetTheme(theme)
	waitForTheme(t, a, theme)
	t.Cleanup(func() {
		a.Settings().SetTheme(previous)
		waitForTheme(t, a, previous)
	})
}

// TempWidgetRenderer allows test scripts to gain access to the current renderer for a widget.
// This can be used for verifying correctness of rendered components for a widget in unit tests.
// The widget renderer is automatically destroyed when the test ends.
//
// Since: 1.0
func TempWidgetRenderer(t *testing.T, wid core.Widget) core.WidgetRenderer {
	t.Cleanup(func() { cache.DestroyRenderer(wid) })
	return cache.Renderer(wid)
}

// DestroyRenderers releases the cached renderers of o and of every object
// under it, the way closing a window releases the renderers of its content.
// Widgets outside the tree keep theirs.
//
// Code that builds widgets without a running driver needs it: a benchmark, or
// any test whose widgets are never shown in a window that later closes. Their
// renderers otherwise stay cached, with everything they reference, for the
// rest of the process (#720).
//
// The renderers are destroyed when DestroyRenderers is called, not at
// tb.Cleanup: a benchmark calls it at the end of each iteration, so the heap
// stays near where it started however many trees the benchmark builds, while
// a cleanup would hold every tree until the benchmark returned. To release a
// tree when a test ends, call it from tb.Cleanup yourself.
//
// The walk reaches the children of each [github.com/belluikit/bellui.Container]
// and the objects of each widget's cached renderer, hidden ones included. It
// never creates a renderer: a widget without one is skipped, and so are
// objects only that widget's renderer would have held. Popups and overlays
// are not under o and are not released, and neither are the items a
// collection widget keeps in its pool of unused items (#740). A widget used
// again afterwards gets a new renderer, as it would after its window closed.
//
// Since: 1.3
func DestroyRenderers(tb testing.TB, o core.CanvasObject) {
	tb.Helper()
	intdriver.DestroyRenderers(o)
}

// WithTestTheme runs a function with the testTheme temporarily set.
// The original theme is restored immediately after f returns.
func WithTestTheme(t *testing.T, f func()) {
	require.IsType(t, &app{}, bellui.CurrentApp())
	a := bellui.CurrentApp().(*app)
	previous := a.Settings().Theme()
	th := NewTheme()
	a.Settings().SetTheme(th)
	waitForTheme(t, a, th)
	defer func() {
		a.Settings().SetTheme(previous)
		waitForTheme(t, a, previous)
	}()
	f()
}
