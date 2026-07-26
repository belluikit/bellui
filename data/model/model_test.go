package model

import (
	"sync"
	"testing"

	"github.com/belluikit/bellui/data/binding"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

// countingListener records how many times it was notified.
type countingListener struct {
	mu sync.Mutex
	n  int
}

func (c *countingListener) DataChanged() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
}

func (c *countingListener) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func collect[T any](m Model[T]) []T {
	out := make([]T, m.Len())
	for i := range out {
		out[i] = m.Item(i)
	}
	return out
}

func TestSliceModel_Basics(t *testing.T) {
	m := NewSliceModel("b", "a", "c")
	assert.Equal(t, 3, m.Len())
	assert.Equal(t, []string{"b", "a", "c"}, collect[string](m))

	m.Append("d")
	assert.Equal(t, []string{"b", "a", "c", "d"}, collect[string](m))

	m.SetItem(0, "z")
	assert.Equal(t, "z", m.Item(0))

	m.Set([]string{"one", "two"})
	assert.Equal(t, []string{"one", "two"}, collect[string](m))
}

func TestSliceModel_NotifiesAndFiresOnAdd(t *testing.T) {
	m := NewSliceModel(1, 2, 3)
	l := &countingListener{}

	m.AddListener(l)
	assert.Equal(t, 1, l.count(), "AddListener fires immediately")

	m.Append(4)
	m.SetItem(0, 9)
	m.Set([]int{5})
	assert.Equal(t, 4, l.count(), "one notification per mutation")

	m.RemoveListener(l)
	m.Append(6)
	assert.Equal(t, 4, l.count(), "no notification after removal")
}

func TestSliceModel_SetIsDefensiveCopy(t *testing.T) {
	src := []int{1, 2, 3}
	m := NewSliceModel[int]()
	m.Set(src)
	src[0] = 99
	assert.Equal(t, 1, m.Item(0), "model must not alias the caller's slice")
}

func TestFromDataList_ReadsAndForwards(t *testing.T) {
	test.NewTempApp(t) // binding list notifications marshal through bellui.Do
	bl := binding.NewStringList()
	assert.NoError(t, bl.Append("x"))
	assert.NoError(t, bl.Append("y"))

	m := FromDataList(bl)
	assert.Equal(t, 2, m.Len())

	l := &countingListener{}
	m.AddListener(l)
	before := l.count()

	assert.NoError(t, bl.Append("z"))
	assert.Equal(t, 3, m.Len())
	assert.Greater(t, l.count(), before, "source change forwarded to model listeners")

	item := m.Item(2)
	s, err := item.(binding.String).Get()
	assert.NoError(t, err)
	assert.Equal(t, "z", s)
}
