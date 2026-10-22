package repository

import (
	"sync"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/stretchr/testify/assert"
)

type stubRepository struct{}

func (stubRepository) Exists(bellui.URI) (bool, error)                 { return false, nil }
func (stubRepository) Reader(bellui.URI) (bellui.URIReadCloser, error) { return nil, nil }
func (stubRepository) CanRead(bellui.URI) (bool, error)                { return false, nil }
func (stubRepository) Destroy(string)                                  {}

// A lookup from a background goroutine (a directory listing) can overlap a new
// app registering its repositories, as test apps do on every test; -race
// catches an unguarded table (#639).
func TestRegisterConcurrentWithLookup(t *testing.T) {
	const scheme = "registertest"
	Register(scheme, stubRepository{})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			Register(scheme, stubRepository{})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_, err := ForScheme(scheme)
			assert.NoError(t, err)
		}
	}()
	wg.Wait()
}
