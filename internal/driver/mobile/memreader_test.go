//go:build !wasm

package mobile

import (
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestMemReaderLoadsOnceUnderConcurrentReadAt pins #1031: the in-memory
// fallback for a picked iOS document loaded it on first use without a lock,
// so goroutines reading at the same moment each loaded it and wrote the data
// unsynchronised. Run with -race.
func TestMemReaderLoadsOnceUnderConcurrentReadAt(t *testing.T) {
	const doc = "0123456789abcdef"
	var loads atomic.Int32
	m := &memReader{load: func() []byte {
		loads.Add(1)
		time.Sleep(10 * time.Millisecond) // widen the window the readers race in
		return []byte(doc)
	}}

	const readers = 8
	var start, done sync.WaitGroup
	start.Add(1)
	got := make([]string, readers)
	for i := range readers {
		done.Go(func() {
			start.Wait()
			b := make([]byte, 4)
			n, err := m.ReadAt(b, int64(i))
			if err != nil {
				t.Errorf("reader %d: %v", i, err)
			}
			got[i] = string(b[:n])
		})
	}
	start.Done()
	done.Wait()

	if n := loads.Load(); n != 1 {
		t.Errorf("the document was loaded %d times, want once", n)
	}
	for i, g := range got {
		if want := doc[i : i+4]; g != want {
			t.Errorf("reader %d read %q, want %q", i, g, want)
		}
	}
}

// TestMemReaderReadsAndSeeks keeps the fallback's Read, Seek and ReadAt
// behaving as they did before #1031 moved them out of the iOS reader.
func TestMemReaderReadsAndSeeks(t *testing.T) {
	m := &memReader{load: func() []byte { return []byte("0123456789") }}

	if end, err := m.Seek(0, io.SeekEnd); err != nil || end != 10 {
		t.Fatalf("Seek(0, End) = %d, %v; want 10", end, err)
	}
	if _, err := m.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 3)
	if _, err := io.ReadFull(m, b); err != nil || string(b) != "456" {
		t.Fatalf("read %q, %v after seeking to 4; want 456", b, err)
	}

	// ReadAt leaves Read's offset where it was.
	if n, err := m.ReadAt(b, 8); n != 2 || err != io.EOF || string(b[:n]) != "89" {
		t.Fatalf("ReadAt(8) = %d %q, %v; want 2 \"89\", EOF", n, b[:n], err)
	}
	if _, err := io.ReadFull(m, b); err != nil || string(b) != "789" {
		t.Fatalf("read %q, %v after ReadAt; want 789", b, err)
	}
	if _, err := m.Read(b); err != io.EOF {
		t.Fatalf("Read at the end = %v, want EOF", err)
	}
	if _, err := m.ReadAt(b, -1); err == nil {
		t.Fatal("ReadAt at a negative offset succeeded")
	}
	if _, err := m.Seek(-1, io.SeekStart); err == nil {
		t.Fatal("Seek to a negative position succeeded")
	}
}
