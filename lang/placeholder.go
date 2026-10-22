package lang

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n/template"
)

// placeholders fills a message's named placeholders (#522). It is go-i18n's
// template parser for every message: catalog translations and the source
// strings passed to L, X and their variants alike.
//
// A placeholder is a name in braces, "{Name}", as in ICU MessageFormat. The
// plain-field form of Go templates, "{{.Name}}", is deprecated: it is filled
// the same way, so source strings written for the text/template parser keep
// working, and each is logged once (#667). A name is filled from a map's key or
// from a struct's exported field (methods are not called), and formatted as
// text/template printed it. With several data values, the first that has the
// name fills it. A name the data does not have, a brace that does not hold a
// name ("{}", "{ x }") and any other template action ("{{if .X}}") are left as
// written; a name that only a method of the data has is logged once, pointing
// to a map.
var placeholders template.Parser = placeholderParser{}

type placeholderParser struct{}

// Cacheable reports that a parsed message can be kept: it holds no state from
// the data it is filled with.
func (placeholderParser) Cacheable() bool { return true }

// Parse splits src into literal text and placeholders. The delimiters are
// go-i18n's per-message Go template delimiters; they have no meaning here.
func (placeholderParser) Parse(src, _, _ string) (template.ParsedTemplate, error) {
	return parsePlaceholders(src), nil
}

// parsedMessage is a parsed message: literal text, with the placeholder name and
// its source spelling at each place one was written.
type parsedMessage struct {
	parts []messagePart
}

type messagePart struct {
	text string // literal text, or the placeholder as written
	name string // the placeholder's name; empty for literal text
}

func parsePlaceholders(src string) *parsedMessage {
	m := &parsedMessage{}
	literal := 0
	for i := 0; i < len(src); {
		if src[i] != '{' {
			i++
			continue
		}
		name, n := placeholderAt(src[i:])
		if n == 0 {
			if strings.HasPrefix(src[i:], "{{") {
				if end := strings.Index(src[i+2:], "}}"); end >= 0 {
					warnUnsupportedAction(src[i : i+2+end+2])
					i += 2 + end + 2
					continue
				}
			}
			i++
			continue
		}
		if literal < i {
			m.parts = append(m.parts, messagePart{text: src[literal:i]})
		}
		if src[i+1] == '{' {
			warnDeprecatedForm(src[i:i+n], name)
		}
		m.parts = append(m.parts, messagePart{text: src[i : i+n], name: name})
		i += n
		literal = i
	}
	if literal < len(src) {
		m.parts = append(m.parts, messagePart{text: src[literal:]})
	}
	return m
}

// placeholderAt reads a placeholder at the start of s, "{Name}" or
// "{{.Name}}" (spaces allowed inside the double braces), and returns its name
// and length; n is 0 when s does not start with one.
func placeholderAt(s string) (name string, n int) {
	if strings.HasPrefix(s, "{{") {
		inner := s[2:]
		end := strings.Index(inner, "}}")
		if end < 0 {
			return "", 0
		}
		action := strings.TrimSpace(inner[:end])
		if !strings.HasPrefix(action, ".") || !isName(action[1:]) {
			return "", 0
		}
		return action[1:], 2 + end + 2
	}
	end := strings.IndexByte(s, '}')
	if end < 0 || !isName(s[1:end]) {
		return "", 0
	}
	return s[1:end], end + 1
}

func isName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		letter := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !letter && (i == 0 || r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// warnedActions keeps unsupported template actions already logged, so that a
// message filled every frame logs once.
var warnedActions sync.Map

func warnUnsupportedAction(action string) {
	if _, seen := warnedActions.LoadOrStore(action, true); !seen {
		log.Printf("lang: template action %q is not supported and is left as written; use a {Name} placeholder", action)
	}
}

// warnDeprecatedForm logs, once per spelling, a placeholder written in the Go
// template form, which is deprecated in favour of "{Name}" (#667).
func warnDeprecatedForm(written, name string) {
	if _, seen := warnedActions.LoadOrStore(written, true); !seen {
		log.Printf("lang: placeholder %q is deprecated; write {%s}", written, name)
	}
}

// warnedMethods keeps the type-and-name pairs already logged by warnMethodOnly.
var warnedMethods sync.Map

// warnMethodOnly logs, once per type and name, a placeholder left unfilled
// although a data value has a method of that name: values come from map keys
// and struct fields only, and text/template used to call the method (#667).
func warnMethodOnly(data any, name string) {
	for _, d := range dataValues(data) {
		t := reflect.TypeOf(d)
		if t == nil {
			continue
		}
		if _, ok := t.MethodByName(name); !ok {
			continue
		}
		key := t.String() + "." + name
		if _, seen := warnedMethods.LoadOrStore(key, true); !seen {
			log.Printf("lang: {%s} is left as written: %s has a method %s, but placeholders are filled only from map keys and struct fields; pass map[string]any{%q: v.%s()}", name, t, name, name, name)
		}
		return
	}
}

// dataList holds the data values of a call that passed more than one. A name
// is filled from the first value that has it.
type dataList []any

// templateData is the data go-i18n passes to a message for a call's data
// arguments: nil, the one value, or a dataList of them all.
func templateData(data []any) any {
	switch len(data) {
	case 0:
		return nil
	case 1:
		return data[0]
	}
	return dataList(data)
}

// dataValues lists the values in data, which may be a dataList.
func dataValues(data any) []any {
	if l, ok := data.(dataList); ok {
		return l
	}
	return []any{data}
}

// Execute fills the message's placeholders from data.
func (m *parsedMessage) Execute(data any) (string, error) {
	if len(m.parts) == 1 && m.parts[0].name == "" {
		return m.parts[0].text, nil
	}
	var b strings.Builder
	for _, p := range m.parts {
		if p.name == "" {
			b.WriteString(p.text)
			continue
		}
		if v, ok := lookupName(data, p.name); ok {
			b.WriteString(formatValue(v))
		} else {
			b.WriteString(p.text)
			warnMethodOnly(data, p.name)
		}
	}
	return b.String(), nil
}

// lookupName finds name in data: a key of a map whose keys are strings (or
// interfaces a string fits), or an exported field of a struct, through any
// pointers. In a dataList, the first value that has the name wins.
func lookupName(data any, name string) (any, bool) {
	switch d := data.(type) {
	case nil:
		return nil, false
	case dataList:
		for _, v := range d {
			if x, ok := lookupName(v, name); ok {
				return x, true
			}
		}
		return nil, false
	case map[string]any:
		v, ok := d[name]
		return v, ok
	case map[string]string:
		v, ok := d[name]
		return v, ok
	case map[string]int:
		v, ok := d[name]
		return v, ok
	}

	v := reflect.ValueOf(data)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Map:
		key := reflect.ValueOf(name)
		switch kt := v.Type().Key(); {
		case key.Type().AssignableTo(kt):
		case kt.Kind() == reflect.String:
			key = key.Convert(kt)
		default:
			return nil, false
		}
		e := v.MapIndex(key)
		if !e.IsValid() {
			return nil, false
		}
		return e.Interface(), true
	case reflect.Struct:
		f, ok := v.Type().FieldByName(name)
		if !ok || !f.IsExported() {
			return nil, false
		}
		// A field promoted through a nil embedded pointer has no value.
		fv, err := v.FieldByIndexErr(f.Index)
		if err != nil {
			return nil, false
		}
		return fv.Interface(), true
	}
	return nil, false
}

var (
	errorType    = reflect.TypeFor[error]()
	stringerType = reflect.TypeFor[fmt.Stringer]()
)

// formatValue prints a value as text/template printed it: as fmt.Sprint
// does, but through any pointers, using a String or Error method the pointer
// has when the value itself has none.
func formatValue(x any) string {
	v := reflect.ValueOf(x)
	if v.Kind() != reflect.Pointer {
		return fmt.Sprint(x)
	}
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return fmt.Sprint(v.Interface())
		}
		v = v.Elem()
	}
	if v.CanAddr() && !v.Type().Implements(errorType) && !v.Type().Implements(stringerType) {
		if p := reflect.PointerTo(v.Type()); p.Implements(errorType) || p.Implements(stringerType) {
			v = v.Addr()
		}
	}
	return fmt.Sprint(v.Interface())
}
