// Package pxf provides a PXF (ProtoWire text format) syntax highlighter for
// editor.
//
// PXF is a human-friendly text encoding for Protocol Buffer messages. The
// highlighter recognises comments (# // /* */), strings (including triple-
// quoted), byte literals (b"..."), booleans, numbers, timestamps, durations,
// @type directives, enum values (ALL_CAPS identifiers), operators, and field
// names.
package pxf

import (
	"image/color"
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
	stateTripleString = 2
)

// Highlighter provides PXF syntax highlighting.
// It implements [editor.Highlighter].
type Highlighter struct {
	// declared holds the workspace-supplied names consulted for schema-informed
	// coloring; a zero value (all-nil sets) reduces to the static behavior.
	declared editor.DeclaredNameSets
}

// New creates a PXF highlighter.
func New() *Highlighter {
	return &Highlighter{}
}

// SetDeclaredNames seeds the highlighter with the workspace's declared RFC-001
// names (see [editor.DeclaredNames]). It colors an @name use site as an
// annotation when name is a declared annotation, and a bare identifier as a
// type when it names a declared type — on top of the static rules, which always
// win. The set is line-based and stale between analyze passes, so refresh it on
// the analyze-completion path and call [editor.Editor.RefreshHighlighter]
// after. Passing a zero [editor.DeclaredNames] clears the workspace names.
//
// PXF is a value-encoding format, so declared functions have no use site here
// and [editor.DeclaredNames.Functions] is ignored.
func (h *Highlighter) SetDeclaredNames(names editor.DeclaredNames) {
	h.declared = names.Sets()
}

// Highlight implements [editor.Highlighter].
func (h *Highlighter) Highlight(line string, state int) ([]editor.StyleRun, int) {
	toks, newState := h.tokenizeLine(line, state)

	// Detect light/dark variant for color selection.
	isLight := false
	if app := bellui.CurrentApp(); app != nil {
		isLight = app.Settings().ThemeVariant() == theme.VariantLight
	}

	runs := make([]editor.StyleRun, len(toks))
	for i, tok := range toks {
		run := editor.StyleRun{
			Text:      tok.text,
			ColorName: tok.colorName(),
			Italic:    tok.kind == tokComment,
		}
		// On light backgrounds, use direct high-contrast colors instead
		// of the Monokai-inspired theme colors (designed for dark bg).
		if isLight {
			if c := lightColor(tok.kind); c != nil {
				run.ForegroundColor = c
				run.ColorName = ""
			}
		}
		runs[i] = run
	}
	return runs, newState
}

// ----------------------------------------------------------------------------
// Token types
// ----------------------------------------------------------------------------

type tokenKind int

const (
	tokDefault    tokenKind = iota
	tokComment              // # ... or // ... or /* ... */
	tokString               // "..." or """...""" or b"..."
	tokNumber               // 42, 3.14
	tokKeyword              // true, false, structural blocks
	tokDirective            // @type
	tokEnum                 // ALL_CAPS identifiers
	tokOperator             // = :
	tokTimestamp            // 2024-01-15T10:30:00Z
	tokDuration             // 1h30m, 30s
	tokPunct                // { } [ ] ,
	tokField                // field names (identifier before = or :)
	tokType                 // PascalCase type names (after type =), or declared type
	tokAnnotation           // @name naming a declared annotation
)

type token struct {
	kind tokenKind
	text string
}

func (t token) colorName() bellui.ThemeColorName {
	switch t.kind {
	case tokComment:
		return editor.ColorNameSyntaxComment
	case tokString:
		return editor.ColorNameSyntaxString
	case tokNumber, tokTimestamp, tokDuration:
		return editor.ColorNameSyntaxNumber
	case tokKeyword, tokDirective:
		return editor.ColorNameSyntaxKeyword
	case tokEnum:
		return editor.ColorNameSyntaxBuiltin
	case tokOperator:
		return editor.ColorNameSyntaxOperator
	case tokField:
		return editor.ColorNameSyntaxFunction
	case tokType:
		return editor.ColorNameSyntaxType
	case tokAnnotation:
		return editor.ColorNameSyntaxAnnotation
	default:
		return theme.ColorNameForeground
	}
}

// lightColors provides high-contrast syntax colors for light backgrounds.
var lightColors = map[tokenKind]color.Color{
	tokComment:    color.NRGBA{R: 0x6A, G: 0x73, B: 0x7D, A: 0xFF}, // slate grey
	tokString:     color.NRGBA{R: 0x03, G: 0x2F, B: 0x62, A: 0xFF}, // dark blue
	tokNumber:     color.NRGBA{R: 0x00, G: 0x5C, B: 0xC5, A: 0xFF}, // blue
	tokTimestamp:  color.NRGBA{R: 0x00, G: 0x5C, B: 0xC5, A: 0xFF}, // blue
	tokDuration:   color.NRGBA{R: 0x00, G: 0x5C, B: 0xC5, A: 0xFF}, // blue
	tokKeyword:    color.NRGBA{R: 0xAF, G: 0x00, B: 0xDB, A: 0xFF}, // purple
	tokDirective:  color.NRGBA{R: 0xAF, G: 0x00, B: 0xDB, A: 0xFF}, // purple
	tokEnum:       color.NRGBA{R: 0x00, G: 0x6B, B: 0x75, A: 0xFF}, // teal
	tokOperator:   color.NRGBA{R: 0xD6, G: 0x33, B: 0x84, A: 0xFF}, // magenta
	tokPunct:      color.NRGBA{R: 0x0B, G: 0x7B, B: 0x00, A: 0xFF}, // green
	tokField:      color.NRGBA{R: 0x6F, G: 0x42, B: 0x00, A: 0xFF}, // dark amber
	tokType:       color.NRGBA{R: 0x00, G: 0x6B, B: 0x75, A: 0xFF}, // teal
	tokAnnotation: color.NRGBA{R: 0xB0, G: 0x54, B: 0x00, A: 0xFF}, // burnt orange
}

// lightColor returns a direct foreground color for light theme.
func lightColor(kind tokenKind) color.Color {
	if c, ok := lightColors[kind]; ok {
		return c
	}
	return nil
}

// ----------------------------------------------------------------------------
// Tokenizer
// ----------------------------------------------------------------------------

func (h *Highlighter) tokenizeLine(line string, state int) ([]token, int) {
	var tokens []token
	runes := []rune(line)
	i := 0

	// Handle continuation of multi-line constructs from a previous line.
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

	case stateTripleString:
		idx := strings.Index(line, `"""`)
		if idx < 0 {
			tokens = append(tokens, token{kind: tokString, text: line})
			return tokens, stateTripleString
		}
		end := idx + 3
		tokens = append(tokens, token{kind: tokString, text: string(runes[:end])})
		i = end
	}

	for i < len(runes) {
		r := runes[i]

		// Whitespace — collect as default text.
		if r == ' ' || r == '\t' {
			j := i + 1
			for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
				j++
			}
			tokens = append(tokens, token{kind: tokDefault, text: string(runes[i:j])})
			i = j
			continue
		}

		// Line comment: # or //
		if r == '#' {
			tokens = append(tokens, token{kind: tokComment, text: string(runes[i:])})
			return tokens, stateNormal
		}
		if r == '/' && i+1 < len(runes) {
			if runes[i+1] == '/' {
				tokens = append(tokens, token{kind: tokComment, text: string(runes[i:])})
				return tokens, stateNormal
			}
			// Block comment: /* ... */
			if runes[i+1] == '*' {
				rest := string(runes[i+2:])
				closeIdx := strings.Index(rest, "*/")
				if closeIdx < 0 {
					// Block comment continues on next line.
					tokens = append(tokens, token{kind: tokComment, text: string(runes[i:])})
					return tokens, stateBlockComment
				}
				end := i + 2 + closeIdx + 2
				tokens = append(tokens, token{kind: tokComment, text: string(runes[i:end])})
				i = end
				continue
			}
		}

		// Directive or annotation use site: @type / @<identifier>. An @name that
		// names a workspace-declared annotation is coloured as an annotation;
		// every other @ident keeps the built-in directive color.
		if r == '@' {
			j := i + 1
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_' || runes[j] == '.') {
				j++
			}
			kind := tokDirective
			if h.declared.Annotations[string(runes[i+1:j])] {
				kind = tokAnnotation
			}
			tokens = append(tokens, token{kind: kind, text: string(runes[i:j])})
			i = j
			continue
		}

		// Byte literal: b"..."
		if r == 'b' && i+1 < len(runes) && runes[i+1] == '"' {
			j := i + 2
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

		// String: "..." or """..."""
		if r == '"' {
			// Triple-quoted string.
			if i+2 < len(runes) && runes[i+1] == '"' && runes[i+2] == '"' {
				rest := string(runes[i+3:])
				closeIdx := strings.Index(rest, `"""`)
				if closeIdx < 0 {
					// Triple-quoted string continues on next line.
					tokens = append(tokens, token{kind: tokString, text: string(runes[i:])})
					return tokens, stateTripleString
				}
				end := i + 3 + closeIdx + 3
				tokens = append(tokens, token{kind: tokString, text: string(runes[i:end])})
				i = end
				continue
			}
			// Regular double-quoted string.
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

		// Number, timestamp, or duration starting with a digit.
		if unicode.IsDigit(r) {
			tok := scanNumberLike(runes, i)
			tokens = append(tokens, tok)
			i += len([]rune(tok.text))
			continue
		}

		// Negative number: - followed by digit.
		if r == '-' && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
			tok := scanNumberLike(runes, i+1)
			tok.text = "-" + tok.text
			tokens = append(tokens, tok)
			i += len([]rune(tok.text))
			continue
		}

		// Identifier or keyword.
		if unicode.IsLetter(r) || r == '_' {
			j := i
			for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			word := string(runes[i:j])

			// Look ahead past whitespace to see what follows the identifier.
			kind := classifyWord(word)
			if kind == tokDefault {
				k := j
				for k < len(runes) && (runes[k] == ' ' || runes[k] == '\t') {
					k++
				}
				if k < len(runes) {
					switch runes[k] {
					case '=', ':':
						kind = tokField
					case '{':
						if isBlockKeyword(word) {
							kind = tokKeyword
						}
					}
				}
				// A bare reference to a workspace-declared type (when it wasn't
				// resolved as a field/block above).
				if kind == tokDefault && h.declared.Types[word] {
					kind = tokType
				}
			}

			tokens = append(tokens, token{kind: kind, text: word})
			i = j
			continue
		}

		// Operators: = :
		if r == '=' || r == ':' {
			tokens = append(tokens, token{kind: tokOperator, text: string(r)})
			i++
			continue
		}

		// Punctuation: { } [ ] ,
		if r == '{' || r == '}' || r == '[' || r == ']' || r == ',' {
			tokens = append(tokens, token{kind: tokPunct, text: string(r)})
			i++
			continue
		}

		// Anything else — emit as default.
		tokens = append(tokens, token{kind: tokDefault, text: string(r)})
		i++
	}

	return tokens, stateNormal
}

// scanNumberLike scans a numeric-looking token starting at runes[start].
// It distinguishes plain numbers, RFC3339 timestamps, and Go-style durations.
// The caller must ensure runes[start] is a digit.
func scanNumberLike(runes []rune, start int) token {
	j := start

	// Consume digits.
	for j < len(runes) && unicode.IsDigit(runes[j]) {
		j++
	}

	// Check for timestamp: digits followed by '-' then digit (e.g. 2024-01-...).
	if j < len(runes) && runes[j] == '-' && j+1 < len(runes) && unicode.IsDigit(runes[j+1]) {
		// Looks like a timestamp — consume the rest greedily.
		// RFC3339: 2024-01-15T10:30:00Z or 2024-01-15T10:30:00+05:30
		for j < len(runes) && !isTimestampTerminator(runes[j]) {
			j++
		}
		text := string(runes[start:j])
		if looksLikeTimestamp(text) {
			return token{kind: tokTimestamp, text: text}
		}
		// Not a valid timestamp; fall back to just the leading digits.
		j = start
		for j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
		}
	}

	// Check for float: digits followed by '.'
	if j < len(runes) && runes[j] == '.' {
		j++
		for j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
		}
		// Scientific notation.
		if j < len(runes) && (runes[j] == 'e' || runes[j] == 'E') {
			j++
			if j < len(runes) && (runes[j] == '+' || runes[j] == '-') {
				j++
			}
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
		}
		return token{kind: tokNumber, text: string(runes[start:j])}
	}

	// Check for duration suffix: digits followed by h, m, s, ms, us, ns, etc.
	if j < len(runes) && isDurationUnit(runes[j]) {
		// Consume the full duration: e.g. 1h30m15s
		for j < len(runes) && (unicode.IsDigit(runes[j]) || isDurationUnit(runes[j])) {
			j++
		}
		text := string(runes[start:j])
		if looksLikeDuration(text) {
			return token{kind: tokDuration, text: text}
		}
		// Fall back to plain number.
		j = start
		for j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
		}
	}

	// Hex literal: 0x...
	if j == start+1 && runes[start] == '0' && j < len(runes) && (runes[j] == 'x' || runes[j] == 'X') {
		j++
		for j < len(runes) && isHexDigit(runes[j]) {
			j++
		}
	}

	return token{kind: tokNumber, text: string(runes[start:j])}
}

// classifyWord returns the token kind for an identifier-like word.
func classifyWord(word string) tokenKind {
	switch word {
	case "true", "false":
		return tokKeyword
	}
	if isAllCaps(word) {
		return tokEnum
	}
	return tokDefault
}

// isBlockKeyword returns true for PXF structural block names.
func isBlockKeyword(word string) bool {
	switch word {
	case "state", "root", "lifecycle", "children", "slots", "defaults",
		"routes", "colors", "typography", "shapes", "spacing",
		"message", "enum", "oneof", "service", "rpc", "option",
		"reserved", "extensions", "syntax", "package", "import":
		return true
	}
	return false
}

// isAllCaps returns true if word matches the pattern [A-Z][A-Z0-9_]* with at
// least one underscore (to distinguish enum values like STATUS_ACTIVE from
// single-word field names that happen to be short).
func isAllCaps(word string) bool {
	if len(word) < 2 {
		return false
	}
	hasUnderscore := false
	for i, r := range word {
		if r == '_' {
			hasUnderscore = true
			continue
		}
		if i == 0 && !unicode.IsUpper(r) {
			return false
		}
		if !unicode.IsUpper(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return hasUnderscore
}

func isTimestampTerminator(r rune) bool {
	return r == ' ' || r == '\t' || r == ',' || r == ']' || r == '}' || r == '#'
}

// looksLikeTimestamp checks whether text roughly matches RFC3339.
// We only need a heuristic — the lexer already filtered for digit-dash-digit
// at the start.
func looksLikeTimestamp(s string) bool {
	// Minimum: 2024-01-15T10:30:00Z  (20 chars)
	if len(s) < 10 {
		return false
	}
	// Must contain T (date-time separator).
	return strings.ContainsRune(s, 'T') || strings.ContainsRune(s, 't')
}

func isDurationUnit(r rune) bool {
	return r == 'h' || r == 'm' || r == 's' || r == 'u' || r == 'n'
}

// looksLikeDuration returns true for strings like "30s", "1h30m", "500ms".
func looksLikeDuration(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must end with a duration unit letter.
	last := rune(s[len(s)-1])
	return isDurationUnit(last)
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
