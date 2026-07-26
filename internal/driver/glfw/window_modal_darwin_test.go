//go:build darwin && !no_glfw && !ci && !mobile

package glfw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWindow_ModalBlocksParentKey verifies the #676 macOS parent-key
// suppression: while a child dialog is modal, the parent NSWindow is prevented
// from becoming key/main (so the keyboard cannot reach it), and the block is
// lifted when the dialog stops being modal or is torn down.
func TestWindow_ModalBlocksParentKey(t *testing.T) {
	var parent, child *window
	runOnMain(func() {
		parent = d.CreateWindow("TestModalKeyParent").(*window)
		parent.create()
		parent.viewport.Show()
		require.False(t, parent.modalBlocked(), "parent starts unblocked")

		child = d.CreateWindow("TestModalKeyChild").(*window)
		child.SetParent(parent)
		child.create()

		child.SetModal(true)
		child.applyModal()
		assert.True(t, parent.modalBlocked(), "modal child blocks the parent from becoming key")

		child.SetModal(false)
		child.applyModal()
		assert.False(t, parent.modalBlocked(), "clearing modality restores the parent")

		// Re-block, then verify teardown (releaseModal) also restores the parent.
		child.SetModal(true)
		child.applyModal()
		require.True(t, parent.modalBlocked())
		child.releaseModal()
		assert.False(t, parent.modalBlocked(), "teardown restores the parent")
	})
}

// TestWindow_ModalStackedBlocksParentKey verifies the #682 ref-count: two modal
// dialogs over the same parent keep it blocked until the last one clears — the
// first closing must not lift the OS-level block while the second is still up.
func TestWindow_ModalStackedBlocksParentKey(t *testing.T) {
	var parent, child1, child2 *window
	runOnMain(func() {
		parent = d.CreateWindow("StackParent").(*window)
		parent.create()
		parent.viewport.Show()

		child1 = d.CreateWindow("StackChild1").(*window)
		child1.SetParent(parent)
		child1.create()
		child2 = d.CreateWindow("StackChild2").(*window)
		child2.SetParent(parent)
		child2.create()

		child1.SetModal(true)
		child1.applyModal()
		require.True(t, parent.modalBlocked(), "first modal child blocks the parent")

		child2.SetModal(true)
		child2.applyModal()
		require.True(t, parent.modalBlocked(), "second modal child keeps it blocked")

		// Closing the first modal child must NOT unblock the parent while the
		// second is still modal.
		child1.releaseModal()
		assert.True(t, parent.modalBlocked(), "parent stays blocked while a second modal child is open")

		child2.releaseModal()
		assert.False(t, parent.modalBlocked(), "parent unblocks only when the last modal child clears")
	})
}
