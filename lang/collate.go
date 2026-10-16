package lang

import (
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// CollatorOption tunes how a [Collator] (or [CompareFold]) weights differences
// between strings. The zero set of options gives the locale's default ordering:
// case- and accent-sensitive, with runs of digits compared lexically.
//
// Since: 1.0
type CollatorOption int

const (
	// CollateNumeric compares embedded runs of digits by numeric value rather
	// than code point, so "file2" sorts before "file10".
	CollateNumeric CollatorOption = iota

	// CollateIgnoreCase makes comparison case-insensitive ("a" == "A").
	CollateIgnoreCase

	// CollateIgnoreDiacritics ignores diacritical marks ("o" == "ö"). Combined
	// with CollateIgnoreCase this yields a loose, search-friendly matching.
	CollateIgnoreDiacritics
)

func collateOptions(opts []CollatorOption) []collate.Option {
	if len(opts) == 0 {
		return nil
	}
	out := make([]collate.Option, 0, len(opts))
	for _, o := range opts {
		switch o {
		case CollateNumeric:
			out = append(out, collate.Numeric)
		case CollateIgnoreCase:
			out = append(out, collate.IgnoreCase)
		case CollateIgnoreDiacritics:
			out = append(out, collate.IgnoreDiacritics)
		}
	}
	return out
}

// currentCollationTag resolves the language tag collation should use. It mirrors
// [CurrentLocale] but returns the raw language.Tag: the active override when set,
// otherwise the system locale, falling back to the (root) undetermined tag.
func currentCollationTag() language.Tag {
	if tag := getCurrentTag(); tag != (language.Tag{}) {
		return tag
	}
	if tag, err := language.Parse(string(SystemLocale())); err == nil {
		return tag
	}
	return language.Und
}

// Collator compares strings under a fixed locale and set of options. Create one
// with [NewCollator] when sorting many strings so the collator is reused rather
// than rebuilt per comparison.
//
// A Collator captures the active locale at construction time and does not track
// later [SetLocale] changes; build a fresh one if the locale may have changed.
// Like the underlying golang.org/x/text/collate.Collator it is not safe for
// concurrent use — give each goroutine its own, or use the package-level
// [Compare], which is goroutine-safe.
//
// Since: 1.0
type Collator struct {
	c *collate.Collator
}

// NewCollator returns a [Collator] for the active locale (see [CurrentLocale])
// with the given options applied.
//
// Since: 1.0
func NewCollator(opts ...CollatorOption) *Collator {
	return &Collator{c: collate.New(currentCollationTag(), collateOptions(opts)...)}
}

// Compare returns -1, 0 or +1 as a sorts before, equal to, or after b under the
// collator's locale and options.
//
// Since: 1.0
func (c *Collator) Compare(a, b string) int {
	return c.c.CompareString(a, b)
}

// SortStrings sorts s in place under the collator's locale and options.
//
// Since: 1.0
func (c *Collator) SortStrings(s []string) {
	c.c.SortStrings(s)
}

// pooledCollator remembers the tag its collator was built for so a cached one is
// discarded and rebuilt after the active locale changes.
type pooledCollator struct {
	tag language.Tag
	c   *collate.Collator
}

// compareVia borrows a collator from pool (rebuilding it if the locale changed
// or the pool was empty), compares, and returns it. It is goroutine-safe: each
// caller gets exclusive use of a collator for the duration of the comparison,
// which is why single-comparison callers must not share one Collator directly.
func compareVia(pool *sync.Pool, opts []CollatorOption, a, b string) int {
	tag := currentCollationTag()
	pc, _ := pool.Get().(*pooledCollator)
	if pc == nil || pc.tag != tag {
		pc = &pooledCollator{tag: tag, c: collate.New(tag, collateOptions(opts)...)}
	}
	res := pc.c.CompareString(a, b)
	pool.Put(pc)
	return res
}

var (
	comparePool     sync.Pool
	compareFoldPool sync.Pool
)

// Compare returns -1, 0 or +1 as a sorts before, equal to, or after b under the
// active locale (see [CurrentLocale]). This is the locale-aware replacement for
// comparing user-visible strings by code point, which mis-sorts most non-English
// text (for example German "äpfel" belongs with "a", and Swedish "ö" sorts after
// "z"). Use it directly as the body of a sort closure:
//
//	sort.Slice(names, func(i, j int) bool { return lang.Compare(names[i], names[j]) < 0 })
//
// Compare is safe for concurrent use and stays in step with [SetLocale]. Callers
// sorting a large slice may prefer a reusable [Collator] from [NewCollator].
//
// Note: referencing collation links the golang.org/x/text/collate tables
// (~1.75 MB); binaries that never call into this file do not pay that cost.
//
// Since: 1.0
func Compare(a, b string) int {
	return compareVia(&comparePool, nil, a, b)
}

// CompareFold is like [Compare] but case- and accent-insensitive, suited to
// user-facing search and matching (for example filtering a list as the user
// types). It is safe for concurrent use.
//
// Since: 1.0
func CompareFold(a, b string) int {
	return compareVia(&compareFoldPool, []CollatorOption{CollateIgnoreCase, CollateIgnoreDiacritics}, a, b)
}
