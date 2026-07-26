package canvas

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
)

func TestKeyframeAnimation_TwoKeyframes(t *testing.T) {
	kf := []Keyframe{
		{Progress: 0, Value: 0},
		{Progress: 1, Value: 100},
	}

	var current float32
	anim := NewKeyframeAnimation(kf, time.Second, func(v float32) { current = v })

	anim.Tick(0.0)
	assert.Equal(t, float32(0), current)
	anim.Tick(0.5)
	assert.Equal(t, float32(50), current)
	anim.Tick(1.0)
	assert.Equal(t, float32(100), current)
}

func TestKeyframeAnimation_ThreeKeyframes(t *testing.T) {
	kf := []Keyframe{
		{Progress: 0, Value: 0},
		{Progress: 0.3, Value: 100},
		{Progress: 1.0, Value: 50},
	}

	var current float32
	anim := NewKeyframeAnimation(kf, time.Second, func(v float32) { current = v })

	// At start
	anim.Tick(0.0)
	assert.Equal(t, float32(0), current)

	// At first keyframe boundary
	anim.Tick(0.3)
	assert.InDelta(t, 100, current, 0.1)

	// Midway between second and third keyframe: 0.3 + (1.0-0.3)/2 = 0.65
	anim.Tick(0.65)
	assert.InDelta(t, 75, current, 0.1)

	// At end
	anim.Tick(1.0)
	assert.Equal(t, float32(50), current)
}

func TestKeyframeAnimation_PerSegmentCurve(t *testing.T) {
	kf := []Keyframe{
		{Progress: 0, Value: 0},
		{Progress: 0.5, Value: 100, Curve: bellui.AnimationLinear},
		{Progress: 1.0, Value: 100, Curve: bellui.AnimationEaseIn}, // hold at 100
	}

	var current float32
	anim := NewKeyframeAnimation(kf, time.Second, func(v float32) { current = v })

	// Linear segment: 25% overall = halfway through first segment
	anim.Tick(0.25)
	assert.InDelta(t, 50, current, 0.1)

	// Second segment holds value at 100
	anim.Tick(0.75)
	assert.InDelta(t, 100, current, 0.1)
}

func TestPositionKeyframeAnimation(t *testing.T) {
	kf := []PositionKeyframe{
		{Progress: 0, Value: geo.NewPos(0, 0)},
		{Progress: 0.5, Value: geo.NewPos(100, 0)},
		{Progress: 1.0, Value: geo.NewPos(100, 100)},
	}

	var current geo.Position
	anim := NewPositionKeyframeAnimation(kf, time.Second, func(p geo.Position) { current = p })

	anim.Tick(0.0)
	assert.Equal(t, geo.NewPos(0, 0), current)

	anim.Tick(0.25)
	assert.Equal(t, geo.NewPos(50, 0), current)

	anim.Tick(0.5)
	assert.Equal(t, geo.NewPos(100, 0), current)

	anim.Tick(0.75)
	assert.Equal(t, geo.NewPos(100, 50), current)

	anim.Tick(1.0)
	assert.Equal(t, geo.NewPos(100, 100), current)
}

func TestKeyframeAnimation_EpsilonEndpoints(t *testing.T) {
	// Float-computed endpoints that land just shy of 0.0/1.0 must not panic the
	// public constructor (#289).
	kf := []Keyframe{
		{Progress: 0.0000001, Value: 0},
		{Progress: 0.9999998, Value: 100},
	}
	assert.NotPanics(t, func() {
		NewKeyframeAnimation(kf, time.Second, func(float32) {})
	})

	pkf := []PositionKeyframe{
		{Progress: 0.0000001, Value: geo.NewPos(0, 0)},
		{Progress: 0.9999998, Value: geo.NewPos(10, 10)},
	}
	assert.NotPanics(t, func() {
		NewPositionKeyframeAnimation(pkf, time.Second, func(geo.Position) {})
	})
}

func TestKeyframeAnimation_PanicsOnTooFew(t *testing.T) {
	assert.Panics(t, func() {
		NewKeyframeAnimation([]Keyframe{{Progress: 0, Value: 0}}, time.Second, func(float32) {})
	})
}

func TestKeyframeAnimation_PanicsOnBadStart(t *testing.T) {
	assert.Panics(t, func() {
		NewKeyframeAnimation([]Keyframe{
			{Progress: 0.5, Value: 0},
			{Progress: 1.0, Value: 100},
		}, time.Second, func(float32) {})
	})
}

func TestKeyframeAnimation_PanicsOnBadEnd(t *testing.T) {
	assert.Panics(t, func() {
		NewKeyframeAnimation([]Keyframe{
			{Progress: 0, Value: 0},
			{Progress: 0.5, Value: 100},
		}, time.Second, func(float32) {})
	})
}

func TestKeyframeAnimation_PanicsOnUnsorted(t *testing.T) {
	assert.Panics(t, func() {
		NewKeyframeAnimation([]Keyframe{
			{Progress: 0, Value: 0},
			{Progress: 0.8, Value: 50},
			{Progress: 0.3, Value: 100},
			{Progress: 1.0, Value: 0},
		}, time.Second, func(float32) {})
	})
}
