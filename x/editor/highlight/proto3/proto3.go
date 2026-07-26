// Package proto3 provides a Protocol Buffers syntax highlighter for editor.
//
// The highlighter recognises proto keywords (syntax, package, import, message,
// enum, service, rpc, ...), built-in scalar types (int32, string, bytes, ...),
// boolean and range constants (true, false, max, inf, nan), strings (double or
// single quoted), comments (including multi-line block comments), numbers
// (decimal, hex, octal, float), operators, type names declared by message/enum/
// service/oneof/extend, and rpc method names.
package proto3

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
)

var keywords = map[string]bool{
	"syntax": true, "edition": true, "package": true, "import": true,
	"weak": true, "public": true, "option": true, "message": true,
	"enum": true, "oneof": true, "service": true, "rpc": true,
	"returns": true, "stream": true, "repeated": true, "optional": true,
	"required": true, "map": true, "reserved": true, "extensions": true,
	"extend": true, "to": true, "group": true,
}

var scalarTypes = map[string]bool{
	"double": true, "float": true,
	"int32": true, "int64": true, "uint32": true, "uint64": true,
	"sint32": true, "sint64": true, "fixed32": true, "fixed64": true,
	"sfixed32": true, "sfixed64": true,
	"bool": true, "string": true, "bytes": true,
}

var constants = map[string]bool{
	"true": true, "false": true, "max": true, "inf": true, "nan": true,
}

// declKeywords introduce a named declaration; the identifier that follows is
// coloured as a type name.
var declKeywords = map[string]bool{
	"message": true, "enum": true, "service": true, "oneof": true,
	"extend": true, "group": true,
}

// Highlighter provides Protocol Buffers syntax highlighting.
// It implements [editor.Highlighter].
type Highlighter struct{}

// New creates a proto highlighter.
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
	tokKeyword            // message, rpc, repeated, ...
	tokType               // int32, string, or a declared type name
	tokConstant           // true, false, max, inf, nan
	tokString             // "..." or '...'
	tokComment            // // ... or /* ... */
	tokNumber             // 42, 0xFF, 3.14
	tokOperator           // =, <, >, ...
	tokFunction           // rpc method name: name(
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
	return strings.ContainsRune("=<>+-;,.(){}[]", r)
}

func tokenizeLine(line string, state int) ([]token, int) {
	var tokens []token
	runes := []rune(line)
	i := 0

	if state == stateBlockComment {
		idx := strings.Index(line, "*/")
		if idx < 0 {
			tokens = append(tokens, token{kind: tokComment, text: line})
			return tokens, stateBlockComment
		}
		end := idx + 2
		tokens = append(tokens, token{kind: tokComment, text: string(runes[:end])})
		i = end
	}

	// The last keyword seen on this line, used to colour the identifier that
	// follows message/enum/service/oneof/extend as a type name.
	prevWord := ""

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

		// String literal: "..." or '...' (proto strings never span lines).
		if r == '"' || r == '\'' {
			quote := r
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\\' {
					j += 2
					continue
				}
				if runes[j] == quote {
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
			if r == '0' && j+1 < len(runes) && (runes[j+1] == 'x' || runes[j+1] == 'X') {
				j += 2
				for j < len(runes) && isHexDigit(runes[j]) {
					j++
				}
			} else {
				j = scanDecimal(runes, j)
			}
			tokens = append(tokens, token{kind: tokNumber, text: string(runes[i:j])})
			i = j
			prevWord = ""
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
				if declKeywords[prevWord] {
					// The name declared by message/enum/service/oneof/extend.
					kind = tokType
				} else {
					// An rpc method name: ident followed by '('.
					k := j
					for k < len(runes) && runes[k] == ' ' {
						k++
					}
					if k < len(runes) && runes[k] == '(' {
						kind = tokFunction
					}
				}
			}
			tokens = append(tokens, token{kind: kind, text: word})
			prevWord = word
			i = j
			continue
		}

		// Operators/punctuation.
		if isOperator(r) {
			tokens = append(tokens, token{kind: tokOperator, text: string(r)})
			i++
			prevWord = ""
			continue
		}

		// Anything else.
		tokens = append(tokens, token{kind: tokDefault, text: string(r)})
		i++
		prevWord = ""
	}

	return tokens, stateNormal
}

func classifyWord(word string) tokenKind {
	if keywords[word] {
		return tokKeyword
	}
	if scalarTypes[word] {
		return tokType
	}
	if constants[word] {
		return tokConstant
	}
	return tokDefault
}

// scanDecimal consumes a decimal or octal number (possibly float with
// exponent) starting at runes[start] and returns the index past the last
// consumed rune.
func scanDecimal(runes []rune, start int) int {
	j := start
	for j < len(runes) && unicode.IsDigit(runes[j]) {
		j++
	}
	if j < len(runes) && runes[j] == '.' {
		j++
		for j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
		}
	}
	if j < len(runes) && (runes[j] == 'e' || runes[j] == 'E') {
		j++
		if j < len(runes) && (runes[j] == '+' || runes[j] == '-') {
			j++
		}
		for j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
		}
	}
	return j
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
