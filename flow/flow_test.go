package flow

import (
	"sync"
	"testing"
)

func TestValue_GetSet(t *testing.T) {
	v := New("hello")
	if v.Get() != "hello" {
		t.Fatalf("expected hello, got %q", v.Get())
	}
	v.Set("world")
	if v.Get() != "world" {
		t.Fatalf("expected world, got %q", v.Get())
	}
}

func TestValue_Listen(t *testing.T) {
	v := New(0)
	var calls int
	v.Listen(func(_, _ int) { calls++ })

	v.Set(1)
	v.Set(2)
	v.Set(2) // same value, no notification
	v.Set(3)

	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestValue_Unsubscribe(t *testing.T) {
	v := New("")
	var calls int
	cancel := v.Listen(func(_, _ string) { calls++ })

	v.Set("a")
	cancel()
	v.Set("b")

	if calls != 1 {
		t.Fatalf("expected 1 call after unsubscribe, got %d", calls)
	}
}

func TestValue_MultipleListeners(t *testing.T) {
	v := New(0)
	var a, b int
	v.Listen(func(_, _ int) { a++ })
	v.Listen(func(_, _ int) { b++ })

	v.Set(1)
	if a != 1 || b != 1 {
		t.Fatalf("expected both listeners called, got a=%d b=%d", a, b)
	}
}

func TestValue_Modify(t *testing.T) {
	v := New(10)
	v.Modify(func(x int) int { return x + 5 })
	if v.Get() != 15 {
		t.Fatalf("expected 15, got %d", v.Get())
	}
}

func TestValue_ConcurrentAccess(t *testing.T) {
	v := New(0)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			v.Set(n)
			_ = v.Get()
		}(i)
	}
	wg.Wait()
}

func TestDerived_Basic(t *testing.T) {
	first := New("John")
	last := New("Doe")

	full := Derive(func() string {
		return first.Get() + " " + last.Get()
	}, first, last)

	if full.Get() != "John Doe" {
		t.Fatalf("expected 'John Doe', got %q", full.Get())
	}

	first.Set("Jane")
	if full.Get() != "Jane Doe" {
		t.Fatalf("expected 'Jane Doe', got %q", full.Get())
	}

	last.Set("Smith")
	if full.Get() != "Jane Smith" {
		t.Fatalf("expected 'Jane Smith', got %q", full.Get())
	}
}

func TestDerived_Listen(t *testing.T) {
	count := New(0)
	doubled := Derive(func() int {
		return count.Get() * 2
	}, count)

	var received int
	doubled.Listen(func(_, new int) {
		received = new
	})

	count.Set(5)
	if received != 10 {
		t.Fatalf("expected 10, got %d", received)
	}
}

func TestDerived_NoSpuriousUpdates(t *testing.T) {
	v := New(3)
	abs := Derive(func() int {
		x := v.Get()
		if x < 0 {
			return -x
		}
		return x
	}, v)

	var calls int
	abs.Listen(func(_, _ int) { calls++ })

	v.Set(3)  // same derived value — no notification
	v.Set(-3) // same derived value (abs(3)==abs(-3)) — no notification
	v.Set(5)  // different derived value — notification

	if calls != 1 {
		t.Fatalf("expected 1 call (no spurious updates), got %d", calls)
	}
}

func TestDerived_Chain(t *testing.T) {
	base := New(2)
	doubled := Derive(func() int { return base.Get() * 2 }, base)
	quadrupled := Derive(func() int { return doubled.Get() * 2 }, doubled)

	if quadrupled.Get() != 8 {
		t.Fatalf("expected 8, got %d", quadrupled.Get())
	}

	base.Set(3)
	if quadrupled.Get() != 12 {
		t.Fatalf("expected 12, got %d", quadrupled.Get())
	}
}

func TestDerived_Close(t *testing.T) {
	v := New(1)
	d := Derive(func() int { return v.Get() * 10 }, v)

	d.Close()
	v.Set(2)

	if d.Get() != 10 { // should still have old value
		t.Fatalf("expected 10 (stale after Close), got %d", d.Get())
	}
}
