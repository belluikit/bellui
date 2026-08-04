package webview

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/belluikit/bellui"
)

// Args are the named arguments of [WebView.EvalFunc], marshalled to JSON and
// passed to the script as data. Each name must be a JavaScript identifier; it
// becomes a parameter of the function the body runs as.
//
// Since: 1.0
type Args map[string]any

// evalState tracks the results this view is still waiting for. Eval results
// are correlated by the platform, so they get their own id space, separate
// from the bridge's envelope ids.
type evalState struct {
	pending map[uint64]func(json.RawMessage, error)
	nextID  uint64
}

// Eval runs JavaScript inside the page and discards its result.
//
// js is source, so a caller that builds it by concatenating values from
// outside the app has an injection hole — a string with a quote in it becomes
// code. Prefer [WebView.EvalFunc], which passes values as data, or
// [WebView.Call], which hands them to a function the page registered.
//
// It is a no-op when the view is not attached; use [WebView.EvalResult] to
// learn that instead.
func (w *WebView) Eval(js string) {
	if !w.attached() {
		return
	}
	if w.evalHook != nil {
		w.evalHook(js, 0, false)
		return
	}
	w.evalRaw(js)
}

// EvalResult runs JavaScript inside the page and reports the value of its last
// expression to onResult, on the main goroutine.
//
// The value arrives as JSON: a script evaluating to a number gives "42", to a
// string gives "\"hi\"", and to nothing gives "null". Decode it with
// [encoding/json]. onResult is called exactly once — with an error if the
// script threw, if its value cannot be represented as JSON, or if the view is
// not attached ([ErrNotAttached]).
//
// The injection caveat on [WebView.Eval] applies here too.
//
// Since: 1.0
func (w *WebView) EvalResult(js string, onResult func(json.RawMessage, error)) {
	if !w.attached() {
		reportResult(onResult, ErrNotAttached)
		return
	}

	w.eval.nextID++
	id := w.eval.nextID
	if onResult != nil {
		if w.eval.pending == nil {
			w.eval.pending = map[uint64]func(json.RawMessage, error){}
		}
		w.eval.pending[id] = onResult
	}

	if w.evalHook != nil {
		w.evalHook(js, id, true)
		return
	}
	w.evalResultRaw(js, id)
}

// EvalFunc runs body as the body of a JavaScript function whose parameters are
// named by args, discarding the result.
//
// The values in args are marshalled to JSON and passed to that function as
// data, never spliced into the source, so content from outside the app cannot
// become executable code in the page:
//
//	view.EvalFunc("document.title = title;", webview.Args{"title": userInput})
//
// This is the injection-safe form of [WebView.Eval] and needs no cooperation
// from the page, unlike [WebView.Call].
//
// Since: 1.0
func (w *WebView) EvalFunc(body string, args Args) {
	js, err := scriptFunc(body, args)
	if err != nil {
		bellui.LogError("webview: EvalFunc", err)
		return
	}
	w.Eval(js)
}

// EvalFuncResult is [WebView.EvalFunc] reporting the function's return value,
// with the result and error contract of [WebView.EvalResult].
//
// Since: 1.0
func (w *WebView) EvalFuncResult(body string, args Args, onResult func(json.RawMessage, error)) {
	js, err := scriptFunc(body, args)
	if err != nil {
		reportResult(onResult, err)
		return
	}
	w.EvalResult(js, onResult)
}

// deliverEvalResult is the backend entry point for a finished eval. An empty
// errMsg means success, and result is the JSON encoding of the script's value.
func (w *WebView) deliverEvalResult(id uint64, result, errMsg string) {
	cb := w.eval.pending[id]
	if cb == nil {
		return // no result was asked for, or the view was destroyed meanwhile
	}
	delete(w.eval.pending, id)
	if errMsg != "" {
		cb(json.RawMessage("null"), errors.New(errMsg))
		return
	}
	cb(payloadOrNull(json.RawMessage(result)), nil)
}

func (w *WebView) failPendingEvals(err error) {
	pending := w.eval.pending
	w.eval.pending = nil
	for _, cb := range pending {
		cb(json.RawMessage("null"), err)
	}
}

// scriptFunc wraps body in an immediately invoked function whose parameters
// are the names in args, applied to their JSON-encoded values. Names are
// sorted so the same call always produces the same script, which keeps the
// output testable.
func scriptFunc(body string, args Args) (string, error) {
	names := make([]string, 0, len(args))
	for name := range args {
		if !isJSIdentifier(name) {
			return "", fmt.Errorf("webview: %q is not a JavaScript identifier", name)
		}
		names = append(names, name)
	}
	slices.Sort(names)

	values := make([]string, 0, len(names))
	for _, name := range names {
		data, err := jsonOrNull(args[name])
		if err != nil {
			return "", fmt.Errorf("webview: argument %q: %w", name, err)
		}
		values = append(values, string(data))
	}
	// encoding/json escapes <, > and U+2028/U+2029, so each value is a literal
	// that is safe in JavaScript source however it was produced.
	return "(function(" + strings.Join(names, ",") + "){" + body + "\n})(" +
		strings.Join(values, ",") + ")", nil
}

// isJSIdentifier reports whether name can be a function parameter. It is
// deliberately stricter than the language: ASCII letters, digits, _ and $,
// not starting with a digit, and not a reserved word we would silently break
// on. Anything rejected here would have to be quoted into the source, which
// is what this whole path exists to avoid.
func isJSIdentifier(name string) bool {
	if name == "" || jsReserved[name] {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_' || r == '$':
		case unicode.IsLetter(r) && r < unicode.MaxASCII:
		case unicode.IsDigit(r) && r < unicode.MaxASCII && i > 0:
		default:
			return false
		}
	}
	return true
}

var jsReserved = map[string]bool{
	"arguments": true, "await": true, "break": true, "case": true, "catch": true,
	"class": true, "const": true, "continue": true, "debugger": true, "default": true,
	"delete": true, "do": true, "else": true, "enum": true, "eval": true,
	"export": true, "extends": true, "false": true, "finally": true, "for": true,
	"function": true, "if": true, "import": true, "in": true, "instanceof": true,
	"let": true, "new": true, "null": true, "return": true, "static": true,
	"super": true, "switch": true, "this": true, "throw": true, "true": true,
	"try": true, "typeof": true, "var": true, "void": true, "while": true,
	"with": true, "yield": true,
}
