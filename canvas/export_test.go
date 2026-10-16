package canvas

// TextMeasureMemoForTest exposes Text's measurement memo (pre-rebrand tracker #923) so external
// tests can assert memoisation behaviour: entry is the current memo as an
// opaque pointer (stable across memo hits, replaced on re-measure), gen the
// theme/font cache generation it was taken under, ok whether one is populated.
func TextMeasureMemoForTest(t *Text) (entry any, gen uint64, ok bool) {
	m := t.measureMemo.Load()
	if m == nil {
		return nil, 0, false
	}
	return m, m.gen, true
}
