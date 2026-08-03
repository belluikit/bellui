package canvas

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOrbitCamera(t *testing.T) {
	cam := Camera3D{
		Position: Vec3{0, 0, 5},
		Target:   Vec3{0, 0, 0},
		Up:       Vec3{0, 1, 0},
	}
	o := NewOrbitCamera(cam)

	assert.Equal(t, cam.Target, o.Target)
	assert.InDelta(t, 5.0, o.Distance, 1e-5)
	assert.Equal(t, Vec3{0, 1, 0}, o.Up)
	// Looking straight down +Z: phi (elevation) is 0.
	assert.InDelta(t, 0.0, o.Phi, 1e-5)
}

func TestNewOrbitCamera_DegenerateDistance(t *testing.T) {
	// Position == Target gives a zero direction; distance falls back to 5.
	cam := Camera3D{Position: Vec3{1, 1, 1}, Target: Vec3{1, 1, 1}}
	o := NewOrbitCamera(cam)
	assert.InDelta(t, 5.0, o.Distance, 1e-5)
}

func TestNewOrbitCamera_DefaultsUp(t *testing.T) {
	// A zero-length Up vector defaults to +Y.
	cam := Camera3D{Position: Vec3{0, 0, 5}, Target: Vec3{0, 0, 0}, Up: Vec3{}}
	o := NewOrbitCamera(cam)
	assert.Equal(t, Vec3{0, 1, 0}, o.Up)
}

func TestOrbitCamera_Rotate_PhiClampUpper(t *testing.T) {
	o := &OrbitCamera{Distance: 5, Up: Vec3{0, 1, 0}}
	// A large positive elevation must clamp just short of the +Y pole.
	o.Rotate(0, 10)
	maxPhi := float32(math.Pi/2 - 0.01)
	assert.InDelta(t, maxPhi, o.Phi, 1e-5)
	assert.Less(t, o.Phi, float32(math.Pi/2)) // never reaches the pole (gimbal lock)
}

func TestOrbitCamera_Rotate_PhiClampLower(t *testing.T) {
	o := &OrbitCamera{Distance: 5, Up: Vec3{0, 1, 0}}
	o.Rotate(0, -10)
	minPhi := float32(-(math.Pi/2 - 0.01))
	assert.InDelta(t, minPhi, o.Phi, 1e-5)
	assert.Greater(t, o.Phi, float32(-math.Pi/2))
}

func TestOrbitCamera_Rotate_ThetaAccumulates(t *testing.T) {
	o := &OrbitCamera{Distance: 5, Up: Vec3{0, 1, 0}}
	o.Rotate(0.5, 0)
	o.Rotate(0.25, 0)
	assert.InDelta(t, 0.75, o.Theta, 1e-5) // theta is unclamped
}

func TestOrbitCamera_Zoom_ClampsDistance(t *testing.T) {
	o := &OrbitCamera{Distance: 5}

	// Zoom out beyond the far clamp.
	o.Zoom(1000)
	assert.InDelta(t, 100.0, o.Distance, 1e-5)

	// Zoom in beyond the near clamp.
	o.Distance = 5
	o.Zoom(0.0001)
	assert.InDelta(t, 0.5, o.Distance, 1e-5)

	// A moderate factor scales within bounds.
	o.Distance = 5
	o.Zoom(2)
	assert.InDelta(t, 10.0, o.Distance, 1e-5)
}

func TestOrbitCamera_Zoom_HonoursCustomLimits(t *testing.T) {
	o := &OrbitCamera{Distance: 500, MinDistance: 50, MaxDistance: 5000}

	o.Zoom(1000)
	assert.InDelta(t, 5000.0, o.Distance, 1e-5)

	o.Distance = 500
	o.Zoom(0.0001)
	assert.InDelta(t, 50.0, o.Distance, 1e-5)

	o.Distance = 500
	o.Zoom(2)
	assert.InDelta(t, 1000.0, o.Distance, 1e-5)
}

func TestOrbitCamera_Zoom_PartialLimitsFallBack(t *testing.T) {
	// Only the max set: the min keeps its default.
	o := &OrbitCamera{Distance: 500, MaxDistance: 5000}
	o.Zoom(0.0001)
	assert.InDelta(t, 0.5, o.Distance, 1e-5)

	// Only the min set: the max keeps its default, even though that puts it
	// below the min — the min wins so the two clamps cannot fight.
	o = &OrbitCamera{Distance: 500, MinDistance: 200}
	o.Zoom(1000)
	assert.InDelta(t, 200.0, o.Distance, 1e-5)
}

func TestNewOrbitCamera_ScalesLimitsToAuthoredDistance(t *testing.T) {
	// A camera far outside the default range must not snap on the first zoom.
	o := NewOrbitCamera(Camera3D{Position: Vec3{0, 0, 500}, Target: Vec3{}})
	assert.InDelta(t, 500.0, o.Distance, 1e-4)

	o.Zoom(1.5)
	assert.InDelta(t, 750.0, o.Distance, 1e-3, "zooming out from an authored camera must not clamp to 100")

	// A miniature scene gets the same treatment in the other direction.
	o = NewOrbitCamera(Camera3D{Position: Vec3{0, 0, 0.1}, Target: Vec3{}})
	o.Zoom(0.5)
	assert.InDelta(t, 0.05, o.Distance, 1e-5)
}

func TestNewOrbitCamera_DefaultRangeUnchanged(t *testing.T) {
	// A camera inside the default range keeps the historical limits, so
	// existing callers see no behaviour change.
	o := NewOrbitCamera(Camera3D{Position: Vec3{0, 0, 5}, Target: Vec3{}})
	assert.Zero(t, o.MinDistance)
	assert.Zero(t, o.MaxDistance)

	o.Zoom(1000)
	assert.InDelta(t, 100.0, o.Distance, 1e-5)
}

func TestOrbitCamera_Camera_RoundTrip(t *testing.T) {
	// Converting a Camera3D to an OrbitCamera and back must reproduce the
	// original position and target (spherical <-> cartesian round-trip).
	orig := Camera3D{
		Position: Vec3{2, 3, 4},
		Target:   Vec3{1, 1, 1},
		Up:       Vec3{0, 1, 0},
	}
	o := NewOrbitCamera(orig)
	got := o.Camera(45, 0.1, 100)

	assert.InDelta(t, orig.Position.X, got.Position.X, 1e-4)
	assert.InDelta(t, orig.Position.Y, got.Position.Y, 1e-4)
	assert.InDelta(t, orig.Position.Z, got.Position.Z, 1e-4)
	assert.Equal(t, orig.Target, got.Target)
	assert.Equal(t, float32(45), got.FOV)
	assert.InDelta(t, 0.1, got.Near, 1e-5)
	assert.Equal(t, float32(100), got.Far)
}

func TestOrbitCamera_Camera_Defaults(t *testing.T) {
	o := &OrbitCamera{Distance: 5, Up: Vec3{0, 1, 0}}
	// Non-positive fov/near/far fall back to sensible defaults.
	c := o.Camera(0, 0, 0)
	assert.Equal(t, float32(45), c.FOV)
	assert.InDelta(t, 0.1, c.Near, 1e-5)
	assert.Equal(t, float32(100), c.Far)
}

func TestOrbitCamera_Camera_DefaultFarReachesCamera(t *testing.T) {
	// The default far plane must clear a camera orbiting a large scene,
	// otherwise the scene it frames is entirely behind it.
	o := &OrbitCamera{Distance: 500, Up: Vec3{0, 1, 0}}
	c := o.Camera(0, 0, 0)
	assert.Greater(t, c.Far, o.Distance)

	// An explicit far is still honoured, however small.
	c = o.Camera(0, 0, 10)
	assert.Equal(t, float32(10), c.Far)
}

func TestOrbitCamera_Camera_DefaultNearScalesWithDistance(t *testing.T) {
	// Depth precision is spent on the far/near ratio, so the default near
	// must scale with the orbit distance like the default far does — a fixed
	// 0.1 would let the ratio grow linearly with scene scale.
	o := &OrbitCamera{Distance: 500, Up: Vec3{0, 1, 0}}
	c := o.Camera(0, 0, 0)
	assert.InDelta(t, 0.5, c.Near, 1e-5)
	assert.LessOrEqual(t, c.Far/c.Near, float32(2000))

	// An explicit near is still honoured, however small.
	c = o.Camera(0, 0.001, 0)
	assert.InDelta(t, 0.001, c.Near, 1e-7)
}

func TestOrbitCamera_Camera_DefaultNearUnchangedInDefaultRange(t *testing.T) {
	// A camera anywhere inside the default zoom range must resolve to the
	// historic 0.1 exactly, not merely approximately.
	for _, dist := range []float32{defaultMinDistance, defaultDistance, defaultMaxDistance} {
		o := &OrbitCamera{Distance: dist, Up: Vec3{0, 1, 0}}
		assert.Equal(t, float32(0.1), o.Camera(0, 0, 0).Near, "distance %v", dist)
	}
}
