//go:build windows && !race

package webview

// Locks the shape #82's first live run caught: comObject.call must reach
// methods through the object's vtable POINTER (word 0), never by reading the
// object's own words as function addresses. The fake object below keeps
// sentinel values where its fields are, so the collapsed single-hop read that
// shipped in #81 would jump to a sentinel instead of the callback — a crash,
// not a wrong answer, which is exactly how the defect presented against the
// real engine.
//
// !race: building a comObject from a Go allocation and reading it back through
// raw addresses is the uintptr round trip checkptr refuses under -race (see
// comHandlerQueryInterface's comment); the production caller's addresses are
// the engine's, outside the Go heap.

import (
	"syscall"
	"testing"
	"unsafe"
)

func TestCOMObjectCallGoesThroughTheVtable(t *testing.T) {
	type fakeObject struct {
		vtbl *[4]uintptr
		// Words a single-hop read would misinterpret as vtable slots.
		sentinelA, sentinelB uintptr
	}

	var gotThis, gotArg uintptr
	calls := 0
	cb := syscall.NewCallback(func(this, arg uintptr) uintptr {
		gotThis, gotArg = this, arg
		calls++
		return 7 // an arbitrary, recognisable HRESULT
	})

	vtbl := [4]uintptr{}
	vtbl[2] = cb
	obj := &fakeObject{vtbl: &vtbl, sentinelA: 0xDEAD, sentinelB: 0xBEEF}

	o := comObject(uintptr(unsafe.Pointer(obj)))
	hr := o.call(2, 42)

	if calls != 1 {
		t.Fatalf("the vtable slot was not invoked (calls=%d)", calls)
	}
	if gotThis != uintptr(unsafe.Pointer(obj)) {
		t.Errorf("this = %#x, want the object %#x", gotThis, unsafe.Pointer(obj))
	}
	if gotArg != 42 {
		t.Errorf("arg = %d, want 42", gotArg)
	}
	if hr != 7 {
		t.Errorf("hr = %d, want the callback's 7", hr)
	}
}
