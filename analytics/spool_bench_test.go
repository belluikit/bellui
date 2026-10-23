package analytics

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// benchQueue is n events shaped like a screen-view track call.
func benchQueue(n int) []Event {
	q := make([]Event, n)
	for i := range q {
		q[i] = Event{
			Kind: KindTrack, Name: "screen_view", SessionID: "lq3k2-abc123", Timestamp: time.Now(),
			Props: map[string]any{"screen": "settings", "index": i, "dark": true},
		}
	}
	return q
}

// BenchmarkSpoolMarshal compares encoding a whole spool as PXF (#523) with the
// JSON it replaced. Track no longer pays this per call: it encodes only the new
// event and joins cached pieces (#657); BenchmarkTrack measures that.
func BenchmarkSpoolMarshal(b *testing.B) {
	for _, n := range []int{DefaultBatchSize, DefaultMaxSpool} {
		q := benchQueue(n)
		b.Run(fmt.Sprintf("pxf/%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := marshalSpool(q); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("json/%d", n), func(b *testing.B) {
			for b.Loop() {
				if _, err := json.Marshal(q); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTrack is what Track costs its caller (the UI goroutine, under
// autocapture) with the queue already at n events: append, evict one, and save
// the spool, to an in-memory store so the encoding is what is measured (#657).
func BenchmarkTrack(b *testing.B) {
	for _, n := range []int{DefaultBatchSize, DefaultMaxSpool} {
		b.Run(fmt.Sprintf("queue=%d", n), func(b *testing.B) {
			tr := New(newFakeSink(), WithStore(&memStore{}), WithBatchSize(1<<30), WithMaxSpool(n)).(*tracker)
			defer tr.Close()
			tr.SetEnabled(true)
			for _, e := range benchQueue(n) {
				tr.Track(e.Name, e.Props)
			}
			props := map[string]any{"screen": "settings", "index": 7, "dark": true}
			b.ResetTimer()
			for b.Loop() {
				tr.Track("screen_view", props)
			}
		})
	}
}
