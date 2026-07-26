// Package ga4 is a dumb [analytics.Sink] over the Google Analytics 4
// Measurement Protocol. It maps each analytics event to a GA4 event and POSTs
// one gzipped batch per flush. Apps that need a different mapping can bring
// their own sink.
//
// GA4 requires a client_id. This package uses the app-supplied identity from
// [analytics.Tracker.Identify] (Event.UserID) when present, falling back to the
// ephemeral session id — it never synthesises a device identifier.
package ga4

import (
	"net/url"

	"github.com/belluikit/bellui/analytics"
)

// endpoint is the GA4 Measurement Protocol collection URL.
const endpoint = "https://www.google-analytics.com/mp/collect"

// Sink delivers events to GA4 via the Measurement Protocol.
type Sink struct {
	measurementID string
	apiSecret     string
	url           string
}

var _ analytics.Sink = (*Sink)(nil)

// New returns a GA4 [analytics.Sink]. measurementID is the "G-XXXX" stream id
// and apiSecret is a Measurement Protocol API secret.
func New(measurementID, apiSecret string) *Sink {
	q := url.Values{}
	q.Set("measurement_id", measurementID)
	q.Set("api_secret", apiSecret)
	return &Sink{
		measurementID: measurementID,
		apiSecret:     apiSecret,
		url:           endpoint + "?" + q.Encode(),
	}
}

type ga4Event struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
}

type ga4Payload struct {
	ClientID string     `json:"client_id"`
	UserID   string     `json:"user_id,omitempty"`
	Events   []ga4Event `json:"events"`
}

// Send implements [analytics.Sink]. GA4 accepts up to 25 events per request; a
// larger batch is split into successive requests.
func (s *Sink) Send(batch []analytics.Event) error {
	const maxPerRequest = 25
	for start := 0; start < len(batch); start += maxPerRequest {
		end := min(start+maxPerRequest, len(batch))
		chunk := batch[start:end]

		clientID := clientIDFor(chunk)
		payload := ga4Payload{ClientID: clientID, Events: make([]ga4Event, 0, len(chunk))}
		for _, e := range chunk {
			if e.Kind == analytics.KindIdentify {
				if e.UserID != "" {
					payload.UserID = e.UserID
				}
				continue // GA4 has no distinct identify call
			}
			payload.Events = append(payload.Events, ga4Event{Name: e.Name, Params: e.Props})
		}
		if len(payload.Events) == 0 {
			continue
		}
		if err := analytics.PostJSONGzip(s.url, nil, payload); err != nil {
			return err
		}
	}
	return nil
}

// clientIDFor picks a stable-ish client id for a chunk: the app-supplied user id
// if any, else the session id.
func clientIDFor(chunk []analytics.Event) string {
	for _, e := range chunk {
		if e.UserID != "" {
			return e.UserID
		}
	}
	for _, e := range chunk {
		if e.SessionID != "" {
			return e.SessionID
		}
	}
	return "anonymous"
}
