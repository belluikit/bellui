package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func tokenTexts(tokens []codeToken) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(t.text)
	}
	return b.String()
}

func findToken(tokens []codeToken, text string) *codeToken {
	for i := range tokens {
		if tokens[i].text == text {
			return &tokens[i]
		}
	}
	return nil
}

func TestGoLexer_Keywords(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("func main() { return }")

	tok := findToken(tokens, "func")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "return")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "main")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenText, tok.kind)
}

func TestGoLexer_Types(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("var x int = 42")

	tok := findToken(tokens, "int")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenType, tok.kind)

	tok = findToken(tokens, "42")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestGoLexer_Strings(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize(`x := "hello world"`)

	tok := findToken(tokens, `"hello world"`)
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)
}

func TestGoLexer_BacktickString(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := `raw\nstring`")

	tok := findToken(tokens, "`raw\nstring`")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)
}

func TestGoLexer_Comments(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 1 // comment")

	tok := findToken(tokens, "// comment")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenComment, tok.kind)
}

func TestGoLexer_BlockComment(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x /* block */ y")

	tok := findToken(tokens, "/* block */")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenComment, tok.kind)
}

func TestGoLexer_Builtins(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := make([]int, len(s))")

	tok := findToken(tokens, "make")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenBuiltin, tok.kind)

	tok = findToken(tokens, "len")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenBuiltin, tok.kind)
}

func TestGoLexer_Roundtrip(t *testing.T) {
	lex := goLexer{}
	code := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}"
	tokens := lex.tokenize(code)
	assert.Equal(t, code, tokenTexts(tokens))
}

func TestJSONLexer(t *testing.T) {
	lex := jsonLexer{}
	code := `{"name": "test", "count": 42, "active": true, "data": null}`
	tokens := lex.tokenize(code)

	assert.Equal(t, code, tokenTexts(tokens))

	tok := findToken(tokens, `"name"`)
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)

	tok = findToken(tokens, "42")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)

	tok = findToken(tokens, "true")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "null")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)
}

func TestYAMLLexer(t *testing.T) {
	lex := yamlLexer{}
	code := "name: test\ncount: 42\nactive: true"
	tokens := lex.tokenize(code)

	assert.Equal(t, code, tokenTexts(tokens))

	tok := findToken(tokens, "name")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "42")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)

	tok = findToken(tokens, "true")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenBuiltin, tok.kind)
}

func TestYAMLLexer_Comment(t *testing.T) {
	lex := yamlLexer{}
	tokens := lex.tokenize("key: value # comment")

	tok := findToken(tokens, "# comment")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenComment, tok.kind)
}

func TestYAMLLexer_Document(t *testing.T) {
	lex := yamlLexer{}
	tokens := lex.tokenize("---\nkey: value")

	tok := findToken(tokens, "---")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)
}

func TestStarlarkLexer(t *testing.T) {
	lex := starlarkLexer{}
	code := "def foo(x):\n  return x + 1"
	tokens := lex.tokenize(code)

	assert.Equal(t, code, tokenTexts(tokens))

	tok := findToken(tokens, "def")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "return")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "1")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestStarlarkLexer_Builtins(t *testing.T) {
	lex := starlarkLexer{}
	tokens := lex.tokenize("x = len(items)")

	tok := findToken(tokens, "len")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenBuiltin, tok.kind)
}

func TestStarlarkLexer_TripleString(t *testing.T) {
	lex := starlarkLexer{}
	tokens := lex.tokenize(`x = """multi\nline"""`)

	found := false
	for _, tok := range tokens {
		if tok.kind == tokenString && strings.Contains(tok.text, `"""`) {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find triple-quoted string")
}

func TestStarlarkLexer_Comment(t *testing.T) {
	lex := starlarkLexer{}
	tokens := lex.tokenize("x = 1 # comment")

	tok := findToken(tokens, "# comment")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenComment, tok.kind)
}

func TestProtoLexer(t *testing.T) {
	lex := protoLexer{}
	code := "syntax = \"proto3\";\n\nmessage Foo {\n  int32 id = 1;\n}"
	tokens := lex.tokenize(code)

	assert.Equal(t, code, tokenTexts(tokens))

	tok := findToken(tokens, "syntax")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "message")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "int32")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenType, tok.kind)

	tok = findToken(tokens, `"proto3"`)
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)

	tok = findToken(tokens, "1")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestProtoLexer_Comment(t *testing.T) {
	lex := protoLexer{}
	tokens := lex.tokenize("// comment\nmessage Foo {}")

	tok := findToken(tokens, "// comment")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenComment, tok.kind)
}

func TestCodeLexerForLanguage(t *testing.T) {
	assert.NotNil(t, codeLexerForLanguage("go"))
	assert.NotNil(t, codeLexerForLanguage("golang"))
	assert.NotNil(t, codeLexerForLanguage("Go"))
	assert.NotNil(t, codeLexerForLanguage("json"))
	assert.NotNil(t, codeLexerForLanguage("yaml"))
	assert.NotNil(t, codeLexerForLanguage("yml"))
	assert.NotNil(t, codeLexerForLanguage("starlark"))
	assert.NotNil(t, codeLexerForLanguage("bzl"))
	assert.NotNil(t, codeLexerForLanguage("proto"))
	assert.NotNil(t, codeLexerForLanguage("protobuf"))
	assert.Nil(t, codeLexerForLanguage("python"))
	assert.Nil(t, codeLexerForLanguage(""))
}

func TestCodeSegment_Markdown(t *testing.T) {
	r := NewRichTextFromMarkdown("```go\nfunc main() {}\n```")

	assert.Len(t, r.Segments, 1)
	code, ok := r.Segments[0].(*CodeSegment)
	if !ok {
		t.Fatal("Segment should be CodeSegment")
	}
	assert.Equal(t, "go", code.Language)
	assert.Equal(t, "func main() {}", code.Text)
}

func TestCodeSegment_MarkdownJSON(t *testing.T) {
	r := NewRichTextFromMarkdown("```json\n{\"key\": \"value\"}\n```")

	assert.Len(t, r.Segments, 1)
	code, ok := r.Segments[0].(*CodeSegment)
	if !ok {
		t.Fatal("Segment should be CodeSegment")
	}
	assert.Equal(t, "json", code.Language)
}

func TestCodeSegment_MarkdownProto(t *testing.T) {
	r := NewRichTextFromMarkdown("```proto\nmessage Foo {}\n```")

	assert.Len(t, r.Segments, 1)
	code, ok := r.Segments[0].(*CodeSegment)
	if !ok {
		t.Fatal("Segment should be CodeSegment")
	}
	assert.Equal(t, "proto", code.Language)
}

func TestCodeSegment_UnrecognizedLanguage(t *testing.T) {
	r := NewRichTextFromMarkdown("```python\nprint('hello')\n```")

	assert.Len(t, r.Segments, 1)
	_, ok := r.Segments[0].(*TextSegment)
	assert.True(t, ok, "Unrecognized language should fall back to TextSegment")
}

func TestCodeSegment_HTMLWithLanguageClass(t *testing.T) {
	r := NewRichTextFromHTML(`<pre><code class="language-go">func main() {}</code></pre>`)

	assert.Len(t, r.Segments, 1)
	code, ok := r.Segments[0].(*CodeSegment)
	if !ok {
		t.Fatal("Segment should be CodeSegment")
	}
	assert.Equal(t, "go", code.Language)
}

// --- Roundtrip tests: tokenized text must reconstruct the original ---

func TestJSONLexer_Roundtrip(t *testing.T) {
	code := "{\n  \"name\": \"test\",\n  \"count\": 42,\n  \"active\": true,\n  \"data\": null\n}"
	assert.Equal(t, code, tokenTexts(jsonLexer{}.tokenize(code)))
}

func TestYAMLLexer_Roundtrip(t *testing.T) {
	code := "---\nname: test\ncount: 42\nactive: true\n# comment\nlist:\n  - item1\n  - item2"
	assert.Equal(t, code, tokenTexts(yamlLexer{}.tokenize(code)))
}

func TestStarlarkLexer_Roundtrip(t *testing.T) {
	code := "def foo(x):\n  if x > 0:\n    return x\n  # comment\n  return None"
	assert.Equal(t, code, tokenTexts(starlarkLexer{}.tokenize(code)))
}

func TestProtoLexer_Roundtrip(t *testing.T) {
	code := "syntax = \"proto3\";\n\npackage example;\n\nmessage Foo {\n  int32 id = 1;\n  string name = 2;\n}"
	assert.Equal(t, code, tokenTexts(protoLexer{}.tokenize(code)))
}

// --- Escaped strings ---

func TestGoLexer_EscapedString(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize(`x := "hello \"world\""`)

	tok := findToken(tokens, `"hello \"world\""`)
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)
}

func TestJSONLexer_EscapedString(t *testing.T) {
	lex := jsonLexer{}
	tokens := lex.tokenize(`{"key": "val\"ue"}`)

	found := false
	for _, tok := range tokens {
		if tok.kind == tokenString && tok.text == `"val\"ue"` {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find escaped string")
}

func TestStarlarkLexer_EscapedString(t *testing.T) {
	lex := starlarkLexer{}
	tokens := lex.tokenize(`x = "hello \"world\""`)

	tok := findToken(tokens, `"hello \"world\""`)
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)
}

func TestProtoLexer_EscapedString(t *testing.T) {
	lex := protoLexer{}
	tokens := lex.tokenize(`option java_package = "com.example.\"test\"";`)

	found := false
	for _, tok := range tokens {
		if tok.kind == tokenString && tok.text == `"com.example.\"test\""` {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find escaped string")
}

// --- Hex / octal / binary numbers ---

func TestGoLexer_HexNumber(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 0xFF")

	tok := findToken(tokens, "0xFF")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestGoLexer_OctalNumber(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 0o755")

	tok := findToken(tokens, "0o755")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestGoLexer_BinaryNumber(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 0b1010")

	tok := findToken(tokens, "0b1010")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

// --- Empty code block with language ---

func TestCodeSegment_EmptyCodeBlock(t *testing.T) {
	r := NewRichTextFromMarkdown("```go\n```")

	// Empty fenced code block produces no segment
	assert.Empty(t, r.Segments)
}

// --- Starlark while keyword ---

func TestStarlarkLexer_WhileNotKeyword(t *testing.T) {
	// "while" is not supported in Starlark (per https://bazel.build/rules/language)
	lex := starlarkLexer{}
	tokens := lex.tokenize("while True:")

	tok := findToken(tokens, "while")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenText, tok.kind, "while should not be a keyword in Starlark")

	tok = findToken(tokens, "True")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenBuiltin, tok.kind)
}

// --- CodeSegment Visual rendering ---

func TestCodeSegment_Visual(t *testing.T) {
	seg := &CodeSegment{Text: "func main() {}", Language: "go"}
	vis := seg.Visual()
	assert.NotNil(t, vis)

	cb, ok := vis.(*richCodeBlock)
	assert.True(t, ok, "Visual should return a richCodeBlock")
	assert.Equal(t, "func main() {}", cb.text)
	assert.Equal(t, "go", cb.language)
	assert.Greater(t, len(cb.tokens), 0, "Should have tokens")
}

func TestCodeSegment_VisualNoLanguage(t *testing.T) {
	seg := &CodeSegment{Text: "plain text", Language: ""}
	vis := seg.Visual()
	assert.NotNil(t, vis)

	cb := vis.(*richCodeBlock)
	// No lexer, so single token with full text
	assert.Len(t, cb.tokens, 1)
	assert.Equal(t, "plain text", cb.tokens[0].text)
	assert.Equal(t, tokenText, cb.tokens[0].kind)
}

func TestCodeSegment_Update(t *testing.T) {
	seg := &CodeSegment{Text: "var x int", Language: "go"}
	vis := seg.Visual()

	seg2 := &CodeSegment{Text: "func foo() {}", Language: "go"}
	seg2.Update(vis)

	cb := vis.(*richCodeBlock)
	assert.Equal(t, "func foo() {}", cb.text)
}

func TestCodeSegment_Textual(t *testing.T) {
	seg := &CodeSegment{Text: "hello world", Language: "go"}
	assert.Equal(t, "hello world", seg.Textual())
}

func TestCodeSegment_Inline(t *testing.T) {
	seg := &CodeSegment{Text: "x", Language: "go"}
	assert.False(t, seg.Inline())
}

// --- YAML edge cases ---

func TestYAMLLexer_NestedKeys(t *testing.T) {
	lex := yamlLexer{}
	code := "parent:\n  child: value"
	tokens := lex.tokenize(code)
	assert.Equal(t, code, tokenTexts(tokens))

	tok := findToken(tokens, "parent")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)

	tok = findToken(tokens, "child")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenKeyword, tok.kind)
}

func TestYAMLLexer_QuotedString(t *testing.T) {
	lex := yamlLexer{}
	tokens := lex.tokenize(`key: "quoted value"`)

	tok := findToken(tokens, `"quoted value"`)
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)
}

// --- Go edge cases ---

func TestGoLexer_FloatNumber(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 3.14")

	tok := findToken(tokens, "3.14")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestGoLexer_ScientificNumber(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 1e10")

	tok := findToken(tokens, "1e10")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

func TestGoLexer_CharLiteral(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := 'a'")

	tok := findToken(tokens, "'a'")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenString, tok.kind)
}

func TestGoLexer_NilTrueFalse(t *testing.T) {
	lex := goLexer{}
	tokens := lex.tokenize("x := nil; y := true; z := false")

	for _, word := range []string{"nil", "true", "false"} {
		tok := findToken(tokens, word)
		assert.NotNil(t, tok, "Should find %s", word)
		assert.Equal(t, tokenBuiltin, tok.kind, "%s should be builtin", word)
	}
}

// --- JSON negative number ---

func TestJSONLexer_NegativeNumber(t *testing.T) {
	lex := jsonLexer{}
	tokens := lex.tokenize(`{"val": -42}`)

	tok := findToken(tokens, "-42")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenNumber, tok.kind)
}

// --- Proto block comment ---

func TestProtoLexer_BlockComment(t *testing.T) {
	lex := protoLexer{}
	tokens := lex.tokenize("/* block\ncomment */\nmessage Foo {}")

	tok := findToken(tokens, "/* block\ncomment */")
	assert.NotNil(t, tok)
	assert.Equal(t, tokenComment, tok.kind)
}

// --- HTML lang- prefix variant ---

func TestCodeSegment_HTMLWithLangPrefix(t *testing.T) {
	r := NewRichTextFromHTML(`<pre><code class="lang-json">{"key": 1}</code></pre>`)

	assert.Len(t, r.Segments, 1)
	code, ok := r.Segments[0].(*CodeSegment)
	if !ok {
		t.Fatal("Segment should be CodeSegment")
	}
	assert.Equal(t, "json", code.Language)
}

func TestCodeSegment_HTMLDataLanguage(t *testing.T) {
	r := NewRichTextFromHTML(`<pre data-language="yaml">key: val</pre>`)

	assert.Len(t, r.Segments, 1)
	code, ok := r.Segments[0].(*CodeSegment)
	if !ok {
		t.Fatal("Segment should be CodeSegment")
	}
	assert.Equal(t, "yaml", code.Language)
}

// --- splitTokensIntoLines ---

func TestSplitTokensIntoLines(t *testing.T) {
	tokens := []codeToken{
		{text: "func", kind: tokenKeyword},
		{text: " main() {\n", kind: tokenText},
		{text: "}", kind: tokenText},
	}
	lines := splitTokensIntoLines(tokens)
	assert.Len(t, lines, 2)
	assert.Equal(t, "func", lines[0][0].text)
	assert.Equal(t, "}", lines[1][0].text)
}

func BenchmarkGoLexer(b *testing.B) {
	code := `package main

import "fmt"

func main() {
	x := 42
	name := "hello"
	// comment
	fmt.Println(name, x)
}
`
	lex := goLexer{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		lex.tokenize(code)
	}
}
