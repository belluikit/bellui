// Package starlark provides a Starlark/Python syntax highlighter for editor.
//
// The highlighter recognises keywords, strings (including triple-quoted),
// comments, numbers, built-in functions, operators, and function calls.
//
// Extra built-in names can be registered via [Highlighter.AddBuiltins] to
// support domain-specific extensions (e.g. gameover's calc_kelly).
package starlark

import (
	"strings"
	"unicode"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/editor"
)

// Lexer states for multi-line string tracking.
const (
	stateNormal        = 0
	stateInDoubleQuote = 1
	stateInSingleQuote = 2
)

var defaultKeywords = map[string]bool{
	"def": true, "if": true, "else": true, "elif": true,
	"for": true, "in": true, "return": true, "True": true,
	"False": true, "None": true, "pass": true, "break": true,
	"continue": true, "lambda": true, "load": true,
}

var defaultLogicalOps = map[string]bool{
	"and": true, "or": true, "not": true,
}

var defaultBuiltins = map[string]bool{
	"len": true, "range": true, "str": true, "int": true,
	"float": true, "bool": true, "list": true, "dict": true,
	"type": true, "print": true, "enumerate": true, "zip": true,
	"min": true, "max": true, "sorted": true, "any": true,
	"all": true, "hasattr": true, "getattr": true, "dir": true,
	"abs": true, "repr": true, "hash": true, "log": true,
	"append": true, "clear": true, "extend": true, "keys": true,
	"values": true, "items": true, "pop": true, "update": true,
	"pow": true, "exp": true, "ln": true, "sqrt": true,
	"floor": true, "ceil": true, "clamp": true,
}

// Highlighter provides Starlark syntax highlighting.
// It implements [editor.Highlighter].
type Highlighter struct {
	builtins map[string]bool // merged: defaults + user-added
}

// New creates a Starlark highlighter with the default built-in set.
func New() *Highlighter {
	h := &Highlighter{builtins: make(map[string]bool, len(defaultBuiltins))}
	for k := range defaultBuiltins {
		h.builtins[k] = true
	}
	return h
}

// AddBuiltins registers additional names as built-in functions.
// This is useful for domain-specific extensions.
func (h *Highlighter) AddBuiltins(names ...string) {
	for _, n := range names {
		h.builtins[n] = true
	}
}

// Highlight implements [editor.Highlighter].
func (h *Highlighter) Highlight(line string, state int) ([]editor.StyleRun, int) {
	inString := state != stateNormal
	var stringQuote rune
	if state == stateInDoubleQuote {
		stringQuote = '"'
	} else if state == stateInSingleQuote {
		stringQuote = '\''
	}

	toks, stillInString, quote := tokenizeLine(line, inString, stringQuote, h.builtins)

	runs := make([]editor.StyleRun, len(toks))
	for i, tok := range toks {
		runs[i] = editor.StyleRun{
			Text:      tok.text,
			ColorName: tok.colorName(),
			Italic:    tok.kind == tokComment,
		}
	}

	newState := stateNormal
	if stillInString {
		if quote == '"' {
			newState = stateInDoubleQuote
		} else {
			newState = stateInSingleQuote
		}
	}
	return runs, newState
}

type tokenKind int

const (
	tokDefault  tokenKind = iota
	tokKeyword            // def, if, else, ...
	tokString             // "..." or '...'
	tokComment            // # ...
	tokNumber             // 42, 3.14
	tokBuiltin            // len, range, ...
	tokOperator           // =, ==, !=, ...
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
	case tokString:
		return editor.ColorNameSyntaxString
	case tokComment:
		return editor.ColorNameSyntaxComment
	case tokNumber:
		return editor.ColorNameSyntaxNumber
	case tokBuiltin:
		return editor.ColorNameSyntaxBuiltin
	case tokFunction:
		return editor.ColorNameSyntaxFunction
	case tokOperator:
		return editor.ColorNameSyntaxOperator
	default:
		return theme.ColorNameForeground
	}
}

func isOperator(r rune) bool {
	return strings.ContainsRune("=!<>+-*/%:()[]{}.,;@&|^~", r)
}

func tokenizeLine(line string, inString bool, stringQuote rune, builtins map[string]bool) ([]token, bool, rune) {
	var tokens []token
	runes := []rune(line)
	i := 0

	if inString {
		tripleQuote := string([]rune{stringQuote, stringQuote, stringQuote})
		idx := strings.Index(line, tripleQuote)
		if idx < 0 {
			tokens = append(tokens, token{kind: tokString, text: line})
			return tokens, true, stringQuote
		}
		end := idx + 3
		tokens = append(tokens, token{kind: tokString, text: string(runes[:end])})
		i = end
	}

	for i < len(runes) {
		r := runes[i]

		if r == '#' {
			tokens = append(tokens, token{kind: tokComment, text: string(runes[i:])})
			return tokens, false, 0
		}

		if r == '"' || r == '\'' {
			quote := r
			if i+2 < len(runes) && runes[i+1] == quote && runes[i+2] == quote {
				tripleQuote := string([]rune{quote, quote, quote})
				rest := string(runes[i+3:])
				endIdx := strings.Index(rest, tripleQuote)
				if endIdx < 0 {
					tokens = append(tokens, token{kind: tokString, text: string(runes[i:])})
					return tokens, true, quote
				}
				end := i + 3 + endIdx + 3
				tokens = append(tokens, token{kind: tokString, text: string(runes[i:end])})
				i = end
				continue
			}
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

		if unicode.IsDigit(r) || (r == '.' && i+1 < len(runes) && unicode.IsDigit(runes[i+1])) {
			j := i
			for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '.' ||
				runes[j] == 'e' || runes[j] == 'E' || runes[j] == 'x' || runes[j] == 'X' ||
				(j > i && (runes[j] == '+' || runes[j] == '-') && (runes[j-1] == 'e' || runes[j-1] == 'E'))) {
				j++
			}
			for j < len(runes) && ((runes[j] >= 'a' && runes[j] <= 'f') || (runes[j] >= 'A' && runes[j] <= 'F') || unicode.IsDigit(runes[j])) {
				j++
			}
			tokens = append(tokens, token{kind: tokNumber, text: string(runes[i:j])})
			i = j
			continue
		}

		if unicode.IsLetter(r) || r == '_' {
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			word := string(runes[i:j])
			kind := tokDefault
			if defaultKeywords[word] {
				kind = tokKeyword
			} else if defaultLogicalOps[word] {
				kind = tokOperator
			} else if builtins[word] {
				kind = tokBuiltin
			} else {
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

		if isOperator(r) {
			j := i + 1
			if j < len(runes) && isOperator(runes[j]) {
				j++
			}
			tokens = append(tokens, token{kind: tokOperator, text: string(runes[i:j])})
			i = j
			continue
		}

		j := i + 1
		for j < len(runes) && !unicode.IsLetter(runes[j]) && !unicode.IsDigit(runes[j]) &&
			runes[j] != '#' && runes[j] != '"' && runes[j] != '\'' &&
			!isOperator(runes[j]) && runes[j] != '_' {
			j++
		}
		tokens = append(tokens, token{kind: tokDefault, text: string(runes[i:j])})
		i = j
	}

	return tokens, false, 0
}
