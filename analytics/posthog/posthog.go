// Package posthog is a dumb [analytics.Sink] over the PostHog batch capture
// API. It maps each analytics event to a PostHog event and POSTs one gzipped
// batch per flush. Apps that need a different mapping can bring their own sink.
//
// PostHog requires a distinct_id. This package uses the app-supplied identity
// from [analytics.Tracker.Identify] (Event.UserID) when present, falling back to
// the ephemeral session id — it never synthesises a device identifier.
package posthog

import (
	"strings"

	"github.com/belluikit/bellui/analytics"
)

// DefaultHost is PostHog's US cloud host. Pass a different host to [New] for EU
// cloud or self-hosted instances.
const DefaultHost = "https://us.i.posthog.com"

// Sink delivers events to PostHog's /batch endpoint.
type Sink struct {
	apiKey string
	url    string
}

var _ analytics.Sink = (*Sink)(nil)

// New returns a PostHog [analytics.Sink] for the given project API key, using
// [DefaultHost].
func New(apiKey string) *Sink {
	return NewWithHost(apiKey, DefaultHost)
}

// NewWithHost returns a PostHog [analytics.Sink] posting to a custom host
// (EU cloud or self-hosted).
func NewWithHost(apiKey, host string) *Sink {
	return &Sink{
		apiKey: apiKey,
		url:    strings.TrimRight(host, "/") + "/batch/",
	}
}

type phEvent struct {
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties,omitempty"`
	Timestamp  string         `json:"timestamp"`
}

type phPayload struct {
	APIKey string    `json:"api_key"`
	Batch  []phEvent `json:"batch"`
}

// Send implements [analytics.Sink].
func (s *Sink) Send(batch []analytics.Event) error {
	payload := phPayload{APIKey: s.apiKey, Batch: make([]phEvent, 0, len(batch))}
	for _, e := range batch {
		ev := phEvent{
			DistinctID: distinctID(e),
			Timestamp:  e.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		}
		switch e.Kind {
		case analytics.KindIdentify:
			ev.Event = "$identify"
			if len(e.Traits) > 0 {
				ev.Properties = map[string]any{"$set": e.Traits}
			}
		default:
			ev.Event = e.Name
			ev.Properties = e.Props
		}
		payload.Batch = append(payload.Batch, ev)
	}
	if len(payload.Batch) == 0 {
		return nil
	}
	return analytics.PostJSONGzip(s.url, nil, payload)
}

func distinctID(e analytics.Event) string {
	if e.UserID != "" {
		return e.UserID
	}
	if e.SessionID != "" {
		return e.SessionID
	}
	return "anonymous"
}
