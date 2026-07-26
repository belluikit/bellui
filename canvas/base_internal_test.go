package canvas

import (
	"sync"
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
)

// Verify that all canvas primitives implement Transformable.
var (
	_ core.Transformable = (*Rectangle)(nil)
	_ core.Transformable = (*Text)(nil)
	_ core.Transformable = (*Image)(nil)
	_ core.Transformable = (*Raster)(nil)
	_ core.Transformable = (*LinearGradient)(nil)
	_ core.Transformable = (*RadialGradient)(nil)
	_ core.Transformable = (*Polygon)(nil)
	_ core.Transformable = (*Arc)(nil)
	_ core.Transformable = (*Path)(nil)
	_ core.Transformable = (*Circle)(nil)
	_ core.Transformable = (*Line)(nil)
)

func TestBase_MinSize(t *testing.T) {
	base := &baseObject{}
	min := base.MinSize()

	assert.Positive(t, min.Width)
	assert.Positive(t, min.Height)
}

func TestBase_Move(t *testing.T) {
	base := &baseObject{}
	base.Move(geo.NewPos(10, 10))
	pos := base.Position()

	assert.Equal(t, float32(10), pos.X)
	assert.Equal(t, float32(10), pos.Y)
}

func TestBase_Resize(t *testing.T) {
	base := &baseObject{}
	base.Resize(geo.NewSize(10, 10))
	size := base.Size()

	assert.Equal(t, float32(10), size.Width)
	assert.Equal(t, float32(10), size.Height)
}

func TestBase_HideShow(t *testing.T) {
	base := &baseObject{}
	assert.True(t, base.Visible())

	base.Hide()
	assert.False(t, base.Visible())

	base.Show()
	assert.True(t, base.Visible())
}

func TestBase_Transform_DefaultIsIdentity(t *testing.T) {
	base := &baseObject{}
	m := base.Transform()
	assert.True(t, m.IsIdentity(), "default transform should be identity")
}

func TestBase_Transform_ZeroValueIsIdentity(t *testing.T) {
	base := &baseObject{}
	// The zero value of geo.Matrix is NOT identity, but Transform()
	// should treat it as identity for objects that never had SetTransform called.
	m := base.Transform()
	assert.Equal(t, float32(1), m.A)
	assert.Equal(t, float32(0), m.B)
	assert.Equal(t, float32(0), m.C)
	assert.Equal(t, float32(1), m.D)
	assert.Equal(t, float32(0), m.TX)
	assert.Equal(t, float32(0), m.TY)
}

func TestBase_Transform_SetAndGet(t *testing.T) {
	base := &baseObject{}
	rot := geo.NewRotation(1.5)
	base.SetTransform(rot)

	got := base.Transform()
	assert.Equal(t, rot, got)
}

func TestBase_Transform_SetIdentityClearsTransform(t *testing.T) {
	base := &baseObject{}
	base.SetTransform(geo.NewRotation(1.0))
	base.SetTransform(geo.Identity())

	m := base.Transform()
	assert.True(t, m.IsIdentity())
}

func TestBase_Transform_ConcurrentAccess(t *testing.T) {
	base := &baseObject{}
	var wg sync.WaitGroup
	const goroutines = 10
	const iterations = 100

	// Concurrent writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				base.SetTransform(geo.NewRotation(float32(j)))
			}
		}()
	}

	// Concurrent readers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = base.Transform()
			}
		}()
	}

	wg.Wait()
	// No race detector failures = pass.
}
