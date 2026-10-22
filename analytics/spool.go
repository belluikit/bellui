package analytics

import (
	"encoding/json"

	"github.com/trendvidia/protowire-go/encoding/pxf"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/belluikit/bellui/internal/analytics/spoolpb"
	"github.com/belluikit/bellui/internal/pxfmigrate"
)

// marshalSpool encodes queued events for the disk spool, as PXF (#523). Strings
// that are not valid UTF-8 are written as JSON wrote them: PXF would write them
// as they are and then refuse to read the whole spool back.
func marshalSpool(events []Event) ([]byte, error) {
	msg := &spoolpb.Spool{Events: make([]*spoolpb.Event, len(events))}
	for i, e := range events {
		msg.Events[i] = &spoolpb.Event{
			Kind:      pxfmigrate.ValidUTF8(string(e.Kind)),
			Name:      pxfmigrate.ValidUTF8(e.Name),
			Props:     toStruct(e.Props),
			Traits:    toStruct(e.Traits),
			UserId:    pxfmigrate.ValidUTF8(e.UserID),
			SessionId: pxfmigrate.ValidUTF8(e.SessionID),
			Timestamp: timestamppb.New(e.Timestamp),
		}
	}
	return pxf.Marshal(msg)
}

// unmarshalSpool decodes a disk spool: PXF, or the JSON array versions before
// #523 wrote. A JSON spool is an array, so it starts with '['; a PXF one starts
// with a field name.
func unmarshalSpool(data []byte) ([]Event, error) {
	if pxfmigrate.IsJSON(data, '[') {
		var events []Event
		err := json.Unmarshal(data, &events)
		return events, err
	}
	var msg spoolpb.Spool
	if err := pxf.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	events := make([]Event, len(msg.GetEvents()))
	for i, e := range msg.GetEvents() {
		events[i] = Event{
			Kind:      EventKind(e.GetKind()),
			Name:      e.GetName(),
			Props:     fromStruct(e.GetProps()),
			Traits:    fromStruct(e.GetTraits()),
			UserID:    e.GetUserId(),
			SessionID: e.GetSessionId(),
		}
		if e.GetTimestamp() != nil {
			events[i].Timestamp = e.GetTimestamp().AsTime().Local()
		}
	}
	return events, nil
}

// toStruct converts event properties for the spool. A value structpb takes as
// it is (strings, booleans, numbers, nil, []any, map[string]any) is kept; any
// other goes through JSON first, as the whole spool did before #523, so a
// time.Time becomes its JSON string and a struct its JSON object. Either way the
// value reads back as JSON would have given it: numbers as float64, and bytes
// that are not valid UTF-8, in a key or a string, as U+FFFD.
func toStruct(m map[string]any) *structpb.Struct {
	if len(m) == 0 {
		return nil
	}
	s := &structpb.Struct{Fields: make(map[string]*structpb.Value, len(m))}
	for k, v := range m {
		pv, err := structpb.NewValue(v)
		if err != nil {
			pv = jsonValue(v)
		}
		if pv != nil {
			s.Fields[pxfmigrate.ValidUTF8(k)] = pv
		}
	}
	return s
}

// jsonValue converts v to a structpb value through its JSON encoding, or
// returns nil when JSON cannot encode it either; the JSON spool failed to save
// at all then, where this drops only that property.
func jsonValue(v any) *structpb.Value {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return nil
	}
	pv, err := structpb.NewValue(generic)
	if err != nil {
		return nil
	}
	return pv
}

func fromStruct(s *structpb.Struct) map[string]any {
	if len(s.GetFields()) == 0 {
		return nil
	}
	return s.AsMap()
}
