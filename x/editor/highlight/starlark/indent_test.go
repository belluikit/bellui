package starlark

import (
	"testing"

	"github.com/belluikit/bellui/x/editor"
)

func TestReindentOnType(t *testing.T) {
	p := IndentProvider().(editor.ReindentProvider)
	unit := "    "

	tests := []struct {
		name       string
		line       string
		prevIndent string
		typed      rune
		want       string
		ok         bool
	}{
		{"else clause dedents", "        else:", "        ", ':', "    ", true},
		{"elif with condition", "        elif x > 1:", "        ", ':', "    ", true},
		{"except with type", "        except ValueError:", "        ", ':', "    ", true},
		{"finally clause", "    finally:", "    ", ':', "", true},
		{"tab indent drops a tab", "\t\telse:", "\t\t", ':', "\t", true},
		{"non-clause colon", "        d = {1:", "        ", ':', "", false},
		{"identifier prefix not keyword", "        elsewhere:", "        ", ':', "", false},
		{"non-colon rune", "        else", "        ", 'e', "", false},
		{"flush-left clause has nothing to give", "else:", "", ':', "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := p.ReindentOnType(editor.IndentContext{
				Line:       tc.line,
				PrevIndent: tc.prevIndent,
				Unit:       unit,
			}, tc.typed)
			if ok != tc.ok || (ok && got != tc.want) {
				t.Errorf("ReindentOnType(%q, %q) = (%q, %v), want (%q, %v)",
					tc.line, tc.typed, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestIndentProvider(t *testing.T) {
	p := IndentProvider()
	unit := "    " // 4 spaces, a typical editor indent unit

	tests := []struct {
		name       string
		line       string
		prevIndent string
		want       string
	}{
		{"colon opens a block", "def foo():", "", unit},
		{"nested colon adds a level", "    if x:", "    ", "    " + unit},
		{"trailing spaces before eol", "for i in xs:   ", "", unit},
		{"plain statement inherits", "    x = 1", "    ", "    "},
		{"assignment ending in bracket", "d = {}", "", ""},
		{"top-level plain line", "print(x)", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.IndentForNewLine(editor.IndentContext{
				Line:       tc.line,
				PrevIndent: tc.prevIndent,
				Unit:       unit,
			})
			if got != tc.want {
				t.Errorf("IndentForNewLine(%q, prev=%q) = %q, want %q", tc.line, tc.prevIndent, got, tc.want)
			}
		})
	}
}
