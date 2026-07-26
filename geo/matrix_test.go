package geo

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

const epsilon = 1e-5

func assertMatrixNear(t *testing.T, expected, got Matrix) {
	t.Helper()
	assert.InDelta(t, expected.A, got.A, epsilon, "A")
	assert.InDelta(t, expected.B, got.B, epsilon, "B")
	assert.InDelta(t, expected.C, got.C, epsilon, "C")
	assert.InDelta(t, expected.D, got.D, epsilon, "D")
	assert.InDelta(t, expected.TX, got.TX, epsilon, "TX")
	assert.InDelta(t, expected.TY, got.TY, epsilon, "TY")
}

func assertPointNear(t *testing.T, ex, ey, gx, gy float32) {
	t.Helper()
	assert.InDelta(t, ex, gx, epsilon, "X")
	assert.InDelta(t, ey, gy, epsilon, "Y")
}

func TestIdentity(t *testing.T) {
	m := Identity()
	assert.True(t, m.IsIdentity())
	assert.Equal(t, float32(1), m.A)
	assert.Equal(t, float32(0), m.B)
	assert.Equal(t, float32(0), m.C)
	assert.Equal(t, float32(1), m.D)
	assert.Equal(t, float32(0), m.TX)
	assert.Equal(t, float32(0), m.TY)
}

func TestIsIdentity(t *testing.T) {
	assert.True(t, Identity().IsIdentity())
	assert.False(t, NewTranslation(1, 0).IsIdentity())
	assert.False(t, NewScale(2, 2).IsIdentity())
	assert.False(t, (Matrix{}).IsIdentity(), "zero value is NOT identity")
}

func TestIsZeroOrIdentity(t *testing.T) {
	assert.True(t, (Matrix{}).IsZeroOrIdentity())
	assert.True(t, Identity().IsZeroOrIdentity())
	assert.False(t, NewTranslation(1, 0).IsZeroOrIdentity())
}

func TestTranslation(t *testing.T) {
	m := NewTranslation(10, 20)
	x, y := m.TransformPoint(5, 5)
	assertPointNear(t, 15, 25, x, y)
}

func TestRotation90(t *testing.T) {
	m := NewRotation(math.Pi / 2)
	// (1, 0) rotated 90° CCW → (0, 1)
	x, y := m.TransformPoint(1, 0)
	assertPointNear(t, 0, 1, x, y)

	// (0, 1) rotated 90° CCW → (-1, 0)
	x, y = m.TransformPoint(0, 1)
	assertPointNear(t, -1, 0, x, y)
}

func TestRotation45(t *testing.T) {
	m := NewRotation(math.Pi / 4)
	s := float32(math.Sqrt2 / 2)
	x, y := m.TransformPoint(1, 0)
	assertPointNear(t, s, s, x, y)
}

func TestRotationAround(t *testing.T) {
	// Rotate (10, 0) by 90° around (5, 0) → should be (5, 5)
	m := NewRotationAround(math.Pi/2, 5, 0)
	x, y := m.TransformPoint(10, 0)
	assertPointNear(t, 5, 5, x, y)

	// The pivot itself should be unchanged.
	x, y = m.TransformPoint(5, 0)
	assertPointNear(t, 5, 0, x, y)
}

func TestScale(t *testing.T) {
	m := NewScale(2, 3)
	x, y := m.TransformPoint(5, 10)
	assertPointNear(t, 10, 30, x, y)
}

func TestSkew(t *testing.T) {
	m := NewSkew(math.Pi/4, 0) // 45° X-skew
	x, y := m.TransformPoint(0, 1)
	assertPointNear(t, 1, 1, x, y) // y shifts x by tan(45°)=1
}

func TestMultiplyIdentity(t *testing.T) {
	m := NewTranslation(10, 20)
	assertMatrixNear(t, m, m.Multiply(Identity()))
	assertMatrixNear(t, m, Identity().Multiply(m))
}

func TestMultiplyTranslateRotate(t *testing.T) {
	// Translate (10,0) then rotate 90° CCW.
	// Point (0,0) → translate → (10,0) → rotate → (0, 10)
	rot := NewRotation(math.Pi / 2)
	tr := NewTranslation(10, 0)
	m := rot.Multiply(tr) // apply tr first, then rot
	x, y := m.TransformPoint(0, 0)
	assertPointNear(t, 0, 10, x, y)
}

func TestMultiplyAssociative(t *testing.T) {
	a := NewRotation(0.5)
	b := NewTranslation(3, 7)
	c := NewScale(2, 0.5)

	abThenC := a.Multiply(b).Multiply(c)
	aThenBC := a.Multiply(b.Multiply(c))
	assertMatrixNear(t, abThenC, aThenBC)
}

func TestTransformVector(t *testing.T) {
	m := NewTranslation(100, 200)
	// TransformVector ignores translation.
	x, y := m.TransformVector(5, 10)
	assertPointNear(t, 5, 10, x, y)

	m2 := NewScale(2, 3)
	x, y = m2.TransformVector(5, 10)
	assertPointNear(t, 10, 30, x, y)
}

func TestInverseIdentity(t *testing.T) {
	inv, ok := Identity().Inverse()
	assert.True(t, ok)
	assert.True(t, inv.IsIdentity())
}

func TestInverseTranslation(t *testing.T) {
	m := NewTranslation(10, 20)
	inv, ok := m.Inverse()
	assert.True(t, ok)
	product := m.Multiply(inv)
	assertMatrixNear(t, Identity(), product)
}

func TestInverseRotation(t *testing.T) {
	m := NewRotation(1.23)
	inv, ok := m.Inverse()
	assert.True(t, ok)
	product := m.Multiply(inv)
	assertMatrixNear(t, Identity(), product)
}

func TestInverseScale(t *testing.T) {
	m := NewScale(2, 3)
	inv, ok := m.Inverse()
	assert.True(t, ok)
	assertMatrixNear(t, Identity(), m.Multiply(inv))
}

func TestInverseComposed(t *testing.T) {
	m := NewRotation(0.7).Multiply(NewTranslation(10, 20)).Multiply(NewScale(2, 3))
	inv, ok := m.Inverse()
	assert.True(t, ok)
	assertMatrixNear(t, Identity(), m.Multiply(inv))
}

func TestInverseDegenerate(t *testing.T) {
	m := NewScale(0, 0)
	_, ok := m.Inverse()
	assert.False(t, ok)
}

func TestDeterminant(t *testing.T) {
	assert.InDelta(t, 1.0, Identity().Determinant(), epsilon)
	assert.InDelta(t, 6.0, NewScale(2, 3).Determinant(), epsilon)
	assert.InDelta(t, 1.0, NewRotation(1.0).Determinant(), epsilon)
	assert.InDelta(t, 0.0, NewScale(0, 0).Determinant(), epsilon)
}

func TestToGL(t *testing.T) {
	m := Matrix{A: 1, B: 2, C: 3, D: 4, TX: 5, TY: 6}
	gl := m.ToGL()
	// Column-major: col0=(A,B,0), col1=(C,D,0), col2=(TX,TY,1)
	assert.Equal(t, [9]float32{1, 2, 0, 3, 4, 0, 5, 6, 1}, gl)
}

func TestToGLIdentity(t *testing.T) {
	gl := Identity().ToGL()
	assert.Equal(t, [9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1}, gl)
}

func TestAABB_NoTransform(t *testing.T) {
	m := NewTranslation(10, 20)
	minX, minY, maxX, maxY := m.AABB(100, 50)
	assert.InDelta(t, 10, minX, epsilon)
	assert.InDelta(t, 20, minY, epsilon)
	assert.InDelta(t, 110, maxX, epsilon)
	assert.InDelta(t, 70, maxY, epsilon)
}

func TestAABB_Rotation45(t *testing.T) {
	// 100x100 square rotated 45° around origin.
	// Diagonal = 100*sqrt(2) ≈ 141.42
	m := NewRotation(math.Pi / 4)
	minX, minY, maxX, maxY := m.AABB(100, 100)

	diag := float32(100 * math.Sqrt2)
	assert.InDelta(t, -100*math.Sqrt2/2, minX, 0.01)
	assert.InDelta(t, 0, minY, 0.01)
	assert.InDelta(t, 100*math.Sqrt2/2, maxX, 0.01)
	assert.InDelta(t, float64(diag), maxY, 0.01)
}

func TestAABB_Rotation90(t *testing.T) {
	// 100x50 rect rotated 90° → 50x100
	m := NewRotation(math.Pi / 2)
	minX, minY, maxX, maxY := m.AABB(100, 50)
	assert.InDelta(t, -50, minX, 0.01)
	assert.InDelta(t, 0, minY, 0.01)
	assert.InDelta(t, 0, maxX, 0.01)
	assert.InDelta(t, 100, maxY, 0.01)
}
