//go:build !no_glfw && !ci && !mobile

package glfw

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"

	"github.com/belluikit/glfw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindow_RequestPosition(t *testing.T) {
	w := createWindow("Test")

	runOnMain(func() {
		w.RequestPosition(120, 130)

		x, y := w.Position()
		assert.Equal(t, 120, x)
		assert.Equal(t, 130, y)
	})
}

func TestWindow_RequestPosition_BeforeCreate(t *testing.T) {
	var w *window
	runOnMain(func() {
		w = d.CreateWindow("TestPos").(*window)
		w.RequestPosition(140, 160) // no view yet — stored and applied at create
		w.create()

		x, y := w.Position()
		assert.Equal(t, 140, x)
		assert.Equal(t, 160, y)
	})
}

func TestWindow_OnMoved(t *testing.T) {
	w := createWindow("Test")

	var gotX, gotY, calls int
	runOnMain(func() {
		w.SetOnMoved(func(x, y int) {
			gotX, gotY = x, y
			calls++
		})

		w.window.moved(w.viewport, 15, 25)
		assert.Equal(t, 1, calls)
		assert.Equal(t, 15, gotX)
		assert.Equal(t, 25, gotY)

		x, y := w.Position()
		assert.Equal(t, 15, x)
		assert.Equal(t, 25, y)

		// the re-seat to the monitor origin when entering fullscreen is not a
		// move of the window the user placed: not reported, not recorded
		w.fullScreen = true
		w.window.moved(w.viewport, 0, 0)
		assert.Equal(t, 1, calls)

		x, y = w.Position()
		assert.Equal(t, 15, x)
		assert.Equal(t, 25, y)

		w.fullScreen = false
		w.SetOnMoved(nil)
	})
}

func TestWindow_RequestAlwaysOnTop(t *testing.T) {
	var w *window
	runOnMain(func() {
		w = d.CreateWindow("TestOnTop").(*window)
		w.RequestAlwaysOnTop() // before create/Show, as documented
		w.create()

		assert.Equal(t, glfw.True, w.viewport.GetAttrib(glfw.Floating))
	})
}

func TestWindow_SetParent(t *testing.T) {
	var parent, child *window
	runOnMain(func() {
		parent = d.CreateWindow("TestParent").(*window)
		parent.create()
		parent.viewport.Show()

		child = d.CreateWindow("TestChild").(*window)
		_, ok := any(child).(desktop.ChildWindow)
		require.True(t, ok, "glfw windows expose desktop.ChildWindow")

		child.SetParent(parent) // before create: stored, applied at show
		require.NotNil(t, child.parent)

		child.create()
		child.applyParent() // native owner call — must not panic

		child.SetParent(nil) // clearing the relationship is also safe
		child.applyParent()
	})
}

func TestWindow_SetModal(t *testing.T) {
	var parent, child *window
	runOnMain(func() {
		parent = d.CreateWindow("TestModalParent").(*window)
		parent.create()
		parent.viewport.Show()

		child = d.CreateWindow("TestModalChild").(*window)
		_, ok := any(child).(desktop.ModalWindow)
		require.True(t, ok, "glfw windows expose desktop.ModalWindow")

		child.SetParent(parent)
		child.SetModal(true) // before create: stored, applied at show
		require.True(t, child.modal)

		child.create()
		child.applyModal() // native modal call — must not panic

		// releaseModal lifts modality and clears the flag so a disabled parent
		// is never left orphaned at teardown.
		child.releaseModal()
		assert.False(t, child.modal)

		// Disabling and re-applying with no resolved parent are both safe.
		child.SetModal(false)
		child.applyModal()
	})
}

func TestWindow_ModalRefCount(t *testing.T) {
	var parent, c1, c2 *window
	runOnMain(func() {
		parent = d.CreateWindow("RefParent").(*window)
		c1 = d.CreateWindow("RefChild1").(*window)
		c2 = d.CreateWindow("RefChild2").(*window)

		// Only the 0<->1 transition toggles the native parent block; extra
		// modal children keep it blocked without re-toggling (pre-rebrand tracker #682).
		assert.True(t, c1.blockParentModal(parent, true), "first block is the 0->1 transition")
		assert.Equal(t, 1, parent.modalChildren)
		assert.False(t, c1.blockParentModal(parent, true), "repeated block by the same child is a no-op")
		assert.Equal(t, 1, parent.modalChildren)

		assert.False(t, c2.blockParentModal(parent, true), "a second modal child does not re-toggle")
		assert.Equal(t, 2, parent.modalChildren)

		assert.False(t, c1.blockParentModal(parent, false), "unblocking one of two is not the 1->0 transition")
		assert.Equal(t, 1, parent.modalChildren)
		assert.False(t, c1.blockParentModal(parent, false), "repeated unblock by the same child is a no-op")
		assert.Equal(t, 1, parent.modalChildren)

		assert.True(t, c2.blockParentModal(parent, false), "unblocking the last child is the 1->0 transition")
		assert.Equal(t, 0, parent.modalChildren)

		// A stray unblock from a child that never blocked must not go negative.
		assert.False(t, c1.blockParentModal(parent, false))
		assert.Equal(t, 0, parent.modalChildren)
	})
}

func TestGLDriver_Screens(t *testing.T) {
	// Verify that the glfw driver implements the desktop extensions.
	var driver bellui.Driver = d
	sd, ok := driver.(desktop.ScreensDriver)
	assert.True(t, ok)

	runOnMain(func() {
		screens := sd.Screens()
		if !assert.NotEmpty(t, screens) {
			return
		}

		primaries := 0
		for _, s := range screens {
			assert.Positive(t, s.Bounds.Width)
			assert.Positive(t, s.Bounds.Height)
			assert.Positive(t, s.WorkArea.Width)
			assert.Positive(t, s.WorkArea.Height)
			assert.Positive(t, s.Scale)
			if s.Primary {
				primaries++
			}
		}
		assert.GreaterOrEqual(t, primaries, 1)

		if len(screens) == 1 {
			assert.False(t, d.HasSecondaryDisplay())
		}
	})
}
