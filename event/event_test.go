package event_test

import (
	"testing"

	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// Verify that event package types can be constructed and assigned.
func TestTypeConstruction(t *testing.T) {
	var pe event.PointEvent
	pe.Position = geo.NewPos(10, 20)
	_ = pe

	var ke event.KeyEvent
	ke.Name = event.KeyReturn
	_ = ke
}

// Verify interface compatibility.
type testWidget struct{}

func (t *testWidget) Tapped(_ *event.PointEvent) {}

func TestInterfaceIdentity(t *testing.T) {
	var tap event.Tappable = &testWidget{}
	var ftap event.Tappable = tap // must compile — same interface
	_ = ftap
}
