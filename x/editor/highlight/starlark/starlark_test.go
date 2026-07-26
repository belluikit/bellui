package starlark

import (
	"testing"

	"github.com/belluikit/bellui/x/editor"
)

func TestHighlightKeyword(t *testing.T) {
	h := New()
	runs, state := h.Highlight("def foo():", 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	if len(runs) == 0 {
		t.Fatal("expected runs")
	}
	if runs[0].Text != "def" {
		t.Errorf("first run text = %q, want 'def'", runs[0].Text)
	}
	if runs[0].ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("first run color = %q, want keyword", runs[0].ColorName)
	}
}

func TestHighlightString(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`x = "hello"`, 0)

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

func TestHighlightComment(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x = 1 # comment", 0)

	last := runs[len(runs)-1]
	if last.ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("last run color = %q, want comment", last.ColorName)
	}
	if last.Text != "# comment" {
		t.Errorf("comment text = %q", last.Text)
	}
	if !last.Italic {
		t.Error("comment should be italic")
	}
}

func TestHighlightTripleQuoteMultiLine(t *testing.T) {
	h := New()

	// Open triple-quote — state should carry forward.
	runs1, state1 := h.Highlight(`x = """start`, 0)
	if state1 != stateInDoubleQuote {
		t.Errorf("state after open triple = %d, want %d", state1, stateInDoubleQuote)
	}
	// Whole tail should be string.
	last := runs1[len(runs1)-1]
	if last.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("triple-quote run color = %q, want string", last.ColorName)
	}

	// Continuation line — still in string.
	runs2, state2 := h.Highlight(`middle""" + done`, state1)
	if state2 != 0 {
		t.Errorf("state after close triple = %d, want 0", state2)
	}
	if runs2[0].ColorName != editor.ColorNameSyntaxString {
		t.Errorf("continuation run color = %q, want string", runs2[0].ColorName)
	}
}

func TestHighlightNumber(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x = 42", 0)

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

func TestHighlightBuiltin(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("n = len(x)", 0)

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

func TestAddBuiltins(t *testing.T) {
	h := New()
	// Before adding, calc_kelly is a function call (not builtin).
	runs, _ := h.Highlight("calc_kelly(x)", 0)
	if runs[0].ColorName == editor.ColorNameSyntaxBuiltin {
		t.Error("calc_kelly should not be builtin before AddBuiltins")
	}

	h.AddBuiltins("calc_kelly")
	runs, _ = h.Highlight("calc_kelly(x)", 0)
	if runs[0].ColorName != editor.ColorNameSyntaxBuiltin {
		t.Errorf("after AddBuiltins, calc_kelly color = %q, want builtin", runs[0].ColorName)
	}
}

func TestHighlightLogicalOps(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("a and b or not c", 0)

	ops := map[string]bool{}
	for _, r := range runs {
		if r.ColorName == editor.ColorNameSyntaxOperator {
			ops[r.Text] = true
		}
	}
	for _, word := range []string{"and", "or", "not"} {
		if !ops[word] {
			t.Errorf("%q should be colored as operator", word)
		}
	}
}
