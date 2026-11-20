package glfw

import (
	"testing"

	"github.com/belluikit/bellui"
)

// The desktop driver posts Do to its main loop, so it must not claim to run it
// inline (bellui.DoRunsInline, #1356).
func TestDriverDoesNotRunDoInline(t *testing.T) {
	if _, ok := any(&gLDriver{}).(bellui.InlineDoDriver); ok {
		t.Fatal("the glfw driver implements InlineDoDriver, but it posts Do to its main loop")
	}
}
