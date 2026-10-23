package mobile

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/belluikit/bellui/cmd/bellui/internal/templates"
)

// plistValue is one value of an XML property list, kept as a tree that
// remembers dict key order, so a merged plist serializes deterministically
// with the generated keys where the generator put them.
//
// Kind is the element name: dict, array, string, integer, real, true, false,
// date or data. Text holds the scalar content; Keys/Vals the dict entries in
// order; Items the array elements.
type plistValue struct {
	Kind  string
	Text  string
	Keys  []string
	Vals  []*plistValue
	Items []*plistValue
}

func plistString(s string) *plistValue { return &plistValue{Kind: "string", Text: s} }

func plistStringArray(ss []string) *plistValue {
	a := &plistValue{Kind: "array"}
	for _, s := range ss {
		a.Items = append(a.Items, plistString(s))
	}
	return a
}

// get returns the value of key in a dict, or nil.
func (v *plistValue) get(key string) *plistValue {
	for i, k := range v.Keys {
		if k == key {
			return v.Vals[i]
		}
	}
	return nil
}

// set replaces key's value in place, or appends the key when the dict lacks it.
func (v *plistValue) set(key string, val *plistValue) {
	for i, k := range v.Keys {
		if k == key {
			v.Vals[i] = val
			return
		}
	}
	v.Keys = append(v.Keys, key)
	v.Vals = append(v.Vals, val)
}

// parsePlist reads an XML property list whose top-level value is a dict.
func parsePlist(data []byte) (*plistValue, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil, errors.New("no <plist> element")
			}
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if se.Name.Local != "plist" {
			return nil, fmt.Errorf("root element is <%s>, want <plist>", se.Name.Local)
		}
		v, err := parsePlistNext(d)
		if err != nil {
			return nil, err
		}
		if v == nil || v.Kind != "dict" {
			return nil, errors.New("top-level value must be a <dict>")
		}
		return v, nil
	}
}

// parsePlistNext reads the next value element, or returns nil at the end of
// the enclosing element.
func parsePlistNext(d *xml.Decoder) (*plistValue, error) {
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil, nil
		case xml.StartElement:
			return parsePlistElement(d, t)
		case xml.CharData:
			if len(bytes.TrimSpace(t)) != 0 {
				return nil, fmt.Errorf("unexpected text %q", strings.TrimSpace(string(t)))
			}
		}
	}
}

func parsePlistElement(d *xml.Decoder, se xml.StartElement) (*plistValue, error) {
	v := &plistValue{Kind: se.Name.Local}
	switch v.Kind {
	case "dict":
		seen := map[string]bool{}
		for {
			tok, err := d.Token()
			if err != nil {
				return nil, err
			}
			switch t := tok.(type) {
			case xml.EndElement:
				return v, nil
			case xml.CharData:
				if len(bytes.TrimSpace(t)) != 0 {
					return nil, fmt.Errorf("unexpected text %q in <dict>", strings.TrimSpace(string(t)))
				}
			case xml.StartElement:
				if t.Name.Local != "key" {
					return nil, fmt.Errorf("<dict> holds <%s> where a <key> belongs", t.Name.Local)
				}
				var key string
				if err := d.DecodeElement(&key, &t); err != nil {
					return nil, err
				}
				if seen[key] {
					return nil, fmt.Errorf("key %q appears twice", key)
				}
				seen[key] = true
				val, err := parsePlistNext(d)
				if err != nil {
					return nil, err
				}
				if val == nil {
					return nil, fmt.Errorf("key %q has no value", key)
				}
				v.Keys = append(v.Keys, key)
				v.Vals = append(v.Vals, val)
			}
		}
	case "array":
		for {
			item, err := parsePlistNext(d)
			if err != nil {
				return nil, err
			}
			if item == nil {
				return v, nil
			}
			v.Items = append(v.Items, item)
		}
	case "true", "false":
		if err := d.Skip(); err != nil {
			return nil, err
		}
		return v, nil
	case "string", "integer", "real", "date", "data":
		if err := d.DecodeElement(&v.Text, &se); err != nil {
			return nil, err
		}
		if v.Kind != "string" {
			v.Text = strings.TrimSpace(v.Text)
		}
		if v.Kind == "data" {
			v.Text = strings.Join(strings.Fields(v.Text), "")
		}
		return v, nil
	default:
		return nil, fmt.Errorf("unknown plist element <%s>", v.Kind)
	}
}

// encodePlist writes v as an XML property list, two-space indented.
func encodePlist(v *plistValue) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
`)
	writePlistValue(&b, v, "")
	b.WriteString("</plist>\n")
	return b.Bytes()
}

func writePlistValue(b *bytes.Buffer, v *plistValue, indent string) {
	switch v.Kind {
	case "dict":
		if len(v.Keys) == 0 {
			b.WriteString(indent + "<dict/>\n")
			return
		}
		b.WriteString(indent + "<dict>\n")
		for i, k := range v.Keys {
			b.WriteString(indent + "  <key>" + templates.XMLEscape(k) + "</key>\n")
			writePlistValue(b, v.Vals[i], indent+"  ")
		}
		b.WriteString(indent + "</dict>\n")
	case "array":
		if len(v.Items) == 0 {
			b.WriteString(indent + "<array/>\n")
			return
		}
		b.WriteString(indent + "<array>\n")
		for _, it := range v.Items {
			writePlistValue(b, it, indent+"  ")
		}
		b.WriteString(indent + "</array>\n")
	case "true", "false":
		b.WriteString(indent + "<" + v.Kind + "/>\n")
	default:
		b.WriteString(indent + "<" + v.Kind + ">" + templates.XMLEscape(v.Text) + "</" + v.Kind + ">\n")
	}
}
