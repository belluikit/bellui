//go:build !wasm

package mobile

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	intdriver "github.com/belluikit/bellui/internal/driver"
)

// A min-size pass over 1000 rows, with a DirectionScope rendered elsewhere in
// the window, never looks a direction up: the walk pushes the resolved
// direction at its root, so a Refresh or Resize inside it finds one ambient
// (#760). The rows' labels change before the pass so that it does lay them
// out again: the visit count is the marker that the walk ran.
func TestEnsureMinSize_NoDirectionLookupInsideWalk(t *testing.T) {
	s := newDirectionBenchScene(true)
	bellui.DoAndWait(func() {
		for i, l := range s.labels {
			l.SetText("changed " + strconv.Itoa(i) + " with a longer text")
		}
		builds, lookups := intdriver.DirectionIndexStats()
		s.c.EnsureMinSize()
		afterBuilds, afterLookups := intdriver.DirectionIndexStats()

		require.Greater(t, s.c.MinSizeVisits(), len(s.labels), "the pass walked the changed rows")
		assert.Equal(t, lookups, afterLookups, "no direction lookup from inside the walk")
		assert.Equal(t, builds, afterBuilds, "no direction cache rebuild from inside the walk")
	})
}
