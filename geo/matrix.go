package geo

import "math"

// Matrix represents a 2D affine transformation matrix.
//
// The matrix is stored as 6 float32 values in row-major order:
//
//	| A  C  TX |
//	| B  D  TY |
//	| 0  0  1  |
//
// The zero value is NOT identity — use [Identity] to create an identity matrix.
// However, [canvas.baseObject.Transform] treats the zero value as identity for
// ergonomics (objects with no transform set behave as untransformed).
type Matrix struct {
	A, B, C, D, TX, TY float32
}

// Identity returns the identity matrix (no transformation).
func Identity() Matrix {
	return Matrix{A: 1, D: 1}
}

// NewTranslation returns a matrix that translates by (tx, ty).
func NewTranslation(tx, ty float32) Matrix {
	return Matrix{A: 1, D: 1, TX: tx, TY: ty}
}

// NewRotation returns a matrix that rotates counter-clockwise by the given
// angle in radians around the origin.
func NewRotation(radians float32) Matrix {
	sin, cos := float32(math.Sin(float64(radians))), float32(math.Cos(float64(radians)))
	return Matrix{A: cos, B: sin, C: -sin, D: cos}
}

// NewRotationAround returns a matrix that rotates counter-clockwise by the
// given angle in radians around the point (px, py).
func NewRotationAround(radians, px, py float32) Matrix {
	sin, cos := float32(math.Sin(float64(radians))), float32(math.Cos(float64(radians)))
	return Matrix{
		A:  cos,
		B:  sin,
		C:  -sin,
		D:  cos,
		TX: px - cos*px + sin*py,
		TY: py - sin*px - cos*py,
	}
}

// NewScale returns a matrix that scales by (sx, sy).
func NewScale(sx, sy float32) Matrix {
	return Matrix{A: sx, D: sy}
}

// NewSkew returns a matrix that skews by (kx, ky) radians.
func NewSkew(kx, ky float32) Matrix {
	return Matrix{
		A: 1,
		B: float32(math.Tan(float64(ky))),
		C: float32(math.Tan(float64(kx))),
		D: 1,
	}
}

// IsIdentity reports whether m is the identity matrix.
func (m Matrix) IsIdentity() bool {
	return m.A == 1 && m.B == 0 && m.C == 0 && m.D == 1 && m.TX == 0 && m.TY == 0
}

// IsZeroOrIdentity reports whether m is the zero value or the identity matrix.
// This is useful for checking if a transform has been set on an object where
// the zero value of Matrix is used as "no transform".
func (m Matrix) IsZeroOrIdentity() bool {
	return m == (Matrix{}) || m.IsIdentity()
}

// Multiply returns the matrix product m * n.
// This applies n first, then m. For example, to translate then rotate:
//
//	rotate.Multiply(translate)
func (m Matrix) Multiply(n Matrix) Matrix {
	return Matrix{
		A:  m.A*n.A + m.C*n.B,
		B:  m.B*n.A + m.D*n.B,
		C:  m.A*n.C + m.C*n.D,
		D:  m.B*n.C + m.D*n.D,
		TX: m.A*n.TX + m.C*n.TY + m.TX,
		TY: m.B*n.TX + m.D*n.TY + m.TY,
	}
}

// TransformPoint applies the full affine transform (including translation)
// to the point (x, y) and returns the transformed coordinates.
func (m Matrix) TransformPoint(x, y float32) (float32, float32) {
	return m.A*x + m.C*y + m.TX, m.B*x + m.D*y + m.TY
}

// TransformVector applies only the linear part of the transform (no translation)
// to the vector (x, y). Useful for transforming directions and sizes.
func (m Matrix) TransformVector(x, y float32) (float32, float32) {
	return m.A*x + m.C*y, m.B*x + m.D*y
}

// Inverse returns the inverse of m and true, or the zero matrix and false
// if m is not invertible (determinant is zero).
func (m Matrix) Inverse() (Matrix, bool) {
	det := m.A*m.D - m.B*m.C
	if det == 0 {
		return Matrix{}, false
	}
	invDet := 1.0 / det
	return Matrix{
		A:  m.D * invDet,
		B:  -m.B * invDet,
		C:  -m.C * invDet,
		D:  m.A * invDet,
		TX: (m.C*m.TY - m.D*m.TX) * invDet,
		TY: (m.B*m.TX - m.A*m.TY) * invDet,
	}, true
}

// Determinant returns the determinant of the linear part of the matrix.
func (m Matrix) Determinant() float32 {
	return m.A*m.D - m.B*m.C
}

// ToGL returns the matrix as a 9-element column-major array suitable for
// uploading to an OpenGL mat3 uniform via glUniformMatrix3fv.
//
// Column-major layout:
//
//	[A, B, 0, C, D, 0, TX, TY, 1]
func (m Matrix) ToGL() [9]float32 {
	return [9]float32{
		m.A, m.B, 0,
		m.C, m.D, 0,
		m.TX, m.TY, 1,
	}
}

// AABB computes the axis-aligned bounding box of the rectangle (0,0)-(w,h)
// after applying this transform. Returns (minX, minY, maxX, maxY).
func (m Matrix) AABB(w, h float32) (minX, minY, maxX, maxY float32) {
	// Transform all 4 corners.
	x0, y0 := m.TransformPoint(0, 0)
	x1, y1 := m.TransformPoint(w, 0)
	x2, y2 := m.TransformPoint(0, h)
	x3, y3 := m.TransformPoint(w, h)

	minX = min4(x0, x1, x2, x3)
	minY = min4(y0, y1, y2, y3)
	maxX = max4(x0, x1, x2, x3)
	maxY = max4(y0, y1, y2, y3)
	return
}

func min4(a, b, c, d float32) float32 {
	return Min(Min(a, b), Min(c, d))
}

func max4(a, b, c, d float32) float32 {
	return Max(Max(a, b), Max(c, d))
}
