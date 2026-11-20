//go:build !wasm

package mobile

import (
	"testing"

	"github.com/belluikit/bellui"
)

// The mobile driver posts Do to its run loop, so it must not claim to run it
// inline (bellui.DoRunsInline, #1356).
func TestDriverDoesNotRunDoInline(t *testing.T) {
	if _, ok := any(&driver{}).(bellui.InlineDoDriver); ok {
		t.Fatal("the mobile driver implements InlineDoDriver, but it posts Do to its run loop")
	}
}
