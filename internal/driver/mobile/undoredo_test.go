package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
)

// fakeActioner declares a configurable accessible-action set (pre-rebrand tracker #1134).
type fakeActioner struct {
	actions   []core.AccessibleAction
	performed []string
}

func (f *fakeActioner) AccessibleActions() []core.AccessibleAction { return f.actions }
func (f *fakeActioner) PerformAccessibleAction(name string) {
	f.performed = append(f.performed, name)
}

// The iOS undo-state mirror (pre-rebrand tracker #1134) derives canUndo/canRedo from the focused
// editor's declared accessible actions — the same gated declarations assistive
// tech sees (pre-rebrand tracker #1025) — and reports none for non-actioners.
func TestUndoActionsState(t *testing.T) {
	canUndo, canRedo := undoActionsState(nil)
	assert.False(t, canUndo)
	assert.False(t, canRedo)

	canUndo, canRedo = undoActionsState("not an actioner")
	assert.False(t, canUndo)
	assert.False(t, canRedo)

	canUndo, canRedo = undoActionsState(&fakeActioner{})
	assert.False(t, canUndo, "no declared actions")
	assert.False(t, canRedo)

	canUndo, canRedo = undoActionsState(&fakeActioner{actions: []core.AccessibleAction{
		{Name: core.ActionUndo},
	}})
	assert.True(t, canUndo)
	assert.False(t, canRedo)

	canUndo, canRedo = undoActionsState(&fakeActioner{actions: []core.AccessibleAction{
		{Name: core.ActionUndo},
		{Name: core.ActionRedo},
		{Name: core.ActionPress}, // unrelated actions are ignored
	}})
	assert.True(t, canUndo)
	assert.True(t, canRedo)
}
