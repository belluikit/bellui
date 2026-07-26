package widget

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// NewRichTextFromHTML configures a RichText widget by parsing the provided HTML content.
//
// Custom fonts cannot be selected via this constructor: the returned widget has
// no named-font allowlist yet, so any font-family / <font face> in the content
// falls through to the default behaviour. To let parsed content select a custom
// font, build the widget, register the font with RegisterNamedFont, then call
// ParseHTML / AppendHTML (#496).
func NewRichTextFromHTML(content string) *RichText {
	return NewRichText(parseHTML(content, nil)...)
}

// ParseHTML allows setting the content of this RichText widget from an HTML string.
// It will replace the content of this widget similarly to SetText, but with the appropriate formatting.
func (t *RichText) ParseHTML(content string) {
	t.Segments = parseHTML(content, t.namedFonts)
	t.rowBoundsVersion++ // invalidate row bounds cache
	t.visualCache = nil  // new segments — old cache keys are stale
	t.Refresh()
}

// AppendHTML parses the given HTML string and appends the content to the widget,
// with the appropriate formatting.
func (t *RichText) AppendHTML(content string) {
	t.Segments = append(t.Segments, parseHTML(content, t.namedFonts)...)
	t.rowBoundsVersion++ // invalidate row bounds cache
	t.Refresh()
}

// htmlStyle tracks inherited inline styling as we walk the HTML tree.
type htmlStyle struct {
	bold          bool
	italic        bool
	monospace     bool
	underline     bool
	strikethrough bool

	// fontSource is the inherited custom font resolved from a font-family /
	// <font face> that matched the RichText's named-font allowlist (#496). It
	// flows down to produced text segments; nil means the default font.
	fontSource bellui.Resource

	// named is the per-RichText named-font allowlist for this parse (#496). It is
	// set once at the root and carried unchanged through the walk; only registered
	// families can be selected. nil/empty means no custom fonts are available.
	named map[string]bellui.Resource
}

func (s htmlStyle) toRichTextStyle(inline bool) RichTextStyle {
	return RichTextStyle{
		ColorName:  theme.ColorNameForeground,
		Inline:     inline,
		SizeName:   theme.SizeNameText,
		FontSource: s.fontSource,
		TextStyle: bellui.TextStyle{
			Bold:          s.bold,
			Italic:        s.italic,
			Monospace:     s.monospace,
			Underline:     s.underline,
			Strikethrough: s.strikethrough,
		},
	}
}

// resolveNamedFont returns the first family in a comma-separated CSS font-family
// (or <font face>) list that matches the allowlist, or nil when none match.
func resolveNamedFont(list string, named map[string]bellui.Resource) bellui.Resource {
	if len(named) == 0 || list == "" {
		return nil
	}
	for _, family := range strings.Split(list, ",") {
		// Strip surrounding quotes CSS permits around family names.
		family = strings.Trim(strings.TrimSpace(family), `"'`)
		if res, ok := named[normalizeFontName(family)]; ok {
			return res
		}
	}
	return nil
}

// fontFamilyFromStyle extracts the value of the font-family declaration from an
// inline CSS style attribute, or "" if absent. Declarations are semicolon
// separated; the property name is matched case-insensitively.
func fontFamilyFromStyle(style string) string {
	if style == "" {
		return ""
	}
	for _, decl := range strings.Split(style, ";") {
		name, val, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "font-family") {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func parseHTML(content string, named map[string]bellui.Resource) []RichTextSegment {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		bellui.LogError("Failed to parse HTML", err)
		return nil
	}
	body := findBody(doc)
	if body == nil {
		return nil
	}
	return walkHTMLChildren(body, htmlStyle{named: named}, false, 0)
}

// findBody returns the <body> node in the parsed document tree.
func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBody(c); b != nil {
			return b
		}
	}
	return nil
}

func walkHTMLChildren(n *html.Node, style htmlStyle, blockquote bool, listDepth int) []RichTextSegment {
	var segs []RichTextSegment
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		segs = append(segs, walkHTMLNode(c, style, blockquote, listDepth)...)
	}
	return segs
}

func walkHTMLNode(n *html.Node, style htmlStyle, blockquote bool, listDepth int) []RichTextSegment {
	switch n.Type {
	case html.TextNode:
		return handleTextNode(n, style, blockquote)
	case html.ElementNode:
		return handleElementNode(n, style, blockquote, listDepth)
	case html.DocumentNode:
		return walkHTMLChildren(n, style, blockquote, listDepth)
	}
	return nil
}

func handleTextNode(n *html.Node, style htmlStyle, blockquote bool) []RichTextSegment {
	// Collapse internal whitespace runs to a single space (browser inline
	// whitespace processing), but — unlike a naive full trim — preserve a single
	// space at a run boundary when this text node sits *between inline siblings*
	// (e.g. "x <b>y</b>" must keep the space). Whitespace at a block edge, or a
	// pure-whitespace node between block elements (pretty-printed HTML), is still
	// dropped. Fixes boundary-space loss on HTML round-trips (#668).
	text := collapseWhitespace(n.Data)
	lead, trail := edgeSpaces(n.Data)
	keepLead := lead && isInlineHTMLNode(n.PrevSibling)
	keepTrail := trail && isInlineHTMLNode(n.NextSibling)

	if text == "" {
		// Pure-whitespace node: contributes a single space only when it bridges
		// two inline siblings; otherwise it is inter-block layout whitespace.
		if !(keepLead && keepTrail) {
			return nil
		}
		text = " "
	} else {
		if keepLead {
			text = " " + text
		}
		if keepTrail {
			text += " "
		}
	}

	if blockquote {
		return []RichTextSegment{&TextSegment{Style: RichTextStyleBlockquote, Text: text}}
	}
	return []RichTextSegment{&TextSegment{Style: style.toRichTextStyle(true), Text: text}}
}

// isCollapsibleSpace reports whether r is whitespace that HTML inline formatting
// collapses and trims. It excludes the non-breaking space U+00A0 (&nbsp;): a
// browser never collapses or trims a &nbsp;, it is significant content, so we
// treat it as an ordinary glyph rather than layout whitespace (#876).
func isCollapsibleSpace(r rune) bool {
	return r != '\u00A0' && unicode.IsSpace(r)
}

// hasVisibleText reports whether s contains any rune that is not collapsible
// whitespace — so a node that is only a &nbsp; counts as visible content.
func hasVisibleText(s string) bool {
	for _, r := range s {
		if !isCollapsibleSpace(r) {
			return true
		}
	}
	return false
}

// edgeSpaces reports whether s begins and/or ends with a collapsible whitespace
// rune (a &nbsp; at an edge is content, not a boundary space, so it does not
// count — #876).
func edgeSpaces(s string) (lead, trail bool) {
	if s == "" {
		return false, false
	}
	if r, _ := utf8.DecodeRuneInString(s); isCollapsibleSpace(r) {
		lead = true
	}
	if r, _ := utf8.DecodeLastRuneInString(s); isCollapsibleSpace(r) {
		trail = true
	}
	return lead, trail
}

// isInlineHTMLNode reports whether a sibling node contributes inline content on
// the same line, so a boundary space next to it is significant. Text nodes with
// visible content and inline elements are inline; block elements (and nil, i.e.
// the block edge) are not. Whitespace-only text is not itself inline content.
func isInlineHTMLNode(n *html.Node) bool {
	if n == nil {
		return false
	}
	switch n.Type {
	case html.TextNode:
		return hasVisibleText(n.Data)
	case html.ElementNode:
		return !isBlockAtom(n.DataAtom)
	}
	return false
}

// isBlockAtom reports whether an element is a block-level box for the purpose of
// inline whitespace trimming — the set handleElementNode renders as its own
// block (plus table structure and <br>). Everything else, including unknown
// tags that get stripped to their inline text, is treated as inline.
func isBlockAtom(a atom.Atom) bool {
	switch a {
	case atom.P, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6,
		atom.Blockquote, atom.Pre, atom.Hr, atom.Br,
		atom.Div, atom.Section, atom.Article, atom.Header, atom.Footer, atom.Nav, atom.Main,
		atom.Ul, atom.Ol, atom.Li,
		atom.Table, atom.Thead, atom.Tbody, atom.Tfoot, atom.Tr, atom.Td, atom.Th:
		return true
	}
	return false
}

func handleElementNode(n *html.Node, style htmlStyle, blockquote bool, listDepth int) []RichTextSegment {
	switch n.DataAtom {
	// Block elements
	case atom.P:
		children := walkHTMLChildren(n, style, blockquote, listDepth)
		return append(children, &TextSegment{Style: RichTextStyleParagraph})
	case atom.H1:
		return []RichTextSegment{&TextSegment{Style: RichTextStyleHeading, Text: collectText(n)}}
	case atom.H2:
		return []RichTextSegment{&TextSegment{Style: RichTextStyleSubHeading, Text: collectText(n)}}
	case atom.H3, atom.H4, atom.H5, atom.H6:
		return []RichTextSegment{&TextSegment{Style: htmlHeadingLevelStyle(n.DataAtom), Text: collectText(n)}}
	case atom.Blockquote:
		return walkHTMLChildren(n, style, true, listDepth)
	case atom.Pre:
		text := collectPreText(n)
		// Check for language class on <pre> or its <code> child (e.g., class="language-go")
		lang := detectPreLanguage(n)
		if lang != "" && codeLexerForLanguage(lang) != nil {
			return []RichTextSegment{&CodeSegment{Text: text, Language: lang}}
		}
		return []RichTextSegment{&TextSegment{Style: RichTextStyleCodeBlock, Text: text}}
	case atom.Hr:
		return []RichTextSegment{&SeparatorSegment{}}
	case atom.Br:
		return []RichTextSegment{&TextSegment{Style: RichTextStyleParagraph}}
	case atom.Div, atom.Section, atom.Article, atom.Header, atom.Footer, atom.Nav, atom.Main:
		return walkHTMLChildren(n, style, blockquote, listDepth)
	case atom.Ul:
		items := walkHTMLListItems(n, style, blockquote, listDepth+1)
		return []RichTextSegment{&ListSegment{Items: items, Ordered: false, indentationLevel: listDepth}}
	case atom.Ol:
		items := walkHTMLListItems(n, style, blockquote, listDepth+1)
		startIdx := 0 // startIndex = start - 1, default start=1 so startIndex=0
		for _, a := range n.Attr {
			if a.Key == "start" {
				if v, err := strconv.Atoi(a.Val); err == nil {
					startIdx = v - 1
				}
			}
		}
		return []RichTextSegment{&ListSegment{Items: items, Ordered: true, startIndex: startIdx, indentationLevel: listDepth}}
	case atom.Table:
		return parseHTMLTable(n)

	// Inline elements
	case atom.Strong, atom.B:
		s := style
		s.bold = true
		return walkHTMLChildren(n, s, blockquote, listDepth)
	case atom.Em, atom.I:
		s := style
		s.italic = true
		return walkHTMLChildren(n, s, blockquote, listDepth)
	case atom.S, atom.Del, atom.Strike:
		s := style
		s.strikethrough = true
		return walkHTMLChildren(n, s, blockquote, listDepth)
	case atom.U, atom.Ins:
		s := style
		s.underline = true
		return walkHTMLChildren(n, s, blockquote, listDepth)
	case atom.Code:
		return []RichTextSegment{&TextSegment{Style: RichTextStyleCodeInline, Text: collectText(n)}}
	case atom.A:
		href := htmlAttr(n, "href")
		link := parsedLinkURL(href)
		return []RichTextSegment{&HyperlinkSegment{
			Alignment: bellui.TextAlignLeading,
			Text:      collectText(n),
			URL:       link,
		}}
	case atom.Img:
		return parseHTMLImage(n)
	case atom.Span:
		s := applyNamedFont(style, fontFamilyFromStyle(htmlAttr(n, "style")))
		return walkHTMLChildren(n, s, blockquote, listDepth)
	case atom.Font:
		s := applyNamedFont(style, htmlAttr(n, "face"))
		return walkHTMLChildren(n, s, blockquote, listDepth)

	default:
		// Unsupported tags: strip the tag, keep text content
		return walkHTMLChildren(n, style, blockquote, listDepth)
	}
}

// htmlHeadingLevelStyle returns the bold paragraph style for an <h3>–<h6>
// heading, tagged with the matching 1-based HeadingLevel.
func htmlHeadingLevelStyle(a atom.Atom) RichTextStyle {
	s := RichTextStyleParagraph
	s.TextStyle.Bold = true
	switch a {
	case atom.H3:
		s.HeadingLevel = 3
	case atom.H4:
		s.HeadingLevel = 4
	case atom.H5:
		s.HeadingLevel = 5
	case atom.H6:
		s.HeadingLevel = 6
	}
	return s
}

// applyNamedFont returns style with its fontSource set to the app-registered
// resource matching the first family name in the comma-separated list, or
// unchanged when nothing matches (#496).
func applyNamedFont(style htmlStyle, families string) htmlStyle {
	if res := resolveNamedFont(families, style.named); res != nil {
		style.fontSource = res
	}
	return style
}

// walkHTMLListItems processes the children of a <ul> or <ol>, returning segments for each <li>.
func walkHTMLListItems(list *html.Node, style htmlStyle, blockquote bool, listDepth int) []RichTextSegment {
	var items []RichTextSegment
	for c := list.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.DataAtom != atom.Li {
			continue
		}
		children := walkHTMLChildren(c, style, blockquote, listDepth)
		var texts []RichTextSegment
		var sublist RichTextSegment
		for _, child := range children {
			if _, ok := child.(*ListSegment); ok {
				sublist = child
			} else {
				texts = append(texts, child)
			}
		}
		items = append(items, &ParagraphSegment{Texts: texts})
		if sublist != nil {
			items = append(items, sublist)
		}
	}
	return items
}

// parseHTMLTable converts an HTML <table> element into a TableSegment.
func parseHTMLTable(table *html.Node) []RichTextSegment {
	var header []string
	var rows [][]string
	var alignments []bellui.TextAlign

	// Collect all <tr> elements from the table, regardless of nesting in thead/tbody.
	// The x/net/html parser auto-inserts <tbody> around bare <tr> elements.
	var allRows []*html.Node
	var hasExplicitThead bool
	collectTableRows(table, &allRows, &hasExplicitThead)

	for i, row := range allRows {
		if i == 0 && hasExplicitThead {
			header, alignments = extractHTMLHeaderCells(row)
		} else if header == nil {
			// First row becomes header when there's no <thead>
			header = extractHTMLCells(row)
			alignments = make([]bellui.TextAlign, len(header))
			for j := range alignments {
				alignments[j] = bellui.TextAlignLeading
			}
		} else {
			rows = append(rows, extractHTMLCells(row))
		}
	}

	if header == nil {
		return nil
	}
	return []RichTextSegment{&TableSegment{
		Header:     header,
		Rows:       rows,
		Alignments: alignments,
	}}
}

// collectTableRows recursively finds all <tr> elements within a table and notes
// whether an explicit <thead> is present.
func collectTableRows(n *html.Node, rows *[]*html.Node, hasExplicitThead *bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.DataAtom {
		case atom.Thead:
			*hasExplicitThead = true
			collectTableRows(c, rows, hasExplicitThead)
		case atom.Tbody, atom.Tfoot:
			collectTableRows(c, rows, hasExplicitThead)
		case atom.Tr:
			*rows = append(*rows, c)
		}
	}
}

// extractHTMLHeaderCells extracts cell text and alignments from a header <tr>.
func extractHTMLHeaderCells(tr *html.Node) ([]string, []bellui.TextAlign) {
	var cells []string
	var aligns []bellui.TextAlign
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.DataAtom != atom.Th && c.DataAtom != atom.Td) {
			continue
		}
		cells = append(cells, strings.TrimFunc(collectText(c), isCollapsibleSpace))
		aligns = append(aligns, parseHTMLAlign(c))
	}
	return cells, aligns
}

// extractHTMLCells extracts cell text from a data <tr>. Cell edges are trimmed of
// *collapsible* whitespace only, so a leading/trailing &nbsp; (U+00A0) survives
// like an internal one (#914) rather than being trimmed by plain TrimSpace.
func extractHTMLCells(tr *html.Node) []string {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || (c.DataAtom != atom.Td && c.DataAtom != atom.Th) {
			continue
		}
		cells = append(cells, strings.TrimFunc(collectText(c), isCollapsibleSpace))
	}
	return cells
}

// parseHTMLAlign reads the align attribute from a table cell element.
func parseHTMLAlign(n *html.Node) bellui.TextAlign {
	switch htmlAttr(n, "align") {
	case "center":
		return bellui.TextAlignCenter
	case "right":
		return bellui.TextAlignTrailing
	default:
		return bellui.TextAlignLeading
	}
}

// collectText recursively gathers all text content from a node, collapsing whitespace.
func collectText(n *html.Node) string {
	var b strings.Builder
	collectTextInto(&b, n)
	return collapseWhitespace(b.String())
}

func collectTextInto(b *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectTextInto(b, c)
	}
}

// collectPreText recursively gathers all text from a <pre> node, preserving whitespace.
func collectPreText(n *html.Node) string {
	var b strings.Builder
	collectPreTextInto(&b, n)
	s := b.String()
	// Trim a single trailing newline (matching markdown code block behavior)
	s = strings.TrimRight(s, "\n")
	return s
}

func collectPreTextInto(b *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectPreTextInto(b, c)
	}
}

// collapseWhitespace replaces runs of collapsible whitespace with a single space
// and trims the result. A non-breaking space (U+00A0, &nbsp;) is not collapsible
// — it is preserved verbatim as content, so it survives and blocks trimming of an
// adjacent edge (#876).
func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := true // start true to trim leading whitespace
	for _, r := range s {
		if isCollapsibleSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	result := b.String()
	// Trim trailing space
	if len(result) > 0 && result[len(result)-1] == ' ' {
		result = result[:len(result)-1]
	}
	return result
}

// detectPreLanguage checks the <pre> or its <code> child for a language class.
// Supports class="language-go", class="lang-go", or data-language="go".
func detectPreLanguage(pre *html.Node) string {
	if lang := languageFromAttrs(pre); lang != "" {
		return lang
	}
	// Check <code> child
	for c := pre.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == atom.Code {
			if lang := languageFromAttrs(c); lang != "" {
				return lang
			}
		}
	}
	return ""
}

func languageFromAttrs(n *html.Node) string {
	if lang := htmlAttr(n, "data-language"); lang != "" {
		return lang
	}
	cls := htmlAttr(n, "class")
	for _, part := range strings.Fields(cls) {
		if strings.HasPrefix(part, "language-") {
			return strings.TrimPrefix(part, "language-")
		}
		if strings.HasPrefix(part, "lang-") {
			return strings.TrimPrefix(part, "lang-")
		}
	}
	return ""
}

// htmlAttr returns the value of the named attribute on the given node, or "" if not found.
func htmlAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
