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

// BenchmarkSpoolMarshal compares the spool encoding the tracker runs on every
// Track, PXF since #523, with the JSON it replaced.
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
