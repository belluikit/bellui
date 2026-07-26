package proto3

import (
	"testing"

	"github.com/belluikit/bellui/x/editor"
)

func runFor(runs []editor.StyleRun, text string) *editor.StyleRun {
	for i := range runs {
		if runs[i].Text == text {
			return &runs[i]
		}
	}
	return nil
}

func TestHighlightKeyword(t *testing.T) {
	h := New()
	runs, state := h.Highlight(`syntax = "proto3";`, 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	if len(runs) == 0 {
		t.Fatal("expected runs")
	}
	if runs[0].Text != "syntax" {
		t.Errorf("first run text = %q, want 'syntax'", runs[0].Text)
	}
	if runs[0].ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("first run color = %q, want keyword", runs[0].ColorName)
	}
}

func TestHighlightAllKeywords(t *testing.T) {
	h := New()
	for _, kw := range []string{
		"syntax", "edition", "package", "import", "weak", "public",
		"option", "message", "enum", "oneof", "service", "rpc", "returns",
		"stream", "repeated", "optional", "required", "map", "reserved",
		"extensions", "extend", "to", "group",
	} {
		runs, _ := h.Highlight(kw, 0)
		if runs[0].ColorName != editor.ColorNameSyntaxKeyword {
			t.Errorf("%q: color = %q, want keyword", kw, runs[0].ColorName)
		}
	}
}

func TestHighlightScalarTypes(t *testing.T) {
	h := New()
	for _, ty := range []string{
		"double", "float", "int32", "int64", "uint32", "uint64",
		"sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64",
		"bool", "string", "bytes",
	} {
		runs, _ := h.Highlight(ty+" name = 1;", 0)
		if runs[0].Text != ty {
			t.Fatalf("%q: first run text = %q", ty, runs[0].Text)
		}
		if runs[0].ColorName != editor.ColorNameSyntaxType {
			t.Errorf("%q: color = %q, want type", ty, runs[0].ColorName)
		}
	}
}

func TestHighlightFieldLine(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("  repeated string names = 12;", 0)

	if r := runFor(runs, "repeated"); r == nil || r.ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("repeated: %+v, want keyword", r)
	}
	if r := runFor(runs, "string"); r == nil || r.ColorName != editor.ColorNameSyntaxType {
		t.Errorf("string: %+v, want type", r)
	}
	if r := runFor(runs, "12"); r == nil || r.ColorName != editor.ColorNameSyntaxNumber {
		t.Errorf("12: %+v, want number", r)
	}
	if r := runFor(runs, "="); r == nil || r.ColorName != editor.ColorNameSyntaxOperator {
		t.Errorf("=: %+v, want operator", r)
	}
}

func TestHighlightDeclaredTypeName(t *testing.T) {
	h := New()
	for _, line := range []string{
		"message GetUserRequest {",
		"enum GetUserRequest {",
		"service GetUserRequest {",
		"oneof GetUserRequest {",
		"extend GetUserRequest {",
	} {
		runs, _ := h.Highlight(line, 0)
		if r := runFor(runs, "GetUserRequest"); r == nil || r.ColorName != editor.ColorNameSyntaxType {
			t.Errorf("%q: declared name %+v, want type", line, r)
		}
	}
}

func TestHighlightRPCMethodName(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("  rpc GetUser(GetUserRequest) returns (GetUserResponse);", 0)

	if r := runFor(runs, "rpc"); r == nil || r.ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("rpc: %+v, want keyword", r)
	}
	if r := runFor(runs, "GetUser"); r == nil || r.ColorName != editor.ColorNameSyntaxFunction {
		t.Errorf("GetUser: %+v, want function", r)
	}
	if r := runFor(runs, "returns"); r == nil || r.ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("returns: %+v, want keyword", r)
	}
}

func TestHighlightString(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`import "google/protobuf/timestamp.proto";`, 0)

	if r := runFor(runs, `"google/protobuf/timestamp.proto"`); r == nil || r.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("import path: %+v, want string", r)
	}
}

func TestHighlightSingleQuoteString(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`option java_package = 'com.example';`, 0)

	if r := runFor(runs, `'com.example'`); r == nil || r.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("single-quoted: %+v, want string", r)
	}
}

func TestHighlightStringEscape(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`option x = "he\"llo";`, 0)

	if r := runFor(runs, `"he\"llo"`); r == nil || r.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("escaped string: %+v, want string", r)
	}
}

func TestHighlightLineComment(t *testing.T) {
	h := New()
	runs, state := h.Highlight("int32 id = 1; // primary key", 0)

	if state != stateNormal {
		t.Errorf("state = %d, want normal", state)
	}
	last := runs[len(runs)-1]
	if last.Text != "// primary key" {
		t.Errorf("comment text = %q", last.Text)
	}
	if last.ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("comment color = %q", last.ColorName)
	}
	if !last.Italic {
		t.Error("comment should be italic")
	}
}

func TestHighlightBlockCommentMultiLine(t *testing.T) {
	h := New()

	runs1, state1 := h.Highlight("/* deprecated:", 0)
	if state1 != stateBlockComment {
		t.Errorf("state after open = %d, want %d", state1, stateBlockComment)
	}
	if runs1[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("open run color = %q, want comment", runs1[0].ColorName)
	}

	runs2, state2 := h.Highlight("use v2 instead", state1)
	if state2 != stateBlockComment {
		t.Errorf("state during = %d, want %d", state2, stateBlockComment)
	}
	if runs2[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("continuation color = %q, want comment", runs2[0].ColorName)
	}

	runs3, state3 := h.Highlight("*/ message Foo {", state2)
	if state3 != stateNormal {
		t.Errorf("state after close = %d, want normal", state3)
	}
	if runs3[0].Text != "*/" || runs3[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("close run = %+v, want comment '*/'", runs3[0])
	}
	if r := runFor(runs3, "message"); r == nil || r.ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("message after close: %+v, want keyword", r)
	}
	if r := runFor(runs3, "Foo"); r == nil || r.ColorName != editor.ColorNameSyntaxType {
		t.Errorf("Foo after close: %+v, want type", r)
	}
}

func TestHighlightNumbers(t *testing.T) {
	h := New()
	for _, tc := range []struct{ line, num string }{
		{"reserved 2, 15, 9 to 11;", "15"},
		{"option x = 0xFF;", "0xFF"},
		{"option y = 3.14;", "3.14"},
		{"option z = 1e10;", "1e10"},
		{"option w = 2.5e-3;", "2.5e-3"},
	} {
		runs, _ := h.Highlight(tc.line, 0)
		if r := runFor(runs, tc.num); r == nil || r.ColorName != editor.ColorNameSyntaxNumber {
			t.Errorf("%q: %q run %+v, want number", tc.line, tc.num, r)
		}
	}
}

func TestHighlightConstants(t *testing.T) {
	h := New()
	for _, tc := range []struct{ line, word string }{
		{"option deprecated = true;", "true"},
		{"option deprecated = false;", "false"},
		{"extensions 100 to max;", "max"},
	} {
		runs, _ := h.Highlight(tc.line, 0)
		if r := runFor(runs, tc.word); r == nil || r.ColorName != editor.ColorNameSyntaxBuiltin {
			t.Errorf("%q: %q run %+v, want builtin", tc.line, tc.word, r)
		}
	}
}

func TestHighlightMapField(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("map<string, Project> projects = 3;", 0)

	if r := runFor(runs, "map"); r == nil || r.ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("map: %+v, want keyword", r)
	}
	if r := runFor(runs, "string"); r == nil || r.ColorName != editor.ColorNameSyntaxType {
		t.Errorf("string: %+v, want type", r)
	}
}

func TestHighlightUnterminatedStringStaysOnLine(t *testing.T) {
	h := New()
	runs, state := h.Highlight(`option x = "unterminated`, 0)

	if state != stateNormal {
		t.Errorf("state = %d, want normal (proto strings never span lines)", state)
	}
	last := runs[len(runs)-1]
	if last.Text != `"unterminated` || last.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("last run = %+v, want string run for the rest of the line", last)
	}
}

func TestRoundTripPreservesText(t *testing.T) {
	h := New()
	lines := []string{
		`syntax = "proto3";`,
		"package acme.demo.v1;",
		`import "google/protobuf/timestamp.proto";`,
		"",
		"// A user record.",
		"message User {",
		"  int32 id = 1; /* primary",
		"     key */ string name = 2;",
		"  map<string, bytes> blobs = 3 [deprecated = true];",
		"}",
	}
	state := 0
	for _, line := range lines {
		var runs []editor.StyleRun
		runs, state = h.Highlight(line, state)
		var got string
		for _, r := range runs {
			got += r.Text
		}
		if got != line {
			t.Errorf("round-trip: got %q, want %q", got, line)
		}
	}
	if state != stateNormal {
		t.Errorf("final state = %d, want normal", state)
	}
}
