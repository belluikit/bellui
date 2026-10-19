package canvas

// TextMeasureMemoForTest exposes Text's measurement memo (pre-rebrand tracker #923) so external
// tests can assert memoisation behaviour: entry identifies the current
// measurement (equal across memo hits, different after a re-measure), gen the
// theme/font cache generation it was taken under, ok whether one is populated.
func TextMeasureMemoForTest(t *Text) (entry any, gen uint64, ok bool) {
	t.measureMu.Lock()
	defer t.measureMu.Unlock()
	if !t.measured {
		return nil, 0, false
	}
	return t.measureMemo.seq, t.measureMemo.gen, true
}
