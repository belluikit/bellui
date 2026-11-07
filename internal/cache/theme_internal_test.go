package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
)

// An owned scope stays while its owner is rendered, and for ValidDuration
// before and after; only then does the sweep release it, container and all
// (#1065).
func TestOwnedScopeLivesWhileItsOwnerIsRendered(t *testing.T) {
	testClearAll()
	defer testClearAll()
	tm := &timeMock{}
	tm.setTime(10, 0)

	owner := &dummyWidget{}
	box := &bellui.Container{Objects: []core.CanvasObject{&dummyWidget{}}}
	OverrideThemeFor(box, nil, owner)
	scope := WidgetScopeID(box)
	assert.NotEmpty(t, scope)

	tm.setTime(10, 30)
	destroyExpiredScopes(tm.now)
	assert.Equal(t, scope, WidgetScopeID(box), "a scope whose owner is not rendered yet went before ValidDuration")

	Renderer(owner)
	tm.setTime(15, 0)
	destroyExpiredScopes(tm.now)
	assert.Equal(t, scope, WidgetScopeID(box), "the scope of a rendered owner was released")

	DestroyRenderer(owner)
	tm.setTime(15, 30)
	destroyExpiredScopes(tm.now)
	assert.Equal(t, scope, WidgetScopeID(box), "the scope went less than ValidDuration after its owner's renderer")

	tm.setTime(16, 30)
	destroyExpiredScopes(tm.now)
	assert.Empty(t, WidgetScopeID(box), "the container kept the scope of an owner gone for ValidDuration")
	assert.Zero(t, ownedScopes.Len())
}

// Applying a new scope for an owner releases the one it replaces, so the
// objects that left the owner's content between the two go with it.
func TestOwnedScopeReplacedReleasesTheOldOne(t *testing.T) {
	testClearAll()
	defer testClearAll()

	owner := &dummyWidget{}
	gone := &dummyWidget{}
	box := &bellui.Container{Objects: []core.CanvasObject{gone}}
	OverrideThemeFor(box, nil, owner)
	assert.NotEmpty(t, WidgetScopeID(gone))

	box.Objects = nil
	OverrideThemeFor(box, nil, owner)
	assert.NotEmpty(t, WidgetScopeID(box))
	assert.Empty(t, WidgetScopeID(gone), "an object that left the content kept the replaced scope")
	assert.Equal(t, 1, ownedScopes.Len())
}
