package app

import (
	"github.com/belluikit/bellui/internal/preferences/prefspb"
	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// isLegacyJSONPreferences reports whether the stored bytes are a legacy JSON
// preferences object rather than PXF. The JSON store is always a top-level object
// so its first non-whitespace byte is '{'; PXF (textproto) for the Preferences
// message begins with the `values` field name, never '{'. This deterministic sniff
// drives the read-both migration window and avoids relying on the decoder's error
// behavior to distinguish the two formats.
func isLegacyJSONPreferences(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// marshalPreferencesPXF encodes the in-memory preference value map to PXF bytes.
// Values whose type is outside the supported preference set are skipped defensively
// rather than failing the whole save.
func marshalPreferencesPXF(values map[string]any) ([]byte, error) {
	msg := &prefspb.Preferences{Values: make(map[string]*prefspb.Value, len(values))}
	for k, v := range values {
		if pv, ok := preferenceValueToProto(v); ok {
			msg.Values[k] = pv
		}
	}
	return pxf.Marshal(msg)
}

// unmarshalPreferencesPXF decodes PXF bytes into the provided value map. It returns
// an error if the bytes are not valid PXF, which the caller uses to fall back to the
// legacy JSON decoder during the migration window.
func unmarshalPreferencesPXF(data []byte, values map[string]any) error {
	var msg prefspb.Preferences
	if err := pxf.Unmarshal(data, &msg); err != nil {
		return err
	}
	for k, pv := range msg.GetValues() {
		if v, ok := preferenceValueFromProto(pv); ok {
			values[k] = v
		}
	}
	return nil
}

// preferenceValueToProto converts a single in-memory preference value to its proto
// form. int and float stay distinct (unlike the JSON store, where both were numbers),
// but the typed getters coerce between them so either persisted form reads back fine.
func preferenceValueToProto(v any) (*prefspb.Value, bool) {
	switch t := v.(type) {
	case bool:
		return &prefspb.Value{Kind: &prefspb.Value_BoolValue{BoolValue: t}}, true
	case int:
		return &prefspb.Value{Kind: &prefspb.Value_IntValue{IntValue: int64(t)}}, true
	case int64:
		return &prefspb.Value{Kind: &prefspb.Value_IntValue{IntValue: t}}, true
	case float64:
		return &prefspb.Value{Kind: &prefspb.Value_FloatValue{FloatValue: t}}, true
	case float32:
		return &prefspb.Value{Kind: &prefspb.Value_FloatValue{FloatValue: float64(t)}}, true
	case string:
		return &prefspb.Value{Kind: &prefspb.Value_StringValue{StringValue: t}}, true
	case []bool:
		return &prefspb.Value{Kind: &prefspb.Value_BoolList{BoolList: &prefspb.BoolList{Values: t}}}, true
	case []int:
		ints := make([]int64, len(t))
		for i, n := range t {
			ints[i] = int64(n)
		}
		return &prefspb.Value{Kind: &prefspb.Value_IntList{IntList: &prefspb.IntList{Values: ints}}}, true
	case []int64:
		return &prefspb.Value{Kind: &prefspb.Value_IntList{IntList: &prefspb.IntList{Values: t}}}, true
	case []float64:
		return &prefspb.Value{Kind: &prefspb.Value_FloatList{FloatList: &prefspb.FloatList{Values: t}}}, true
	case []string:
		return &prefspb.Value{Kind: &prefspb.Value_StringList{StringList: &prefspb.StringList{Values: t}}}, true
	case []any:
		// An empty untyped list survives from a migrated legacy JSON store
		// (convertLists leaves an empty JSON array as []any). Preserve the key as an
		// empty list rather than silently dropping it on the PXF save. A non-empty
		// []any is unreachable here (convertLists types those), so only the empty
		// case is handled.
		if len(t) == 0 {
			return &prefspb.Value{Kind: &prefspb.Value_StringList{StringList: &prefspb.StringList{Values: []string{}}}}, true
		}
	}
	return nil, false
}

// preferenceValueFromProto converts a proto Value back to the Go type the in-memory
// store and the typed getters (see root preferences.go coerce) expect: int (not int64)
// for scalars and []int for lists, matching what SetPref[int]/SetPrefList[int] store.
func preferenceValueFromProto(pv *prefspb.Value) (any, bool) {
	switch k := pv.GetKind().(type) {
	case *prefspb.Value_BoolValue:
		return k.BoolValue, true
	case *prefspb.Value_IntValue:
		return int(k.IntValue), true
	case *prefspb.Value_FloatValue:
		return k.FloatValue, true
	case *prefspb.Value_StringValue:
		return k.StringValue, true
	case *prefspb.Value_BoolList:
		if src := k.BoolList.GetValues(); len(src) > 0 {
			return append([]bool(nil), src...), true
		}
	case *prefspb.Value_IntList:
		if src := k.IntList.GetValues(); len(src) > 0 {
			ints := make([]int, len(src))
			for i, n := range src {
				ints[i] = int(n)
			}
			return ints, true
		}
	case *prefspb.Value_FloatList:
		if src := k.FloatList.GetValues(); len(src) > 0 {
			return append([]float64(nil), src...), true
		}
	case *prefspb.Value_StringList:
		if src := k.StringList.GetValues(); len(src) > 0 {
			return append([]string(nil), src...), true
		}
	default:
		return nil, false
	}
	// An empty list of any element type maps back to an untyped empty list, matching
	// the legacy JSON store (convertLists), so PrefListOr[T] adapts to the reader's
	// requested type and returns an empty slice rather than the fallback.
	return []any{}, true
}
