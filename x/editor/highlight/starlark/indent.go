package starlark

import (
	"strings"

	"github.com/belluikit/bellui/x/editor"
)

// indentProvider implements [editor.IndentProvider] with Starlark/Python indent
// rules.
type indentProvider struct{}

// IndentProvider returns a [editor.IndentProvider] with Starlark/Python
// auto-indent rules: the new line inherits the current line's indentation, plus
// one extra level after a line whose code ends with a colon (the `def`/`if`/
// `for`/`while`/`with`/… block openers).
//
// It reads the editor's indent unit (spaces vs. tab and width) from
// [editor.IndentContext.Unit], so Enter-indent agrees with Tab-indent. It is
// the auto-indent counterpart to [New] — a language template a host can install
// directly (alongside the highlighter) or model its own rule on.
func IndentProvider() editor.IndentProvider {
	return indentProvider{}
}

// IndentForNewLine inherits the current line's indentation and adds one level
// after a block-opening colon.
func (indentProvider) IndentForNewLine(ctx editor.IndentContext) string {
	if opensBlock(ctx.Line) {
		return ctx.PrevIndent + ctx.Unit
	}
	return ctx.PrevIndent
}

// opensBlock reports whether line ends with a block-opening colon. It trims
// trailing whitespace and checks for a final ':'. This is the language-template
// heuristic: it catches the common openers (`def f():`, `if x:`, `for i in xs:`)
// and, kept deliberately simple, does not special-case a trailing comment after
// the colon or a colon inside brackets/slices — a host wanting richer behavior
// supplies its own IndentProvider.
func opensBlock(line string) bool {
	return strings.HasSuffix(strings.TrimRight(line, " \t"), ":")
}

// ReindentOnType implements the optional [editor.ReindentProvider] extension:
// typing the ':' that completes a dedent clause (`else:`, `elif …:`, `except…:`,
// `finally:`) dedents that line one level, since the clause belongs to the
// enclosing block, not the suite above it. Like [opensBlock] this is the simple
// language-template rule: it dedents one Unit relative to the line's current
// indentation (matching what auto-indent nested), not a re-derivation of the
// matching opener's column.
func (indentProvider) ReindentOnType(ctx editor.IndentContext, typed rune) (string, bool) {
	if typed != ':' || ctx.PrevIndent == "" {
		return "", false
	}
	if !isDedentClause(strings.TrimSpace(ctx.Line)) {
		return "", false
	}
	return trimOneLevel(ctx.PrevIndent, ctx.Unit), true
}

// dedentKeywords are the clause openers that belong to the enclosing block. The
// keyword is either the whole clause head ("else:") or followed by a condition
// ("elif x:", "except ValueError:").
var dedentKeywords = []string{"else", "elif", "except", "finally"}

// isDedentClause reports whether the trimmed line is a complete dedent clause —
// a dedent keyword (bare or with a condition) ending in ':'.
func isDedentClause(code string) bool {
	if !strings.HasSuffix(code, ":") {
		return false
	}
	head := strings.TrimSuffix(code, ":")
	for _, kw := range dedentKeywords {
		if head == kw || strings.HasPrefix(head, kw+" ") {
			return true
		}
	}
	return false
}

// trimOneLevel removes one indentation level from the end of the whitespace
// prefix indent: a full Unit when present, otherwise a single trailing tab or
// whatever shorter run remains.
func trimOneLevel(indent, unit string) string {
	if unit != "" && strings.HasSuffix(indent, unit) {
		return indent[:len(indent)-len(unit)]
	}
	if strings.HasSuffix(indent, "\t") {
		return indent[:len(indent)-1]
	}
	if n := len(indent) - len(unit); n > 0 {
		return indent[:n]
	}
	return ""
}
