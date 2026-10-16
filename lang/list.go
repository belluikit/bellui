package lang

import "strings"

// FormatList joins items into a single string using the current locale's CLDR
// list pattern. "a, b, and c" is English-specific — the conjunction word, the
// separators and the Oxford comma are all per-locale data — so this must not be
// hand-assembled with a literal ", " and "and". For example the same three
// items render "a, b, and c" (en) but "a, b und c" (de).
//
// The pattern strings live in the translation catalog under list.two / list.start
// / list.middle / list.end and follow the CLDR two/start/middle/end structure,
// so they react to lang.SetLocale like the other formatters. Items are inserted
// verbatim (already-final strings); no templating is applied to them.
//
// Since: 1.0
func FormatList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return applyListPattern(X("list.two", "{0} and {1}"), items[0], items[1])
	}

	// Three or more: acc = start(0,1), fold the middle, then end(acc, last).
	acc := applyListPattern(X("list.start", "{0}, {1}"), items[0], items[1])
	middle := X("list.middle", "{0}, {1}")
	for i := 2; i < len(items)-1; i++ {
		acc = applyListPattern(middle, acc, items[i])
	}
	return applyListPattern(X("list.end", "{0}, and {1}"), acc, items[len(items)-1])
}

// applyListPattern substitutes the CLDR list-pattern placeholders {0} and {1}.
// The replacement is simultaneous so an item that itself contains "{1}" is not
// re-substituted.
func applyListPattern(pattern, a, b string) string {
	return strings.NewReplacer("{0}", a, "{1}", b).Replace(pattern)
}
