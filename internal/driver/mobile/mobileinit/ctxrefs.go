package mobileinit

import "sync"

// ctxRefs tracks the Android Context global reference and how many RunOnJVM
// callbacks are using each one, so that the reference a resume replaces is
// deleted only once nothing holds it.
//
// Every onResume makes a new global reference to the activity and retires the
// last one. A callback already running with the old reference used to have it
// deleted under it, and the next JNI call that passed it aborted the process
// ("jobject is an invalid global reference … deleted reference"). Picking a
// file did it: the picker's result runs as the activity resumes (#852).
//
// Nothing blocks on it: a resume never waits for a callback, and a callback
// that calls RunOnJVM again cannot deadlock against a resume.
type ctxRefs struct {
	mu      sync.Mutex
	current uintptr
	held    map[uintptr]int
	retired map[uintptr]bool
}

// acquire returns the current reference, held until release. It may be 0.
func (r *ctxRefs) acquire() uintptr {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.current
	if c != 0 {
		if r.held == nil {
			r.held = map[uintptr]int{}
		}
		r.held[c]++
	}
	return c
}

// release ends a hold from acquire. It reports whether c was retired while
// held and this was its last holder, in which case the caller deletes it.
func (r *ctxRefs) release(c uintptr) bool {
	if c == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.held[c]--
	if r.held[c] > 0 {
		return false
	}
	delete(r.held, c)
	if !r.retired[c] {
		return false
	}
	delete(r.retired, c)
	return true
}

// replace makes c current. It returns the reference it replaced when nothing
// holds it, for the caller to delete now, and 0 otherwise: a held one is
// deleted by its last release.
func (r *ctxRefs) replace(c uintptr) uintptr {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := r.current
	r.current = c
	if prev == 0 || prev == c {
		return 0
	}
	if r.held[prev] > 0 {
		if r.retired == nil {
			r.retired = map[uintptr]bool{}
		}
		r.retired[prev] = true
		return 0
	}
	return prev
}
