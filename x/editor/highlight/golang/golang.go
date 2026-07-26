// Package golang provides a Go syntax highlighter for editor.
//
// The highlighter recognises keywords, built-in types, built-in functions,
// predeclared constants (true, false, nil, iota), strings (including raw
// backtick strings that span multiple lines), rune literals, comments
// (including multi-line block comments), numbers (decimal, hex, octal,
// binary, float, imaginary), operators, and function calls.
package golang

import (
	"strings"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/editor"
)

// Lexer states for multi-line constructs.
const (
	stateNormal       = 0
	stateBlockComment = 1
	stateRawString    = 2
)

var keywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true,
	"continue": true, "default": true, "defer": true, "else": true,
	"fallthrough": true, "for": true, "func": true, "go": true,
	"goto": true, "if": true, "import": true, "interface": true,
	"map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true,
	"var": true,
}

var builtinTypes = map[string]bool{
	"bool": true, "byte": true, "comparable": true,
	"complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"rune": true, "string": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "any": true,
}

var builtinFuncs = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true,
	"complex": true, "copy": true, "delete": true, "imag": true,
	"len": true, "make": true, "max": true, "min": true,
	"new": true, "panic": true, "print": true, "println": true,
	"real": true, "recover": true,
}

var predeclaredConsts = map[string]bool{
	"true": true, "false": true, "nil": true, "iota": true,
}

// Highlighter provides Go syntax highlighting.
// It implements [editor.Highlighter].
type Highlighter struct{}

// New creates a Go highlighter.
func New() *Highlighter {
	return &Highlighter{}
}

// Highlight implements [editor.Highlighter].
func (h *Highlighter) Highlight(line string, state int) ([]editor.StyleRun, int) {
	toks, newState := tokenizeLine(line, state)

	runs := make([]editor.StyleRun, len(toks))
	for i, tok := range toks {
		runs[i] = editor.StyleRun{
			Text:      tok.text,
			ColorName: tok.colorName(),
			Italic:    tok.kind == tokComment,
		}
	}
	return runs, newState
}

type tokenKind int

const (
	tokDefault  tokenKind = iota
	tokKeyword            // func, if, return, ...
	tokType               // int, string, error, ...
	tokBuiltin            // len, make, append, ...
	tokConstant           // true, false, nil, iota
	tokString             // "..." or `...`
	tokComment            // // ... or /* ... */
	tokNumber             // 42, 0xFF, 3.14, 1i
	tokOperator           // +, -, :=, ==, ...
	tokFunction           // name(
)

type token struct {
	kind tokenKind
	text string
}

func (t token) colorName() bellui.ThemeColorName {
	switch t.kind {
	case tokKeyword:
		return editor.ColorNameSyntaxKeyword
	case tokType:
		return editor.ColorNameSyntaxType
	case tokBuiltin:
		return editor.ColorNameSyntaxBuiltin
	case tokConstant:
		return editor.ColorNameSyntaxBuiltin
	case tokString:
		return editor.ColorNameSyntaxString
	case tokComment:
		return editor.ColorNameSyntaxComment
	case tokNumber:
		return editor.ColorNameSyntaxNumber
	case tokFunction:
		return editor.ColorNameSyntaxFunction
	case tokOperator:
		return editor.ColorNameSyntaxOperator
	default:
		return theme.ColorNameForeground
	}
}

func isOperator(r rune) bool {
	return strings.ContainsRune("+-*/%&|^<>=!:;.,()[]{}~", r)
}

func tokenizeLine(line string, state int) ([]token, int) {
	var tokens []token
	runes := []rune(line)
	i := 0

	switch state {
	case stateBlockComment:
		idx := strings.Index(line, "*/")
		if idx < 0 {
			tokens = append(tokens, token{kind: tokComment, text: line})
			return tokens, stateBlockComment
		}
		end := idx + 2
		tokens = append(tokens, token{kind: tokComment, text: string(runes[:end])})
		i = end

	case stateRawString:
		idx := strings.IndexRune(line, '`')
		if idx < 0 {
			tokens = append(tokens, token{kind: tokString, text: line})
			return tokens, stateRawString
		}
		end := idx + 1
		tokens = append(tokens, token{kind: tokString, text: string(runes[:end])})
		i = end
	}

	for i < len(runes) {
		r := runes[i]

		// Whitespace.
		if r == ' ' || r == '\t' {
			j := i + 1
			for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
				j++
			}
			tokens = append(tokens, token{kind: tokDefault, text: string(runes[i:j])})
			i = j
			continue
		}

		// Comments.
		if r == '/' && i+1 < len(runes) {
			if runes[i+1] == '/' {
				tokens = append(tokens, token{kind: tokComment, text: string(runes[i:])})
				return tokens, stateNormal
			}
			if runes[i+1] == '*' {
				rest := string(runes[i+2:])
				closeIdx := strings.Index(rest, "*/")
				if closeIdx < 0 {
					tokens = append(tokens, token{kind: tokComment, text: string(runes[i:])})
					return tokens, stateBlockComment
				}
				end := i + 2 + closeIdx + 2
				tokens = append(tokens, token{kind: tokComment, text: string(runes[i:end])})
				i = end
				continue
			}
		}

		// Raw string literal: `...`
		if r == '`' {
			rest := string(runes[i+1:])
			closeIdx := strings.IndexRune(rest, '`')
			if closeIdx < 0 {
				tokens = append(tokens, token{kind: tokString, text: string(runes[i:])})
				return tokens, stateRawString
			}
			end := i + 1 + closeIdx + 1
			tokens = append(tokens, token{kind: tokString, text: string(runes[i:end])})
			i = end
			continue
		}

		// Interpreted string: "..."
		if r == '"' {
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\\' {
					j += 2
					continue
				}
				if runes[j] == '"' {
					j++
					break
				}
				j++
			}
			tokens = append(tokens, token{kind: tokString, text: string(runes[i:j])})
			i = j
			continue
		}

		// Rune literal: '...'
		if r == '\'' {
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\\' {
					j += 2
					continue
				}
				if runes[j] == '\'' {
					j++
					break
				}
				j++
			}
			tokens = append(tokens, token{kind: tokString, text: string(runes[i:j])})
			i = j
			continue
		}

		// Numbers.
		if unicode.IsDigit(r) || (r == '.' && i+1 < len(runes) && unicode.IsDigit(runes[i+1])) {
			j := i
			if r == '0' && j+1 < len(runes) {
				next := runes[j+1]
				if next == 'x' || next == 'X' {
					// Hex.
					j += 2
					for j < len(runes) && (isHexDigit(runes[j]) || runes[j] == '_') {
						j++
					}
				} else if next == 'o' || next == 'O' {
					// Octal.
					j += 2
					for j < len(runes) && ((runes[j] >= '0' && runes[j] <= '7') || runes[j] == '_') {
						j++
					}
				} else if next == 'b' || next == 'B' {
					// Binary.
					j += 2
					for j < len(runes) && (runes[j] == '0' || runes[j] == '1' || runes[j] == '_') {
						j++
					}
				} else {
					j = scanDecimal(runes, j)
				}
			} else {
				j = scanDecimal(runes, j)
			}
			// Imaginary suffix.
			if j < len(runes) && runes[j] == 'i' {
				j++
			}
			tokens = append(tokens, token{kind: tokNumber, text: string(runes[i:j])})
			i = j
			continue
		}

		// Identifiers and keywords.
		if unicode.IsLetter(r) || r == '_' {
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			word := string(runes[i:j])
			kind := classifyWord(word)
			if kind == tokDefault {
				// Check if it's a function call: ident followed by '('.
				k := j
				for k < len(runes) && runes[k] == ' ' {
					k++
				}
				if k < len(runes) && runes[k] == '(' {
					kind = tokFunction
				}
			}
			tokens = append(tokens, token{kind: kind, text: word})
			i = j
			continue
		}

		// Operators/punctuation.
		if isOperator(r) {
			j := i + 1
			// Consume multi-character operators like :=, ==, !=, <=, >=, <<, >>, <-, &&, ||, ...
			if j < len(runes) && isOperator(runes[j]) {
				j++
				if j < len(runes) && isOperator(runes[j]) {
					j++ // e.g. <<=, >>=
				}
			}
			tokens = append(tokens, token{kind: tokOperator, text: string(runes[i:j])})
			i = j
			continue
		}

		// Anything else.
		tokens = append(tokens, token{kind: tokDefault, text: string(r)})
		i++
	}

	return tokens, stateNormal
}

func classifyWord(word string) tokenKind {
	if keywords[word] {
		return tokKeyword
	}
	if builtinTypes[word] {
		return tokType
	}
	if builtinFuncs[word] {
		return tokBuiltin
	}
	if predeclaredConsts[word] {
		return tokConstant
	}
	return tokDefault
}

// scanDecimal consumes a decimal number (possibly float with exponent) starting
// at runes[start] and returns the index past the last consumed rune.
func scanDecimal(runes []rune, start int) int {
	j := start
	// Integer part.
	for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '_') {
		j++
	}
	// Fractional part.
	if j < len(runes) && runes[j] == '.' {
		j++
		for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '_') {
			j++
		}
	}
	// Exponent.
	if j < len(runes) && (runes[j] == 'e' || runes[j] == 'E') {
		j++
		if j < len(runes) && (runes[j] == '+' || runes[j] == '-') {
			j++
		}
		for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '_') {
			j++
		}
	}
	return j
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
