package async

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/internal/thread"
)

var mainRoutineID uint64

func init() {
	mainRoutineID = thread.GoroutineID()
}

func TestGoroutineID(t *testing.T) {
	assert.Equal(t, uint64(1), mainRoutineID)

	var childID1, childID2 uint64
	testID1 := thread.GoroutineID()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		childID1 = thread.GoroutineID()
		wg.Done()
	}()
	go func() {
		childID2 = thread.GoroutineID()
		wg.Done()
	}()
	wg.Wait()
	testID2 := thread.GoroutineID()

	assert.Equal(t, testID1, testID2)
	assert.Greater(t, childID1, uint64(0))
	assert.NotEqual(t, testID1, childID1)
	assert.Greater(t, childID2, uint64(0))
	assert.NotEqual(t, childID1, childID2)
}
