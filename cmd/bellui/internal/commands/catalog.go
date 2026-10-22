package commands

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
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

// messageFields are go-i18n's reserved message keys. go-i18n reads a block as
// a message when its keys are these, matched without case; it is written with
// "key = value". Any other block is a namespace of IDs, written with
// "key: value".
var messageFields = map[string]bool{
	"id": true, "description": true, "hash": true, "leftdelim": true, "rightdelim": true,
	"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true,
	"translation": true,
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
	var b bytes.Buffer
	b.WriteString("messages = ")
	writeCatalogBlock(&b, m, false, 0)
	b.WriteByte('\n')
	return b.Bytes()
}

// writeCatalogBlock writes m as a { … } block, its entries sorted by key: as
// message fields ("one = …") in a message, as map entries ("\"ID\": …")
// otherwise.
func writeCatalogBlock(b *bytes.Buffer, m map[string]any, message bool, level int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("{\n")
	for _, k := range keys {
		b.WriteString(strings.Repeat("  ", level+1))
		if message {
			b.WriteString(k + " = ")
		} else {
			b.WriteString(catalogKey(k) + ": ")
		}
		switch v := m[k].(type) {
		case string:
			b.Write(pxf.FormatValue(&pxf.StringVal{Value: v}))
		case map[string]string:
			nested := make(map[string]any, len(v))
			for nk, nv := range v {
				nested[nk] = nv
			}
			writeCatalogBlock(b, nested, isMessage(nested), level+1)
		case map[string]any:
			writeCatalogBlock(b, v, isMessage(v), level+1)
		default:
			b.Write(pxf.FormatValue(&pxf.StringVal{Value: fmt.Sprint(v)}))
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat("  ", level) + "}")
}

// catalogKey writes a message ID as a map key: bare when PXF reads it back as
// that identifier, quoted otherwise. true, false and null are values in PXF,
// not identifiers.
func catalogKey(k string) string {
	switch k {
	case "", "true", "false", "null":
		return strconv.Quote(k)
	}
	for i, r := range k {
		letter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !letter && (i == 0 || r < '0' || r > '9') {
			return strconv.Quote(k)
		}
	}
	return k
}

// isMessage reports whether go-i18n reads m as a message: every key is a
// message field, holding a string ("translation" may hold anything). A block
// mixing fields and IDs is not a message to go-i18n either.
func isMessage(m map[string]any) bool {
	for k, v := range m {
		if !messageFields[strings.ToLower(k)] {
			return false
		}
		if _, ok := v.(string); !ok && k != "translation" {
			return false
		}
	}
	return len(m) > 0
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
