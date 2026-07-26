package canvas

import "math"

// OrbitCamera provides orbit controls for a Camera3D: rotate around a target
// and zoom in/out by changing the distance.
type OrbitCamera struct {
	Target   Vec3    // point to orbit around
	Distance float32 // distance from target
	Theta    float32 // horizontal angle in radians (azimuth)
	Phi      float32 // vertical angle in radians (elevation), clamped to avoid poles
	Up       Vec3
}

// NewOrbitCamera creates an OrbitCamera from a Camera3D position and target.
func NewOrbitCamera(cam Camera3D) *OrbitCamera {
	dir := cam.Position.Sub(cam.Target)
	dist := dir.Length()
	if dist == 0 {
		dist = 5
	}

	// Compute spherical angles from direction vector
	theta := float32(math.Atan2(float64(dir.X), float64(dir.Z)))
	phi := float32(math.Asin(float64(dir.Y / dist)))

	up := cam.Up
	if up.Length() == 0 {
		up = Vec3{X: 0, Y: 1, Z: 0}
	}

	return &OrbitCamera{
		Target:   cam.Target,
		Distance: dist,
		Theta:    theta,
		Phi:      phi,
		Up:       up,
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

// Zoom changes the distance from the target by a factor.
// factor > 1 zooms out, factor < 1 zooms in.
func (o *OrbitCamera) Zoom(factor float32) {
	o.Distance *= factor
	if o.Distance < 0.5 {
		o.Distance = 0.5
	}
	if o.Distance > 100 {
		o.Distance = 100
	}
}

// Camera returns the Camera3D corresponding to the current orbit state.
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
		near = 0.1
	}
	if far <= 0 {
		far = 100
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
