package lang

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// A translation catalog is PXF (#522): one messages map, keyed by message ID,
// whose values are go-i18n's messages. A string is the translation; a block
// holds the plural forms (zero, one, two, few, many, other) and the optional
// description and hash; a map of IDs nests a namespace, as "." does in an ID.
//
//	messages = {
//	  "Cancel": "Abbrechen"
//	  "file.selectedCount": {
//	    one = "{Count} Datei ausgewählt"
//	    other = "{Count} Dateien ausgewählt"
//	  }
//	}

// errJSONCatalog is returned for a catalog file named .json: JSON catalogs
// are not read since #522.
var errJSONCatalog = errors.New("lang: JSON translation catalogs are not supported; convert the catalog to PXF (see the lang package documentation)")

// checkCatalogName rejects a catalog file that is not PXF, by its name.
func checkCatalogName(name string) error {
	switch path.Ext(name) {
	case ".pxf":
		return nil
	case ".json":
		return fmt.Errorf("%s: %w", name, errJSONCatalog)
	}
	return fmt.Errorf("lang: %s: a translation catalog is a .pxf file named [prefix.]locale.pxf", name)
}

// unmarshalCatalog is go-i18n's reader for PXF catalogs. It stores in v,
// which go-i18n passes as a *any, the nested map of strings the JSON reader
// gave go-i18n, so go-i18n reads the messages the same way.
func unmarshalCatalog(data []byte, v any) error {
	out, ok := v.(*any)
	if !ok {
		return fmt.Errorf("lang: catalog reader needs a *any, got %T", v)
	}
	doc, err := pxf.Parse(data)
	if err != nil {
		return err
	}
	var messages *pxf.BlockVal
	for _, e := range doc.Entries {
		a, ok := e.(*pxf.Assignment)
		if !ok || a.Key != "messages" {
			return fmt.Errorf("lang: catalog: unexpected %s; a catalog holds one messages = { … } map", entryName(e))
		}
		if messages != nil {
			return errors.New("lang: catalog: messages is set twice")
		}
		block, ok := a.Value.(*pxf.BlockVal)
		if !ok {
			return errors.New("lang: catalog: messages must be a { … } map")
		}
		messages = block
	}
	if messages == nil {
		*out = map[string]any{}
		return nil
	}
	m, err := catalogMap(messages.Entries, "messages")
	if err != nil {
		return err
	}
	*out = m
	return nil
}

// catalogMap converts a block's entries, "key: value" or "key = value", to a
// map of strings and nested maps.
func catalogMap(entries []pxf.Entry, at string) (map[string]any, error) {
	m := make(map[string]any, len(entries))
	for _, e := range entries {
		var key string
		var val pxf.Value
		switch e := e.(type) {
		case *pxf.MapEntry:
			key, val = e.Key, e.Value
		case *pxf.Assignment:
			key, val = e.Key, e.Value
		case *pxf.Block:
			nested, err := catalogMap(e.Entries, at+"."+e.Name)
			if err != nil {
				return nil, err
			}
			m[e.Name] = nested
			continue
		default:
			return nil, fmt.Errorf("lang: catalog: %s: unexpected %s", at, entryName(e))
		}
		if _, dup := m[key]; dup {
			return nil, fmt.Errorf("lang: catalog: %s: %q is set twice", at, key)
		}
		switch v := val.(type) {
		case *pxf.StringVal:
			m[key] = v.Value
		case *pxf.BlockVal:
			nested, err := catalogMap(v.Entries, at+"."+key)
			if err != nil {
				return nil, err
			}
			m[key] = nested
		default:
			return nil, fmt.Errorf("lang: catalog: %s.%s: a message is a string or a { … } block", at, key)
		}
	}
	return m, nil
}

func entryName(e pxf.Entry) string {
	switch e := e.(type) {
	case *pxf.Assignment:
		return strings.TrimSpace(e.Key + " =")
	case *pxf.MapEntry:
		return fmt.Sprintf("%q:", e.Key)
	case *pxf.Block:
		return e.Name + " { … }"
	}
	return fmt.Sprintf("%T", e)
}
