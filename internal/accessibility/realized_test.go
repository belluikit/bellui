//go:build accessibility

package accessibility

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverflowRealized(t *testing.T) {
	// Under the cap: keep everything, evict nothing.
	keep, evict := OverflowRealized([]any{"a", "b"}, 3)
	assert.Equal(t, []any{"a", "b"}, keep)
	assert.Nil(t, evict)

	// At the cap: still nothing to evict.
	keep, evict = OverflowRealized([]any{"a", "b", "c"}, 3)
	assert.Equal(t, []any{"a", "b", "c"}, keep)
	assert.Nil(t, evict)

	// Over the cap: the oldest (front) are evicted, newest kept, FIFO order.
	keep, evict = OverflowRealized([]any{"a", "b", "c", "d", "e"}, 3)
	assert.Equal(t, []any{"c", "d", "e"}, keep)
	assert.Equal(t, []any{"a", "b"}, evict)
}

func TestDropRealizedID(t *testing.T) {
	order := []any{"a", "b", "c"}
	assert.Equal(t, []any{"a", "c"}, DropRealizedID(order, "b"))
	assert.Equal(t, []any{"a", "b", "c"}, DropRealizedID([]any{"a", "b", "c"}, "missing"))
	assert.Empty(t, DropRealizedID([]any{"x"}, "x"))
}
