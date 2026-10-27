package mobileinit

import (
	"sync"
	"testing"
)

// TestCtxRefsKeepsAHeldReference pins #852: a reference replaced while a
// callback holds it is deleted by that callback's release, not by the
// replacement.
func TestCtxRefsKeepsAHeldReference(t *testing.T) {
	var r ctxRefs
	if prev := r.replace(1); prev != 0 {
		t.Fatalf("first replace returned %d to delete; want 0", prev)
	}
	held := r.acquire()
	if held != 1 {
		t.Fatalf("acquire = %d; want 1", held)
	}
	if prev := r.replace(2); prev != 0 {
		t.Fatalf("replace deleted %d while a callback held it", prev)
	}
	if got := r.acquire(); got != 2 {
		t.Fatalf("acquire after replace = %d; want 2", got)
	} else if r.release(got) {
		t.Fatal("releasing the current reference asked to delete it")
	}
	if !r.release(held) {
		t.Fatal("the last release of the retired reference did not ask to delete it")
	}
	if len(r.held) != 0 || len(r.retired) != 0 {
		t.Errorf("leftover state: held %v, retired %v", r.held, r.retired)
	}
}

// TestCtxRefsDeletesAnUnheldReferenceAtOnce: with no callback running, the
// replaced reference is the replacement's to delete, as before.
func TestCtxRefsDeletesAnUnheldReferenceAtOnce(t *testing.T) {
	var r ctxRefs
	r.replace(1)
	r.release(r.acquire())
	if prev := r.replace(2); prev != 1 {
		t.Fatalf("replace returned %d to delete; want 1", prev)
	}
}

// TestCtxRefsDeletesEachReferenceOnce runs holders against replacements and
// checks that every retired reference is deleted exactly once and none is
// deleted while held.
func TestCtxRefsDeletesEachReferenceOnce(t *testing.T) {
	var r ctxRefs
	var mu sync.Mutex
	deleted := map[uintptr]int{}
	inUse := map[uintptr]int{}
	del := func(c uintptr) {
		mu.Lock()
		defer mu.Unlock()
		if inUse[c] > 0 {
			t.Errorf("reference %d deleted while %d callback(s) held it", c, inUse[c])
		}
		deleted[c]++
	}
	r.replace(1)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				c := r.acquire()
				mu.Lock()
				inUse[c]++
				mu.Unlock()
				mu.Lock()
				inUse[c]--
				mu.Unlock()
				if r.release(c) {
					del(c)
				}
			}
		}()
	}
	for c := uintptr(2); c <= 500; c++ {
		if prev := r.replace(c); prev != 0 {
			del(prev)
		}
	}
	wg.Wait()
	for c := uintptr(1); c < 500; c++ {
		if deleted[c] != 1 {
			t.Errorf("reference %d deleted %d times; want once", c, deleted[c])
		}
	}
	if deleted[500] != 0 {
		t.Errorf("the current reference was deleted")
	}
}
