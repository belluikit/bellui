package thread

import (
	"sync"
	"testing"
)

func TestGoroutineIDMatchesStackTrace(t *testing.T) {
	if got, want := GoroutineID(), stackGoroutineID(); got != want {
		t.Fatalf("GoroutineID() = %d, stack trace says %d", got, want)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, want := GoroutineID(), stackGoroutineID(); got != want {
				errs <- "GoroutineID disagrees with the stack trace"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

// Fails the day a Go release changes the goroutine record so that the ID can no
// longer be found in it. GoroutineID still works then, but at the stack trace's
// cost (#457).
func TestGoroutineIDReadsTheRecord(t *testing.T) {
	if !haveGetg {
		t.Skip("no getg on this platform; GoroutineID parses stack traces")
	}
	if goidOffset < 0 {
		t.Fatal("the goroutine ID was not found in the runtime's goroutine record")
	}
}

func BenchmarkGoroutineID(b *testing.B) {
	for b.Loop() {
		GoroutineID()
	}
}

func BenchmarkStackGoroutineID(b *testing.B) {
	for b.Loop() {
		stackGoroutineID()
	}
}
