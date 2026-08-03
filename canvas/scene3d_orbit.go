package canvas

import "math"

// Distance limits applied by [OrbitCamera.Zoom] when the camera does not set
// its own. They suit a scene authored around the unit cube, which is what
// defaultDistance frames.
const (
	defaultDistance    = 5
	defaultMinDistance = 0.5
	defaultMaxDistance = 100
)

// OrbitCamera provides orbit controls for a Camera3D: rotate around a target
// and zoom in/out by changing the distance.
type OrbitCamera struct {
	Target   Vec3    // point to orbit around
	Distance float32 // distance from target
	Theta    float32 // horizontal angle in radians (azimuth)
	Phi      float32 // vertical angle in radians (elevation), clamped to avoid poles
	Up       Vec3

	// MinDistance and MaxDistance bound [OrbitCamera.Zoom]. Either left at or
	// below zero falls back to its default — 0.5 and 100 respectively — so a
	// zero-value OrbitCamera zooms exactly as it always has. Set them for a
	// scene authored at a different scale, where the defaults would snap the
	// camera to a limit on the first zoom.
	MinDistance float32
	MaxDistance float32
}

// NewOrbitCamera creates an OrbitCamera from a Camera3D position and target.
//
// A camera positioned outside the default zoom range gets limits scaled to its
// own distance, so orbiting a large or a miniature scene does not yank the
// camera to a limit the moment the user zooms.
func NewOrbitCamera(cam Camera3D) *OrbitCamera {
	dir := cam.Position.Sub(cam.Target)
	dist := dir.Length()
	if dist == 0 {
		dist = defaultDistance
	}

	// Keep the authored camera at the same place in its range that a
	// default-distance camera occupies in the default range.
	var minDist, maxDist float32
	if dist < defaultMinDistance || dist > defaultMaxDistance {
		scale := dist / defaultDistance
		minDist = defaultMinDistance * scale
		maxDist = defaultMaxDistance * scale
	}

	// Compute spherical angles from direction vector
	theta := float32(math.Atan2(float64(dir.X), float64(dir.Z)))
	phi := float32(math.Asin(float64(dir.Y / dist)))

	up := cam.Up
	if up.Length() == 0 {
		up = Vec3{X: 0, Y: 1, Z: 0}
	}

	return &OrbitCamera{
		Target:      cam.Target,
		Distance:    dist,
		Theta:       theta,
		Phi:         phi,
		Up:          up,
		MinDistance: minDist,
		MaxDistance: maxDist,
	}
}

// Rotate adjusts the orbit angles by the given deltas (in radians).
func (o *OrbitCamera) Rotate(dTheta, dPhi float32) {
	o.Theta += dTheta
	o.Phi += dPhi

	// Clamp phi to avoid gimbal lock at poles
	maxPhi := float32(math.Pi/2 - 0.01)
	if o.Phi > maxPhi {
		o.Phi = maxPhi
	}
	if o.Phi < -maxPhi {
		o.Phi = -maxPhi
	}
}

// Zoom changes the distance from the target by a factor, bounded by
// [OrbitCamera.MinDistance] and [OrbitCamera.MaxDistance].
// factor > 1 zooms out, factor < 1 zooms in.
func (o *OrbitCamera) Zoom(factor float32) {
	minDist, maxDist := o.limits()
	o.Distance *= factor
	if o.Distance < minDist {
		o.Distance = minDist
	}
	if o.Distance > maxDist {
		o.Distance = maxDist
	}
}

// limits resolves the zoom bounds, substituting the defaults for unset ones. A
// max below the min would make Zoom's two clamps order-dependent, so the max
// gives way.
func (o *OrbitCamera) limits() (minDist, maxDist float32) {
	minDist, maxDist = o.MinDistance, o.MaxDistance
	if minDist <= 0 {
		minDist = defaultMinDistance
	}
	if maxDist <= 0 {
		maxDist = defaultMaxDistance
	}
	if maxDist < minDist {
		maxDist = minDist
	}
	return minDist, maxDist
}

// Camera returns the Camera3D corresponding to the current orbit state.
//
// Non-positive fov, near and far resolve to defaults; the near and far
// defaults both scale with the orbit distance. Callers that feed a resolved
// Near or Far back into the next call freeze that default at its first value —
// keep passing zero (or an explicit choice) to let it track the zoom.
func (o *OrbitCamera) Camera(fov, near, far float32) Camera3D {
	// Convert spherical to cartesian
	cosP := float32(math.Cos(float64(o.Phi)))
	sinP := float32(math.Sin(float64(o.Phi)))
	cosT := float32(math.Cos(float64(o.Theta)))
	sinT := float32(math.Sin(float64(o.Theta)))

	pos := Vec3{
		X: o.Target.X + o.Distance*cosP*sinT,
		Y: o.Target.Y + o.Distance*sinP,
		Z: o.Target.Z + o.Distance*cosP*cosT,
	}

	if fov <= 0 {
		fov = 45
	}
	if near <= 0 {
		// Depth precision is spent on the far/near ratio, so a fixed near
		// would let the ratio grow linearly with scene scale once the far
		// default below starts tracking the distance. Scaling both keeps the
		// ratio bounded (~2,000) at any scale, while a camera inside the
		// default zoom range keeps the historic 0.1 exactly.
		near = 0.1
		if scaled := o.Distance / 1000; scaled > near {
			near = scaled
		}
	}
	if far <= 0 {
		// A fixed default far plane would sit in front of a camera orbiting a
		// large scene, clipping away everything it is meant to frame.
		far = defaultMaxDistance
		if reach := o.Distance * 2; reach > far {
			far = reach
		}
	}

	return Camera3D{
		Position: pos,
		Target:   o.Target,
		Up:       o.Up,
		FOV:      fov,
		Near:     near,
		Far:      far,
	}
}
