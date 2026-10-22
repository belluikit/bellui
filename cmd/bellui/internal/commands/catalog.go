package commands

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/trendvidia/protowire-go/encoding/pxf"
)

// A translation catalog is PXF (bellui#522): one messages map, keyed by
// message ID, whose values are go-i18n's messages. A string is the
// translation; a block holds the plural forms and the optional description and
// hash; a map of IDs nests a namespace.
//
//	messages = {
//	  "Cancel": "Cancel"
//	  "file.selectedCount": {
//	    one = "{Count} file selected"
//	    other = "{Count} files selected"
//	  }
//	}
//
// This reads and writes the format the toolkit's lang package reads. The CLI
// builds against a released toolkit, so it keeps its own copy.

// messageFields are the keys of a go-i18n message block. A block with any of
// them is a message, written with "key = value"; any other block is a
// namespace of IDs, written with "key: value".
var messageFields = map[string]bool{
	"id": true, "description": true, "hash": true, "leftdelim": true, "rightdelim": true,
	"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true,
}

// readCatalog decodes a PXF catalog into a map of strings and nested maps.
func readCatalog(data []byte) (map[string]any, error) {
	doc, err := pxf.Parse(data)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	seen := false
	for _, e := range doc.Entries {
		a, ok := e.(*pxf.Assignment)
		if !ok || a.Key != "messages" {
			return nil, errors.New("a catalog holds one messages = { … } map")
		}
		if seen {
			return nil, errors.New("messages is set twice")
		}
		seen = true
		block, ok := a.Value.(*pxf.BlockVal)
		if !ok {
			return nil, errors.New("messages must be a { … } map")
		}
		if m, err = catalogMap(block.Entries, "messages"); err != nil {
			return nil, err
		}
	}
	return m, nil
}

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
			key, val = e.Name, &pxf.BlockVal{Entries: e.Entries}
		default:
			return nil, fmt.Errorf("%s: unexpected %T", at, e)
		}
		if _, dup := m[key]; dup {
			return nil, fmt.Errorf("%s: %q is set twice", at, key)
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
			return nil, fmt.Errorf("%s.%s: a message is a string or a { … } block", at, key)
		}
	}
	return m, nil
}

// formatCatalog encodes a catalog as PXF, with IDs in sorted order. Values are
// strings, map[string]string (a message block) or map[string]any.
func formatCatalog(m map[string]any) []byte {
	doc := &pxf.Document{Entries: []pxf.Entry{
		&pxf.Assignment{Key: "messages", Value: &pxf.BlockVal{Entries: catalogEntries(m, false)}},
	}}
	return pxf.FormatDocument(doc)
}

// catalogEntries lists m's entries sorted by key: as message fields
// ("one = …") inside a message block, as map entries ("\"ID\": …") otherwise.
func catalogEntries(m map[string]any, message bool) []pxf.Entry {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entries := make([]pxf.Entry, 0, len(keys))
	for _, k := range keys {
		var val pxf.Value
		switch v := m[k].(type) {
		case string:
			val = &pxf.StringVal{Value: v}
		case map[string]string:
			nested := make(map[string]any, len(v))
			for nk, nv := range v {
				nested[nk] = nv
			}
			val = &pxf.BlockVal{Entries: catalogEntries(nested, isMessage(nested))}
		case map[string]any:
			val = &pxf.BlockVal{Entries: catalogEntries(v, isMessage(v))}
		default:
			val = &pxf.StringVal{Value: fmt.Sprint(v)}
		}
		if message {
			entries = append(entries, &pxf.Assignment{Key: k, Value: val})
		} else {
			entries = append(entries, &pxf.MapEntry{Key: k, Value: val})
		}
	}
	return entries
}

func isMessage(m map[string]any) bool {
	for k := range m {
		if messageFields[strings.ToLower(k)] {
			return true
		}
	}
	return false
}

// goTemplateField matches a Go template plain-field action, "{{.Name}}".
var goTemplateField = regexp.MustCompile(`\{\{\s*\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// namedPlaceholders rewrites Go template plain-field actions to named
// placeholders: "{{.Count}} files" becomes "{Count} files". Translations are
// written with named placeholders; the toolkit still fills the old form in
// source strings.
func namedPlaceholders(s string) string {
	return goTemplateField.ReplaceAllString(s, "{$1}")
}

// placeholderName matches a named placeholder, "{Name}".
var placeholderName = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// placeholderSet returns the names of the placeholders in s, in either form.
func placeholderSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, m := range placeholderName.FindAllStringSubmatch(namedPlaceholders(s), -1) {
		set[m[1]] = true
	}
	return set
}
