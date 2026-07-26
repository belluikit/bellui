package golang

import (
	"testing"

	"github.com/belluikit/bellui/x/editor"
)

func TestHighlightKeyword(t *testing.T) {
	h := New()
	runs, state := h.Highlight("func main() {", 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	if len(runs) == 0 {
		t.Fatal("expected runs")
	}
	if runs[0].Text != "func" {
		t.Errorf("first run text = %q, want 'func'", runs[0].Text)
	}
	if runs[0].ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("first run color = %q, want keyword", runs[0].ColorName)
	}
}

func TestHighlightAllKeywords(t *testing.T) {
	h := New()
	for _, kw := range []string{
		"break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var",
	} {
		runs, _ := h.Highlight(kw, 0)
		if runs[0].ColorName != editor.ColorNameSyntaxKeyword {
			t.Errorf("%q: color = %q, want keyword", kw, runs[0].ColorName)
		}
	}
}

func TestHighlightString(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`x := "hello"`, 0)

	var strRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxString {
			strRun = &runs[i]
			break
		}
	}
	if strRun == nil {
		t.Fatal("no string run found")
	}
	if strRun.Text != `"hello"` {
		t.Errorf("string run text = %q, want '\"hello\"'", strRun.Text)
	}
}

func TestHighlightStringEscape(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`x := "he\"llo"`, 0)

	var strRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxString {
			strRun = &runs[i]
			break
		}
	}
	if strRun == nil {
		t.Fatal("no string run found")
	}
	if strRun.Text != `"he\"llo"` {
		t.Errorf("string run text = %q", strRun.Text)
	}
}

func TestHighlightRawString(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := `hello`", 0)

	var strRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxString {
			strRun = &runs[i]
			break
		}
	}
	if strRun == nil {
		t.Fatal("no raw string run found")
	}
	if strRun.Text != "`hello`" {
		t.Errorf("raw string text = %q", strRun.Text)
	}
}

func TestHighlightRawStringMultiLine(t *testing.T) {
	h := New()

	// Open raw string — state should carry forward.
	runs1, state1 := h.Highlight("x := `start", 0)
	if state1 != stateRawString {
		t.Errorf("state after open raw = %d, want %d", state1, stateRawString)
	}
	last := runs1[len(runs1)-1]
	if last.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("raw string run color = %q, want string", last.ColorName)
	}

	// Continuation line — still in raw string.
	runs2, state2 := h.Highlight("middle", state1)
	if state2 != stateRawString {
		t.Errorf("state during raw = %d, want %d", state2, stateRawString)
	}
	if runs2[0].ColorName != editor.ColorNameSyntaxString {
		t.Errorf("continuation run color = %q, want string", runs2[0].ColorName)
	}

	// Closing line.
	runs3, state3 := h.Highlight("end` + x", state2)
	if state3 != stateNormal {
		t.Errorf("state after close raw = %d, want 0", state3)
	}
	if runs3[0].ColorName != editor.ColorNameSyntaxString {
		t.Errorf("close raw string run color = %q, want string", runs3[0].ColorName)
	}
}

func TestHighlightRuneLiteral(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 'a'", 0)

	var strRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxString {
			strRun = &runs[i]
			break
		}
	}
	if strRun == nil {
		t.Fatal("no rune literal found")
	}
	if strRun.Text != "'a'" {
		t.Errorf("rune literal text = %q", strRun.Text)
	}
}

func TestHighlightLineComment(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 1 // comment", 0)

	last := runs[len(runs)-1]
	if last.ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("last run color = %q, want comment", last.ColorName)
	}
	if last.Text != "// comment" {
		t.Errorf("comment text = %q", last.Text)
	}
	if !last.Italic {
		t.Error("comment should be italic")
	}
}

func TestHighlightBlockComment(t *testing.T) {
	h := New()
	runs, state := h.Highlight("x /* comment */ y", 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	var commentRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxComment {
			commentRun = &runs[i]
			break
		}
	}
	if commentRun == nil {
		t.Fatal("no comment run found")
	}
	if commentRun.Text != "/* comment */" {
		t.Errorf("block comment text = %q", commentRun.Text)
	}
}

func TestHighlightBlockCommentMultiLine(t *testing.T) {
	h := New()

	// Open block comment.
	runs1, state1 := h.Highlight("x /* start", 0)
	if state1 != stateBlockComment {
		t.Errorf("state after open = %d, want %d", state1, stateBlockComment)
	}
	last := runs1[len(runs1)-1]
	if last.ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("open comment color = %q, want comment", last.ColorName)
	}

	// Continuation.
	runs2, state2 := h.Highlight("middle", state1)
	if state2 != stateBlockComment {
		t.Errorf("state during = %d, want %d", state2, stateBlockComment)
	}
	if runs2[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("continuation color = %q, want comment", runs2[0].ColorName)
	}

	// Closing line.
	runs3, state3 := h.Highlight("end */ y", state2)
	if state3 != stateNormal {
		t.Errorf("state after close = %d, want 0", state3)
	}
	if runs3[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("close comment color = %q, want comment", runs3[0].ColorName)
	}
}

func TestHighlightNumber(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 42", 0)

	var numRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			numRun = &runs[i]
			break
		}
	}
	if numRun == nil {
		t.Fatal("no number run found")
	}
	if numRun.Text != "42" {
		t.Errorf("number text = %q, want '42'", numRun.Text)
	}
}

func TestHighlightHexNumber(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 0xFF", 0)

	var numRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			numRun = &runs[i]
			break
		}
	}
	if numRun == nil {
		t.Fatal("no hex number run found")
	}
	if numRun.Text != "0xFF" {
		t.Errorf("hex number text = %q", numRun.Text)
	}
}

func TestHighlightFloat(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 3.14", 0)

	var numRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			numRun = &runs[i]
			break
		}
	}
	if numRun == nil {
		t.Fatal("no float run found")
	}
	if numRun.Text != "3.14" {
		t.Errorf("float text = %q", numRun.Text)
	}
}

func TestHighlightImaginary(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 2i", 0)

	var numRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			numRun = &runs[i]
			break
		}
	}
	if numRun == nil {
		t.Fatal("no imaginary number run found")
	}
	if numRun.Text != "2i" {
		t.Errorf("imaginary text = %q", numRun.Text)
	}
}

func TestHighlightBuiltinType(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("var x int", 0)

	var typeRun *editor.StyleRun
	for i := range runs {
		if runs[i].Text == "int" {
			typeRun = &runs[i]
			break
		}
	}
	if typeRun == nil {
		t.Fatal("no 'int' run found")
	}
	if typeRun.ColorName != editor.ColorNameSyntaxType {
		t.Errorf("type color = %q, want type", typeRun.ColorName)
	}
}

func TestHighlightBuiltinFunc(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("n := len(x)", 0)

	var builtinRun *editor.StyleRun
	for i := range runs {
		if runs[i].Text == "len" {
			builtinRun = &runs[i]
			break
		}
	}
	if builtinRun == nil {
		t.Fatal("no 'len' run found")
	}
	if builtinRun.ColorName != editor.ColorNameSyntaxBuiltin {
		t.Errorf("builtin color = %q, want builtin", builtinRun.ColorName)
	}
}

func TestHighlightPredeclaredConst(t *testing.T) {
	h := New()
	for _, c := range []string{"true", "false", "nil", "iota"} {
		runs, _ := h.Highlight(c, 0)
		if runs[0].ColorName != editor.ColorNameSyntaxBuiltin {
			t.Errorf("%q: color = %q, want builtin", c, runs[0].ColorName)
		}
	}
}

func TestHighlightFunctionCall(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("foo()", 0)

	if runs[0].Text != "foo" {
		t.Errorf("first run = %q, want 'foo'", runs[0].Text)
	}
	if runs[0].ColorName != editor.ColorNameSyntaxFunction {
		t.Errorf("function color = %q, want function", runs[0].ColorName)
	}
}

func TestHighlightOperator(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x := 1", 0)

	var opRun *editor.StyleRun
	for i := range runs {
		if runs[i].Text == ":=" {
			opRun = &runs[i]
			break
		}
	}
	if opRun == nil {
		t.Fatal("no ':=' run found")
	}
	if opRun.ColorName != editor.ColorNameSyntaxOperator {
		t.Errorf("operator color = %q, want operator", opRun.ColorName)
	}
}

func TestHighlightPackageClause(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("package main", 0)

	if runs[0].Text != "package" || runs[0].ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("package keyword: text=%q color=%q", runs[0].Text, runs[0].ColorName)
	}
}

func TestHighlightOctalAndBinary(t *testing.T) {
	h := New()

	runs, _ := h.Highlight("x := 0o77", 0)
	var numRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			numRun = &runs[i]
			break
		}
	}
	if numRun == nil || numRun.Text != "0o77" {
		t.Errorf("octal: got %v", numRun)
	}

	runs, _ = h.Highlight("x := 0b1010", 0)
	numRun = nil
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			numRun = &runs[i]
			break
		}
	}
	if numRun == nil || numRun.Text != "0b1010" {
		t.Errorf("binary: got %v", numRun)
	}
}
