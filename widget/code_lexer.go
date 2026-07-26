package widget

import (
	"strings"
	"unicode"
)

// tokenKind represents the type of a code token for syntax highlighting.
type tokenKind int

const (
	tokenText        tokenKind = iota // default text
	tokenKeyword                      // language keywords
	tokenType                         // type names
	tokenBuiltin                      // builtin functions/constants
	tokenString                       // string literals
	tokenNumber                       // numeric literals
	tokenComment                      // comments
	tokenPunctuation                  // operators and punctuation
)

// codeToken is a single colored run of text produced by a lexer.
type codeToken struct {
	text string
	kind tokenKind
}

// codeLexer tokenizes source code into colored tokens.
type codeLexer interface {
	tokenize(code string) []codeToken
}

// codeLexerForLanguage returns a lexer for the given language identifier, or nil if unsupported.
func codeLexerForLanguage(lang string) codeLexer {
	switch strings.ToLower(lang) {
	case "go", "golang":
		return goLexer{}
	case "json":
		return jsonLexer{}
	case "yaml", "yml":
		return yamlLexer{}
	case "starlark", "bzl", "star":
		return starlarkLexer{}
	case "proto", "protobuf", "proto3", "proto2":
		return protoLexer{}
	}
	return nil
}

// --- shared lexing helpers ---

type lexer struct {
	src  []rune
	pos  int
	toks []codeToken
}

func newLexer(code string) *lexer {
	return &lexer{src: []rune(code)}
}

func (l *lexer) done() bool    { return l.pos >= len(l.src) }
func (l *lexer) peek() rune    { return l.src[l.pos] }
func (l *lexer) advance() rune { r := l.src[l.pos]; l.pos++; return r }

func (l *lexer) emit(text string, kind tokenKind) {
	if text != "" {
		l.toks = append(l.toks, codeToken{text: text, kind: kind})
	}
}

func (l *lexer) scanWhile(pred func(rune) bool) string {
	start := l.pos
	for l.pos < len(l.src) && pred(l.src[l.pos]) {
		l.pos++
	}
	return string(l.src[start:l.pos])
}

func (l *lexer) scanLineComment(prefix string) {
	start := l.pos
	l.pos += len([]rune(prefix))
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.pos++
	}
	l.emit(string(l.src[start:l.pos]), tokenComment)
}

func (l *lexer) scanBlockComment() {
	start := l.pos
	l.pos += 2 // skip /*
	for l.pos < len(l.src)-1 {
		if l.src[l.pos] == '*' && l.src[l.pos+1] == '/' {
			l.pos += 2
			l.emit(string(l.src[start:l.pos]), tokenComment)
			return
		}
		l.pos++
	}
	l.pos = len(l.src) // unterminated
	l.emit(string(l.src[start:l.pos]), tokenComment)
}

func (l *lexer) scanString(quote rune) {
	start := l.pos
	l.pos++ // skip opening quote
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		l.pos++
		if ch == '\\' && l.pos < len(l.src) {
			l.pos++ // skip escaped char
		} else if ch == quote {
			l.emit(string(l.src[start:l.pos]), tokenString)
			return
		} else if ch == '\n' && quote != '`' {
			// unterminated single-line string
			l.emit(string(l.src[start:l.pos]), tokenString)
			return
		}
	}
	l.emit(string(l.src[start:l.pos]), tokenString)
}

func (l *lexer) scanBacktickString() {
	start := l.pos
	l.pos++ // skip opening `
	for l.pos < len(l.src) {
		if l.src[l.pos] == '`' {
			l.pos++
			l.emit(string(l.src[start:l.pos]), tokenString)
			return
		}
		l.pos++
	}
	l.emit(string(l.src[start:l.pos]), tokenString)
}

func (l *lexer) scanNumber() {
	start := l.pos
	if l.src[l.pos] == '0' && l.pos+1 < len(l.src) {
		next := l.src[l.pos+1]
		if next == 'x' || next == 'X' || next == 'o' || next == 'O' || next == 'b' || next == 'B' {
			l.pos += 2
			l.scanWhile(func(r rune) bool { return isHexDigit(r) || r == '_' })
			l.emit(string(l.src[start:l.pos]), tokenNumber)
			return
		}
	}
	l.scanWhile(func(r rune) bool { return unicode.IsDigit(r) || r == '_' })
	if l.pos < len(l.src) && l.src[l.pos] == '.' && l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1]) {
		l.pos++
		l.scanWhile(func(r rune) bool { return unicode.IsDigit(r) || r == '_' })
	}
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
		}
		l.scanWhile(func(r rune) bool { return unicode.IsDigit(r) })
	}
	l.emit(string(l.src[start:l.pos]), tokenNumber)
}

func (l *lexer) scanIdentifier() string {
	return l.scanWhile(func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' })
}

func (l *lexer) scanWhitespaceAndPunctuation() {
	start := l.pos
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if unicode.IsLetter(ch) || ch == '_' || unicode.IsDigit(ch) || ch == '"' || ch == '\'' || ch == '`' || ch == '#' || ch == '/' {
			break
		}
		// Break on '-' followed by digit so negative numbers are handled by the main loop
		if ch == '-' && l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1]) {
			break
		}
		l.pos++
	}
	if l.pos > start {
		l.emit(string(l.src[start:l.pos]), tokenText)
	}
}

func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func classifyWord(word string, keywords, types, builtins map[string]bool) tokenKind {
	if keywords[word] {
		return tokenKeyword
	}
	if types[word] {
		return tokenType
	}
	if builtins[word] {
		return tokenBuiltin
	}
	return tokenText
}

// --- Go lexer ---

type goLexer struct{}

var goKeywords = toSet("break", "case", "chan", "const", "continue", "default", "defer",
	"else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
	"map", "package", "range", "return", "select", "struct", "switch", "type", "var")

var goTypes = toSet("bool", "byte", "complex64", "complex128", "error", "float32", "float64",
	"int", "int8", "int16", "int32", "int64", "rune", "string", "uint", "uint8",
	"uint16", "uint32", "uint64", "uintptr", "any")

var goBuiltins = toSet("append", "cap", "clear", "close", "complex", "copy", "delete",
	"imag", "len", "make", "max", "min", "new", "panic", "print", "println", "real",
	"recover", "true", "false", "nil", "iota")

func (goLexer) tokenize(code string) []codeToken {
	l := newLexer(code)
	for !l.done() {
		ch := l.peek()
		switch {
		case ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/':
			l.scanLineComment("//")
		case ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*':
			l.scanBlockComment()
		case ch == '"':
			l.scanString('"')
		case ch == '\'':
			l.scanString('\'')
		case ch == '`':
			l.scanBacktickString()
		case unicode.IsDigit(ch):
			l.scanNumber()
		case unicode.IsLetter(ch) || ch == '_':
			word := l.scanIdentifier()
			l.emit(word, classifyWord(word, goKeywords, goTypes, goBuiltins))
		default:
			l.scanWhitespaceAndPunctuation()
		}
	}
	return l.toks
}

// --- JSON lexer ---

type jsonLexer struct{}

var jsonKeywords = toSet("true", "false", "null")

func (jsonLexer) tokenize(code string) []codeToken {
	l := newLexer(code)
	for !l.done() {
		ch := l.peek()
		switch {
		case ch == '"':
			l.scanString('"')
		case unicode.IsDigit(ch) || (ch == '-' && l.pos+1 < len(l.src) && unicode.IsDigit(l.src[l.pos+1])):
			if ch == '-' {
				l.pos++
			}
			start := l.pos
			if ch == '-' {
				start--
			}
			l.scanNumber()
			// overwrite the last token with the correct range including the minus
			if ch == '-' && len(l.toks) > 0 {
				l.toks[len(l.toks)-1].text = string(l.src[start:l.pos])
			}
		case unicode.IsLetter(ch):
			word := l.scanIdentifier()
			l.emit(word, classifyWord(word, jsonKeywords, nil, nil))
		default:
			l.scanWhitespaceAndPunctuation()
		}
	}
	return l.toks
}

// --- YAML lexer ---

type yamlLexer struct{}

var yamlConstants = toSet("true", "false", "null", "yes", "no", "on", "off",
	"True", "False", "Null", "Yes", "No", "On", "Off",
	"TRUE", "FALSE", "NULL", "YES", "NO", "ON", "OFF")

func (yamlLexer) tokenize(code string) []codeToken {
	l := newLexer(code)
	lineStart := true

	for !l.done() {
		ch := l.peek()
		switch {
		case ch == '#':
			l.scanLineComment("#")
			lineStart = false
		case ch == '\n':
			l.emit(string(l.advance()), tokenText)
			lineStart = true
		case ch == '"':
			l.scanString('"')
			lineStart = false
		case ch == '\'':
			l.scanString('\'')
			lineStart = false
		case l.yamlIsDocSeparator():
			l.emit("---", tokenKeyword)
			l.pos += 3
			lineStart = false
		case lineStart && (unicode.IsLetter(ch) || ch == '_'):
			l.yamlScanKey()
			lineStart = false
		case unicode.IsDigit(ch):
			l.scanNumber()
			lineStart = false
		case unicode.IsLetter(ch):
			l.yamlScanWord()
			lineStart = false
		default:
			// Consume whitespace/punctuation but preserve lineStart for indentation
			if l.yamlScanWhitespaceAndPunctuation() {
				lineStart = false
			}
		}
	}
	return l.toks
}

// yamlIsDocSeparator reports whether the lexer is positioned at a "---" document separator.
func (l *lexer) yamlIsDocSeparator() bool {
	return l.peek() == '-' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '-' &&
		l.pos+2 < len(l.src) && l.src[l.pos+2] == '-'
}

// yamlScanKey scans a potential mapping key (or plain text if not followed by ':').
func (l *lexer) yamlScanKey() {
	start := l.pos
	l.scanWhile(func(r rune) bool { return r != ':' && r != '\n' && r != '#' })
	text := string(l.src[start:l.pos])
	if l.pos < len(l.src) && l.src[l.pos] == ':' {
		l.emit(text, tokenKeyword)
	} else {
		l.emit(text, tokenText)
	}
}

// yamlScanWord scans an identifier, emitting it as a builtin constant or plain text.
func (l *lexer) yamlScanWord() {
	word := l.scanIdentifier()
	if yamlConstants[word] {
		l.emit(word, tokenBuiltin)
	} else {
		l.emit(word, tokenText)
	}
}

// yamlIsWordOrSpecialStart reports whether r begins a token handled by another branch.
func yamlIsWordOrSpecialStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_' || unicode.IsDigit(r) ||
		r == '"' || r == '\'' || r == '`' || r == '#' || r == '/' || r == '\n'
}

// yamlScanWhitespaceAndPunctuation consumes a run of whitespace/punctuation and emits it
// as plain text. It returns true if the run contained any non-whitespace character.
func (l *lexer) yamlScanWhitespaceAndPunctuation() bool {
	start := l.pos
	onlyWhitespace := true
	for l.pos < len(l.src) {
		r := l.src[l.pos]
		if yamlIsWordOrSpecialStart(r) {
			break
		}
		if r == '-' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == '-' || unicode.IsDigit(l.src[l.pos+1])) {
			break
		}
		if !unicode.IsSpace(r) {
			onlyWhitespace = false
		}
		l.pos++
	}
	if l.pos > start {
		l.emit(string(l.src[start:l.pos]), tokenText)
	}
	return !onlyWhitespace
}

// --- Starlark lexer ---

type starlarkLexer struct{}

var starlarkKeywords = toSet("and", "break", "continue", "def", "elif", "else",
	"for", "if", "in", "lambda", "load", "not", "or", "pass", "return")

var starlarkBuiltins = toSet("True", "False", "None", "len", "range", "str", "int",
	"float", "list", "dict", "tuple", "bool", "print", "type", "hasattr", "getattr",
	"enumerate", "zip", "sorted", "reversed", "min", "max", "any", "all", "dir",
	"hash", "repr", "fail", "struct")

func (starlarkLexer) tokenize(code string) []codeToken {
	l := newLexer(code)
	for !l.done() {
		ch := l.peek()
		switch {
		case ch == '#':
			l.scanLineComment("#")
		case ch == '"':
			if l.pos+2 < len(l.src) && l.src[l.pos+1] == '"' && l.src[l.pos+2] == '"' {
				l.scanTripleString('"')
			} else {
				l.scanString('"')
			}
		case ch == '\'':
			if l.pos+2 < len(l.src) && l.src[l.pos+1] == '\'' && l.src[l.pos+2] == '\'' {
				l.scanTripleString('\'')
			} else {
				l.scanString('\'')
			}
		case unicode.IsDigit(ch):
			l.scanNumber()
		case unicode.IsLetter(ch) || ch == '_':
			word := l.scanIdentifier()
			l.emit(word, classifyWord(word, starlarkKeywords, nil, starlarkBuiltins))
		default:
			l.scanWhitespaceAndPunctuation()
		}
	}
	return l.toks
}

func (l *lexer) scanTripleString(quote rune) {
	start := l.pos
	l.pos += 3 // skip opening triple
	for l.pos < len(l.src)-2 {
		if l.src[l.pos] == quote && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
			l.pos += 3
			l.emit(string(l.src[start:l.pos]), tokenString)
			return
		}
		l.pos++
	}
	l.pos = len(l.src)
	l.emit(string(l.src[start:l.pos]), tokenString)
}

// --- Protobuf lexer ---

type protoLexer struct{}

var protoKeywords = toSet("syntax", "package", "import", "option", "message", "enum",
	"service", "rpc", "returns", "oneof", "map", "repeated", "optional", "required",
	"reserved", "extend", "extensions", "to", "max", "stream", "weak", "public")

var protoTypes = toSet("double", "float", "int32", "int64", "uint32", "uint64",
	"sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64",
	"bool", "string", "bytes")

var protoBuiltins = toSet("true", "false")

func (protoLexer) tokenize(code string) []codeToken {
	l := newLexer(code)
	for !l.done() {
		ch := l.peek()
		switch {
		case ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/':
			l.scanLineComment("//")
		case ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*':
			l.scanBlockComment()
		case ch == '"':
			l.scanString('"')
		case ch == '\'':
			l.scanString('\'')
		case unicode.IsDigit(ch):
			l.scanNumber()
		case unicode.IsLetter(ch) || ch == '_':
			word := l.scanIdentifier()
			l.emit(word, classifyWord(word, protoKeywords, protoTypes, protoBuiltins))
		default:
			l.scanWhitespaceAndPunctuation()
		}
	}
	return l.toks
}

// --- helpers ---

func toSet(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}
