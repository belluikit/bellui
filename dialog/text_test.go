package dialog

import (
	"testing"

	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/test"
)

// TestCreateBeforeShowHook_ReleasesMeasuringLabel checks that a text dialog
// leaves no renderer behind for the label it builds only to measure its
// message (#722).
func TestCreateBeforeShowHook_ReleasesMeasuringLabel(t *testing.T) {
	test.NewTempApp(t)
	before := cache.RenderedWidgets()

	_ = createBeforeShowHook(&dialog{}, "a message that is measured once")

	for w := range cache.RenderedWidgets() {
		if _, ok := before[w]; !ok {
			t.Errorf("renderer left behind for a %T", w)
		}
	}
}
