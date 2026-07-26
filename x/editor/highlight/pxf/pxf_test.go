package pxf

import (
	"testing"

	"github.com/belluikit/bellui/x/editor"
)

func TestHighlightHashComment(t *testing.T) {
	h := New()
	runs, state := h.Highlight("# this is a comment", 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("color = %q, want comment", runs[0].ColorName)
	}
	if !runs[0].Italic {
		t.Error("comment should be italic")
	}
}

func TestHighlightSlashComment(t *testing.T) {
	h := New()
	runs, state := h.Highlight("x = 1 // inline", 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	last := runs[len(runs)-1]
	if last.Text != "// inline" {
		t.Errorf("comment text = %q, want %q", last.Text, "// inline")
	}
	if last.ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("color = %q, want comment", last.ColorName)
	}
}

func TestHighlightBlockCommentSingleLine(t *testing.T) {
	h := New()
	runs, state := h.Highlight("x = /* comment */ 1", 0)

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
		t.Errorf("comment text = %q", commentRun.Text)
	}
}

func TestHighlightBlockCommentMultiLine(t *testing.T) {
	h := New()

	// Open block comment.
	runs1, state1 := h.Highlight("x = /* start", 0)
	if state1 != stateBlockComment {
		t.Errorf("state after open = %d, want %d", state1, stateBlockComment)
	}
	last := runs1[len(runs1)-1]
	if last.ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("open comment color = %q, want comment", last.ColorName)
	}

	// Middle line — entirely inside comment.
	runs2, state2 := h.Highlight("still commenting", state1)
	if state2 != stateBlockComment {
		t.Errorf("state in middle = %d, want %d", state2, stateBlockComment)
	}
	if len(runs2) != 1 || runs2[0].ColorName != editor.ColorNameSyntaxComment {
		t.Error("middle line should be entirely comment")
	}

	// Close block comment.
	runs3, state3 := h.Highlight("end */ y = 2", state2)
	if state3 != stateNormal {
		t.Errorf("state after close = %d, want 0", state3)
	}
	if runs3[0].ColorName != editor.ColorNameSyntaxComment {
		t.Errorf("closing comment color = %q, want comment", runs3[0].ColorName)
	}
}

func TestHighlightString(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`name = "hello world"`, 0)

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
	if strRun.Text != `"hello world"` {
		t.Errorf("string text = %q", strRun.Text)
	}
}

func TestHighlightStringWithEscape(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`x = "line1\nline2"`, 0)

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
	if strRun.Text != `"line1\nline2"` {
		t.Errorf("string text = %q", strRun.Text)
	}
}

func TestHighlightTripleQuotedString(t *testing.T) {
	h := New()

	// Open triple-quoted string.
	runs1, state1 := h.Highlight(`x = """start`, 0)
	if state1 != stateTripleString {
		t.Errorf("state = %d, want %d", state1, stateTripleString)
	}
	last := runs1[len(runs1)-1]
	if last.ColorName != editor.ColorNameSyntaxString {
		t.Errorf("triple-quote color = %q, want string", last.ColorName)
	}

	// Middle continuation.
	runs2, state2 := h.Highlight("middle line", state1)
	if state2 != stateTripleString {
		t.Errorf("state in middle = %d, want %d", state2, stateTripleString)
	}
	if runs2[0].ColorName != editor.ColorNameSyntaxString {
		t.Errorf("middle color = %q, want string", runs2[0].ColorName)
	}

	// Closing line.
	runs3, state3 := h.Highlight(`end""" done`, state2)
	if state3 != stateNormal {
		t.Errorf("state after close = %d, want 0", state3)
	}
	if runs3[0].ColorName != editor.ColorNameSyntaxString {
		t.Errorf("close color = %q, want string", runs3[0].ColorName)
	}
}

func TestHighlightTripleQuotedSingleLine(t *testing.T) {
	h := New()
	runs, state := h.Highlight(`x = """inline"""`, 0)

	if state != stateNormal {
		t.Errorf("state = %d, want 0", state)
	}
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
	if strRun.Text != `"""inline"""` {
		t.Errorf("string text = %q", strRun.Text)
	}
}

func TestHighlightBytesLiteral(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`data = b"SGVsbG8="`, 0)

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
	if strRun.Text != `b"SGVsbG8="` {
		t.Errorf("bytes text = %q", strRun.Text)
	}
}

func TestHighlightBooleans(t *testing.T) {
	h := New()
	for _, val := range []string{"true", "false"} {
		runs, _ := h.Highlight("flag = "+val, 0)

		var found bool
		for _, r := range runs {
			if r.Text == val {
				if r.ColorName != editor.ColorNameSyntaxKeyword {
					t.Errorf("%s color = %q, want keyword", val, r.ColorName)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no run found for %s", val)
		}
	}
}

func TestHighlightNumber(t *testing.T) {
	h := New()
	for _, tc := range []struct {
		line string
		want string
	}{
		{"x = 42", "42"},
		{"x = 3.14", "3.14"},
		{"x = 0xFF", "0xFF"},
	} {
		runs, _ := h.Highlight(tc.line, 0)

		var numRun *editor.StyleRun
		for i := range runs {
			if runs[i].ColorName == editor.ColorNameSyntaxNumber {
				numRun = &runs[i]
				break
			}
		}
		if numRun == nil {
			t.Fatalf("no number run found for %q", tc.line)
		}
		if numRun.Text != tc.want {
			t.Errorf("number text = %q, want %q", numRun.Text, tc.want)
		}
	}
}

func TestHighlightTimestamp(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("ts = 2024-01-15T10:30:00Z", 0)

	var tsRun *editor.StyleRun
	for i := range runs {
		if runs[i].ColorName == editor.ColorNameSyntaxNumber {
			tsRun = &runs[i]
			break
		}
	}
	if tsRun == nil {
		t.Fatal("no timestamp run found")
	}
	if tsRun.Text != "2024-01-15T10:30:00Z" {
		t.Errorf("timestamp text = %q", tsRun.Text)
	}
}

func TestHighlightDuration(t *testing.T) {
	h := New()
	for _, tc := range []struct {
		line string
		want string
	}{
		{"d = 30s", "30s"},
		{"d = 1h30m", "1h30m"},
		{"d = 500ms", "500ms"},
	} {
		runs, _ := h.Highlight(tc.line, 0)

		var durRun *editor.StyleRun
		for i := range runs {
			if runs[i].Text == tc.want {
				durRun = &runs[i]
				break
			}
		}
		if durRun == nil {
			t.Fatalf("no duration run for %q", tc.line)
		}
		if durRun.ColorName != editor.ColorNameSyntaxNumber {
			t.Errorf("duration color = %q, want number", durRun.ColorName)
		}
	}
}

func TestHighlightDirective(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("@type test.v1.Message", 0)

	if runs[0].Text != "@type" {
		t.Errorf("directive text = %q, want @type", runs[0].Text)
	}
	if runs[0].ColorName != editor.ColorNameSyntaxKeyword {
		t.Errorf("directive color = %q, want keyword", runs[0].ColorName)
	}
}

func TestHighlightEnum(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("status = STATUS_ACTIVE", 0)

	var enumRun *editor.StyleRun
	for i := range runs {
		if runs[i].Text == "STATUS_ACTIVE" {
			enumRun = &runs[i]
			break
		}
	}
	if enumRun == nil {
		t.Fatal("no enum run found")
	}
	if enumRun.ColorName != editor.ColorNameSyntaxBuiltin {
		t.Errorf("enum color = %q, want builtin", enumRun.ColorName)
	}
}

func TestHighlightOperators(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("key: value", 0)

	var opRun *editor.StyleRun
	for i := range runs {
		if runs[i].Text == ":" {
			opRun = &runs[i]
			break
		}
	}
	if opRun == nil {
		t.Fatal("no operator run for ':'")
	}
	if opRun.ColorName != editor.ColorNameSyntaxOperator {
		t.Errorf("operator color = %q, want operator", opRun.ColorName)
	}
}

func TestHighlightFieldName(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("field_name = 1", 0)

	if runs[0].Text != "field_name" {
		t.Errorf("first run = %q, want field_name", runs[0].Text)
	}
	// Field names use the default foreground color.
	if runs[0].ColorName == editor.ColorNameSyntaxKeyword ||
		runs[0].ColorName == editor.ColorNameSyntaxBuiltin {
		t.Errorf("field name should not be keyword/builtin, got %q", runs[0].ColorName)
	}
}

func TestHighlightEmptyLine(t *testing.T) {
	h := New()
	runs, state := h.Highlight("", 0)

	if state != 0 {
		t.Errorf("state = %d, want 0", state)
	}
	if len(runs) != 0 {
		t.Errorf("len(runs) = %d, want 0 for empty line", len(runs))
	}
}

func TestHighlightExampleFile(t *testing.T) {
	// Run all lines of a representative PXF document through the highlighter
	// to verify no panics and state consistency.
	lines := []string{
		`@type test.v1.AllTypes`,
		``,
		`# Scalar fields`,
		`string_field = "hello world"`,
		`int32_field = 42`,
		`float_field = 3.14`,
		`bool_field = true`,
		`bytes_field = b"SGVsbG8="`,
		``,
		`enum_field = STATUS_ACTIVE`,
		``,
		`nested_field {`,
		`  name = "inner"`,
		`  value = 99`,
		`}`,
		``,
		`repeated_string = ["alpha", "beta"]`,
		``,
		`ts_field = 2024-01-15T10:30:00Z`,
		`dur_field = 1h30m`,
	}

	h := New()
	state := 0
	for i, line := range lines {
		runs, newState := h.Highlight(line, state)
		// Verify runs cover the full line text.
		var combined string
		for _, r := range runs {
			combined += r.Text
		}
		if combined != line {
			t.Errorf("line %d: runs text = %q, want %q", i, combined, line)
		}
		state = newState
	}
	if state != 0 {
		t.Errorf("final state = %d, want 0", state)
	}
}

func TestHighlightNegativeNumber(t *testing.T) {
	h := New()
	runs, _ := h.Highlight("x = -42", 0)

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
	if numRun.Text != "-42" {
		t.Errorf("number text = %q, want %q", numRun.Text, "-42")
	}
}

func TestHighlightMapSyntax(t *testing.T) {
	h := New()
	runs, _ := h.Highlight(`  env: "production"`, 0)

	// Should contain an operator for ":"
	var foundColon bool
	for _, r := range runs {
		if r.Text == ":" && r.ColorName == editor.ColorNameSyntaxOperator {
			foundColon = true
		}
	}
	if !foundColon {
		t.Error("expected ':' operator in map syntax")
	}
}

func TestRunsCoverFullLine(t *testing.T) {
	h := New()
	lines := []string{
		`@type test.v1.Msg`,
		`field = "value" # comment`,
		`nested {`,
		`  x = 42`,
		`}`,
		`list = [1, 2, 3]`,
		`  key: "val"`,
	}
	for _, line := range lines {
		runs, _ := h.Highlight(line, 0)
		var combined string
		for _, r := range runs {
			combined += r.Text
		}
		if combined != line {
			t.Errorf("line %q: runs = %q", line, combined)
		}
	}
}
