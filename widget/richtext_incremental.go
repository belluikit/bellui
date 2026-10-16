package widget

import (
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Edit-incremental dense build + render (pre-rebrand tracker #921).
//
// RichEntry re-syncs its provider on every keystroke, and the dense
// (full-materialisation) RichText path rebuilt the whole row model, object
// list and layout each time — O(document) per edit even though an edit changes
// only one segment (pre-rebrand tracker #896 removed the measurement cost; this removes the
// bookkeeping).
//
// Mechanism: the dense build (denseBuildCtx.iterate) optionally records a
// checkpoint of its iterate state before each top-level segment. On the next
// rebuild the new segment list is diffed against a snapshot of the old one by
// identity + content (pointer/string compares, allocation-free); only the
// changed segment range is re-processed from its checkpoint and the untouched
// prefix/tail rows are spliced around it. The renderer then regenerates
// objects and layout for the dirty row range only.
//
// Every step is gated and falls back to the full rebuild/render on any
// mismatch, so the incremental path can only ever reproduce what the full
// path would have built.

// denseIncrMinSegments is the document size below which checkpoints are not
// recorded — tiny documents rebuild faster than they diff.
const denseIncrMinSegments = 8

// Test hooks: counters of successful incremental rebuilds/renders, so tests
// can assert the incremental path actually ran (a silent fallback to the full
// path would make an equivalence test vacuous).
var (
	denseIncrRebuilds int
	denseIncrRenders  int
)

// denseSegSnapshot captures one top-level segment's identity and the content
// that determines its contribution to the row model and rendering.
type denseSegSnapshot struct {
	seg    RichTextSegment
	isText bool
	text   string        // TextSegment content ("" otherwise)
	style  RichTextStyle // TextSegment style value (zero otherwise)
}

// denseCheckpoint is the dense-iterate state immediately before a segment is
// processed: how many rows exist, whether the last row is still open (an
// inline run the segment would extend) and, when open, that row's pre-segment
// end offset and segment count so it can be restored; wrapWidth is the
// remaining first-line width for wrapping modes.
type denseCheckpoint struct {
	rowStart  int32
	openEnd   int32 // -1 when no open row
	openSegs  int32
	wrapWidth float32
}

func (cp *denseCheckpoint) open() bool { return cp.openEnd >= 0 }

// denseIncrState is the per-RichText bookkeeping for the incremental dense
// build: the snapshot + checkpoints of the last build, the build key they are
// valid for, and the dirty row range for the renderer to consume.
type denseIncrState struct {
	snaps []denseSegSnapshot
	cps   []denseCheckpoint // cps[i] = state before segment i; len == len(snaps)
	endCp denseCheckpoint   // state after the last segment

	// Build key — a mismatch forces a full rebuild.
	width    float32
	wrap     bellui.TextWrap
	innerPad float32

	// Dirty render info (consumed by textRenderer.Refresh). Row indices are in
	// the freshly spliced (new) bounds except dirtyHiOld/dirtyOldObjs, which
	// describe the replaced range of the previous bounds/object list.
	dirty        bool
	dirtyLo      int
	dirtyHiNew   int
	dirtyHiOld   int
	dirtyOldObjs int // object count of the replaced rows in the previous render
	oldRowCount  int // len(bounds) before the splice — renderer sanity check
	removedSegs  []RichTextSegment

	scratch    []rowBoundary     // reusable region rebuild buffer
	scratchCps []denseCheckpoint // reusable region checkpoint buffer
	scratchObj []core.CanvasObject
}

// spliceGap replaces s[lo:hi] with an uninitialised gap of n elements (moving
// the tail as needed) and returns the resized slice. The gap s[lo:lo+n] keeps
// whatever values the move left there — the caller must overwrite it.
func spliceGap[T any](s []T, lo, hi, n int) []T {
	newLen := len(s) - (hi - lo) + n
	if newLen > cap(s) {
		grown := make([]T, newLen)
		copy(grown, s[:lo])
		copy(grown[lo+n:], s[hi:])
		return grown
	}
	tail := s[hi:]
	s = s[:newLen]
	copy(s[lo+n:], tail) // copy handles the overlap in either direction
	return s
}

// denseBuildCtx carries the mutable state of the dense row-bounds build — the
// former iterateSegments closure of updateRowBoundsInner, extracted so the
// incremental rebuild can run the identical logic over a segment subrange with
// injected state.
type denseBuildCtx struct {
	t            *RichText
	th           bellui.Theme
	innerPadding float32
	maxWidth     float32
	fitSize      geo.Size

	bounds    []rowBoundary
	open      bool
	wrapWidth float32
	flat      bool // no RichTextBlock encountered (checkpoint eligibility)
}

// iterate processes segList exactly as the dense build always has, appending
// rows to c.bounds. When record is true (top level only) it appends a
// checkpoint before each segment to cps and returns the extended slice.
func (c *denseBuildCtx) iterate(segList []RichTextSegment, cps []denseCheckpoint, record bool) []denseCheckpoint {
	t := c.t
	for _, seg := range segList {
		if record {
			cps = append(cps, c.checkpoint())
		}
		if parent, ok := seg.(RichTextBlock); ok {
			c.flat = false
			segs := parent.Segments()
			c.iterate(segs, nil, false)
			if len(segs) > 0 && !segs[len(segs)-1].Inline() {
				c.wrapWidth = c.maxWidth
				c.open = false
			}
			continue
		}
		if _, ok := seg.(*TextSegment); !ok {
			// A block (non-inline) segment fills the whole line width, so it
			// cannot share a row with a trailing inline run that was not
			// terminated by a newline (pre-rebrand tracker #207). Close the open inline row first
			// so the block advances to a fresh row instead of overlapping it.
			if !seg.Inline() && c.open {
				c.open = false
				c.wrapWidth = c.maxWidth
			}
			if !c.open {
				c.bounds = append(c.bounds, rowBoundary{segments: []RichTextSegment{seg}})
				c.open = true
			} else {
				last := len(c.bounds) - 1
				c.bounds[last].segments = append(c.bounds[last].segments, seg)
			}

			itemMin := t.cachedSegmentVisual(seg, 0).MinSize()
			if seg.Inline() {
				c.wrapWidth -= itemMin.Width
			} else {
				c.wrapWidth = c.maxWidth
				c.open = false
				c.fitSize.Height -= itemMin.Height + c.th.Size(theme.SizeNameLineSpacing)
			}
			continue
		}
		textSeg := seg.(*TextSegment)
		textStyle := textSeg.Style.TextStyle
		textSize := textSeg.size()
		textSource := textSeg.Style.FontSource

		leftPad := float32(0)
		if textSeg.Style.isBlockquote() {
			leftPad = c.innerPadding * 2
		}
		retBounds, height := lineBounds(textSeg, t.Wrapping, t.Truncation, c.wrapWidth-leftPad, geo.NewSize(c.maxWidth, c.fitSize.Height), func(text []rune) geo.Size {
			m, _ := bellui.MeasureTextFull(string(text), textSize, textStyle, textSource)
			return m
		})
		if c.open {
			if len(retBounds) > 0 {
				last := len(c.bounds) - 1
				c.bounds[last].end = retBounds[0].end // invalidate row ending as we have more content
				c.bounds[last].segments = append(c.bounds[last].segments, seg)
				c.bounds = append(c.bounds, retBounds[1:]...)

				c.fitSize.Height -= height
			}
		} else {
			c.bounds = append(c.bounds, retBounds...)

			c.fitSize.Height -= height
		}
		c.open = true
		if seg.Inline() {
			// wrapWidth only threads into the next segment's lineBounds firstWidth,
			// which lineBounds ignores when wrapping and truncation are both off (it
			// returns the whole-line split unmeasured). Skipping this trailing-width
			// measurement in that mode drops the last O(document) per-keystroke
			// MeasureTextFull for a large wrap-off doc — the dominant residual after
			// the render memo (pre-rebrand tracker #896). It stays exact for wrapping/truncating content.
			if t.Wrapping != bellui.TextWrapOff || t.Truncation != bellui.TextTruncateOff {
				last := c.bounds[len(c.bounds)-1]
				begin := 0
				if len(last.segments) == 1 {
					begin = last.begin
				}
				runes := []rune(textSeg.Text)
				// check ranges - as we resize it can be wrong?
				if begin > len(runes) {
					begin = len(runes)
				}
				end := last.end
				if end > len(runes) {
					end = len(runes)
				}
				text := string(runes[begin:end])
				measured, _ := bellui.MeasureTextFull(text, textSeg.size(), textSeg.Style.TextStyle, textSeg.Style.FontSource)
				lastWidth := measured.Width
				if len(retBounds) == 1 {
					c.wrapWidth -= lastWidth
				} else {
					c.wrapWidth = c.maxWidth - lastWidth
				}
			}
		} else {
			c.open = false
			c.wrapWidth = c.maxWidth
		}
	}
	return cps
}

// checkpoint captures the current iterate state.
func (c *denseBuildCtx) checkpoint() denseCheckpoint {
	cp := denseCheckpoint{rowStart: int32(len(c.bounds)), openEnd: -1, wrapWidth: c.wrapWidth}
	if c.open {
		last := &c.bounds[len(c.bounds)-1]
		cp.openEnd = int32(last.end)
		cp.openSegs = int32(len(last.segments))
	}
	return cp
}

// snapshotSegment records a top-level segment for the next build's diff.
func snapshotSegment(seg RichTextSegment) denseSegSnapshot {
	s := denseSegSnapshot{seg: seg}
	if ts, ok := seg.(*TextSegment); ok {
		s.isText = true
		s.text = ts.Text
		s.style = ts.Style
	}
	return s
}

// segUnchanged reports whether a segment is identical to its snapshot: same
// object, same text (string == short-circuits on the shared backing of an
// untouched segment) and — for text runs — an equal style, so an in-place
// restyle is never treated as unchanged.
func segUnchanged(snap *denseSegSnapshot, seg RichTextSegment) bool {
	if snap.seg != seg {
		return false
	}
	ts, ok := seg.(*TextSegment)
	if !ok {
		return true // atoms are compared by identity only
	}
	return snap.isText && snap.text == ts.Text &&
		richStyleEqual(snap.style, ts.Style) && snap.style.FontSource == ts.Style.FontSource
}

// denseIncrEligible reports whether checkpoints should be recorded for the
// current dense build: a document large enough to pay off, truncation off
// (truncation threads a height budget through the build that a splice cannot
// reproduce) and no folds. Flatness (no RichTextBlock) is confirmed by the
// build itself via denseBuildCtx.flat.
func (t *RichText) denseIncrEligible() bool {
	return t.Truncation == bellui.TextTruncateOff &&
		len(t.Segments) >= denseIncrMinSegments &&
		len(t.hiddenRanges) == 0 && len(t.hiddenLogical) == 0
}

// denseIncrRecord installs the incremental state after a full dense build.
func (t *RichText) denseIncrRecord(ctx *denseBuildCtx, cps []denseCheckpoint, width float32) {
	if !ctx.flat {
		t.denseIncr = nil
		return
	}
	st := t.denseIncr
	if st == nil {
		st = &denseIncrState{}
		t.denseIncr = st
	}
	if cap(st.snaps) >= len(t.Segments) {
		st.snaps = st.snaps[:0]
	} else {
		st.snaps = make([]denseSegSnapshot, 0, len(t.Segments))
	}
	for _, seg := range t.Segments {
		st.snaps = append(st.snaps, snapshotSegment(seg))
	}
	st.cps = append(st.cps[:0], cps...)
	st.endCp = ctx.checkpoint()
	st.width = width
	st.wrap = t.Wrapping
	st.innerPad = ctx.innerPadding
	st.dirty = false
	st.removedSegs = nil
}

// denseIncrEdit carries one incremental rebuild's working state between the
// phase helpers of denseIncrRebuild.
type denseIncrEdit struct {
	st    *denseIncrState
	th    bellui.Theme
	width float32
	segs  []RichTextSegment

	i0, oi, ni int // diff: old[i0:oi) replaced by segs[i0:ni)
	k          int // one past the last new segment re-processed (>= ni after convergence)

	startCp, boundaryCp denseCheckpoint
	dirtyLo, dirtyHiOld int
	oldBounds           []rowBoundary

	ctx *denseBuildCtx
	cps []denseCheckpoint

	mergeSegDelta, mergePreEnd int32
}

// denseIncrGate checks the cheap preconditions for an incremental rebuild.
func (t *RichText) denseIncrGate(width float32) (*denseIncrState, bool) {
	st := t.denseIncr
	if st == nil {
		return nil, false
	}
	if st.dirty {
		// The previous incremental build was never rendered; merging two dirty
		// ranges is not worth the complexity — rebuild fully.
		t.denseIncr = nil
		return nil, false
	}
	if t.Truncation != bellui.TextTruncateOff || t.Wrapping != st.wrap || width != st.width {
		return nil, false
	}
	if len(t.hiddenRanges) > 0 || len(t.hiddenLogical) > 0 {
		return nil, false
	}
	if len(st.snaps) != len(st.cps) || int(st.endCp.rowStart) != len(t.rowBounds) {
		return nil, false
	}
	if t.Theme().Size(theme.SizeNameInnerPadding) != st.innerPad {
		return nil, false
	}
	if len(t.Segments) < denseIncrMinSegments {
		return nil, false
	}
	return st, true
}

// denseIncrDiff locates the changed segment range by identity + content from
// both ends: old[i0:oi) was replaced by segs[i0:ni).
func denseIncrDiff(old []denseSegSnapshot, segs []RichTextSegment) (i0, oi, ni int, ok bool) {
	for i0 < len(old) && i0 < len(segs) && segUnchanged(&old[i0], segs[i0]) {
		i0++
	}
	if i0 == len(old) && i0 == len(segs) {
		return 0, 0, 0, false // version bumped but nothing changed — full path
	}
	oi, ni = len(old), len(segs)
	for oi > i0 && ni > i0 && segUnchanged(&old[oi-1], segs[ni-1]) {
		oi--
		ni--
	}
	return i0, oi, ni, true
}

// denseIncrPrepare resolves the start checkpoint and builds the region rebuild
// context, restoring the open row it extends when there is one.
func (t *RichText) denseIncrPrepare(e *denseIncrEdit) bool {
	st := e.st
	e.startCp = st.endCp // pure append at the end: start from the final state
	if e.i0 < len(st.cps) {
		e.startCp = st.cps[e.i0]
	}
	e.oldBounds = t.rowBounds
	e.dirtyLo = int(e.startCp.rowStart)
	if e.startCp.open() {
		e.dirtyLo--
	}
	if e.dirtyLo < 0 || e.dirtyLo > len(e.oldBounds) || e.startCp.open() && e.dirtyLo >= len(e.oldBounds) {
		return false
	}

	e.ctx = &denseBuildCtx{
		t: t, th: e.th, innerPadding: st.innerPad,
		maxWidth:  e.width - 2*st.innerPad + 2*t.inset.Width,
		fitSize:   geo.NewSize(e.width, float32(2<<20)), // effectively unbounded: truncation is off
		bounds:    st.scratch[:0],
		open:      e.startCp.open(),
		wrapWidth: e.startCp.wrapWidth,
		flat:      true,
	}
	if e.startCp.open() {
		// Restore the open row to its pre-region state; the iterate extends it.
		oldOpen := e.oldBounds[e.dirtyLo]
		if int(e.startCp.openSegs) > len(oldOpen.segments) {
			return false
		}
		segsCopy := append(make([]RichTextSegment, 0, int(e.startCp.openSegs)+2), oldOpen.segments[:e.startCp.openSegs]...)
		e.ctx.bounds = append(e.ctx.bounds, rowBoundary{
			segments:          segsCopy,
			firstSegmentReuse: oldOpen.firstSegmentReuse,
			begin:             oldOpen.begin,
			end:               int(e.startCp.openEnd),
		})
	}
	return true
}

// denseIncrConverge re-runs the dense iterate over the changed range, then
// extends forward past unchanged segments until the iterate state re-converges
// with the old build's checkpoint (wrapping can shift the resync point).
func (t *RichText) denseIncrConverge(e *denseIncrEdit) bool {
	st := e.st
	wrapping := t.Wrapping != bellui.TextWrapOff
	if st.scratchCps == nil {
		st.scratchCps = make([]denseCheckpoint, 0, 8)
	}
	e.cps = e.ctx.iterate(e.segs[e.i0:e.ni], st.scratchCps[:0], true)
	e.k = e.ni // one past the last new segment processed
	for {
		if !e.ctx.flat {
			return false
		}
		endCpOld := st.endCp
		if e.oi < len(st.snaps) {
			endCpOld = st.cps[e.oi]
		}
		converged := e.ctx.open == endCpOld.open() && (!wrapping || !e.ctx.open || e.ctx.wrapWidth == endCpOld.wrapWidth)
		if converged {
			return true
		}
		if e.k >= len(e.segs) || e.oi >= len(st.snaps) {
			return false // never converged — full rebuild
		}
		e.cps = e.ctx.iterate(e.segs[e.k:e.k+1], e.cps, true)
		e.k++
		e.oi++
	}
}

// denseIncrBoundary resolves the old region's row extent and, when the resync
// point sits inside an open row, merges the unchanged tail segments'
// extension of that row into the rebuilt one.
func (t *RichText) denseIncrBoundary(e *denseIncrEdit) bool {
	st := e.st
	e.boundaryCp = st.endCp
	if e.oi < len(st.snaps) {
		e.boundaryCp = st.cps[e.oi]
	}
	e.dirtyHiOld = int(e.boundaryCp.rowStart)
	if e.boundaryCp.open() {
		e.dirtyHiOld-- // the old open row is replaced by the rebuilt one
	}
	if e.dirtyHiOld < e.dirtyLo || e.dirtyHiOld > len(e.oldBounds) {
		return false
	}
	if !e.boundaryCp.open() {
		return true
	}
	if !e.ctx.open || len(e.ctx.bounds) == 0 || e.dirtyHiOld >= len(e.oldBounds) {
		return false
	}
	oldRow := e.oldBounds[e.dirtyHiOld]
	if int(e.boundaryCp.openSegs) > len(oldRow.segments) {
		return false
	}
	last := len(e.ctx.bounds) - 1
	e.mergeSegDelta = int32(len(e.ctx.bounds[last].segments)) - e.boundaryCp.openSegs
	e.mergePreEnd = int32(e.ctx.bounds[last].end)
	// Re-apply the unchanged tail segments' extension of the open row.
	if extra := oldRow.segments[e.boundaryCp.openSegs:]; len(extra) > 0 {
		e.ctx.bounds[last].segments = append(e.ctx.bounds[last].segments, extra...)
		e.ctx.bounds[last].end = oldRow.end
	}
	e.dirtyHiOld++ // consume the old open row as part of the replaced range
	return true
}

// denseIncrCaptureDirty records what the renderer needs before the splice
// moves old rows: the replaced rows' object count and the segments the edit
// removed (for visual-cache eviction).
func denseIncrCaptureDirty(e *denseIncrEdit) {
	st := e.st
	st.dirtyOldObjs = 0
	for r := e.dirtyLo; r < e.dirtyHiOld; r++ {
		st.dirtyOldObjs += len(e.oldBounds[r].segments)
	}
	st.oldRowCount = len(e.oldBounds)
	st.removedSegs = st.removedSegs[:0]
	for j := e.i0; j < e.oi; j++ {
		removedSeg := st.snaps[j].seg
		still := false
		for m := e.i0; m < e.k; m++ {
			if e.segs[m] == removedSeg {
				still = true
				break
			}
		}
		if !still {
			st.removedSegs = append(st.removedSegs, removedSeg)
		}
	}
}

// denseIncrSplice splices the rebuilt rows into rowBounds (preserving the
// per-row height cache when it described the replaced bounds) and patches the
// snapshot + checkpoint bookkeeping for the next edit.
func (t *RichText) denseIncrSplice(e *denseIncrEdit) {
	st := e.st
	region := e.ctx.bounds
	rowDelta := len(region) - (e.dirtyHiOld - e.dirtyLo)
	preserveHeights := len(t.rowHeightCache) == len(e.oldBounds) &&
		t.rowHeightCacheVer == t.rowBoundsComputed &&
		t.rowHeightCacheWidth == e.width &&
		t.rowHeightCacheInnerPad == st.innerPad &&
		t.rowHeightCacheLineSpacing == e.th.Size(theme.SizeNameLineSpacing) &&
		t.rowHeightCacheTextSize == e.th.Size(theme.SizeNameText)

	t.rowBounds = spliceGap(e.oldBounds, e.dirtyLo, e.dirtyHiOld, len(region))
	copy(t.rowBounds[e.dirtyLo:], region)

	if preserveHeights {
		t.rowHeightCache = spliceGap(t.rowHeightCache, e.dirtyLo, e.dirtyHiOld, len(region))
		for r := e.dirtyLo; r < e.dirtyLo+len(region); r++ {
			t.rowHeightCache[r] = rowHeightUnmeasured
		}
		t.rowHeightCacheVer = t.rowBoundsVersion
	}

	// Splice the snapshot + checkpoints. Region checkpoints were recorded
	// relative to the scratch slice; rebase to absolute row indices.
	for j := range e.cps {
		e.cps[j].rowStart += int32(e.dirtyLo)
	}
	regionSegs := e.k - e.i0
	st.snaps = spliceGap(st.snaps, e.i0, e.oi, regionSegs)
	for j := 0; j < regionSegs; j++ {
		st.snaps[e.i0+j] = snapshotSegment(e.segs[e.i0+j])
	}
	oldSegCount := len(st.cps)
	st.cps = spliceGap(st.cps, e.i0, e.oi, regionSegs)
	copy(st.cps[e.i0:], e.cps)
	if rowDelta != 0 {
		for j := e.i0 + regionSegs; j < len(st.cps); j++ {
			st.cps[j].rowStart += int32(rowDelta)
		}
	}
	if e.oi == oldSegCount {
		st.endCp = e.ctx.checkpoint()
		st.endCp.rowStart = int32(len(t.rowBounds))
	} else {
		st.endCp.rowStart += int32(rowDelta)
	}

	t.denseIncrRebaseMerged(e, rowDelta, regionSegs)
	st.scratch = region[:0]
	st.scratchCps = e.cps[:0]
}

// denseIncrRebaseMerged rebases tail checkpoints that still describe the
// merged open row: its leading (rebuilt) part may have a different segment
// count and end than the old row it replaced. Checkpoints reference it while
// their rowStart still equals the merged row's successor row index and they
// are open; the first one's openEnd is an offset into the rebuilt content,
// later ones' openEnd offsets live in unchanged tail segments.
func (t *RichText) denseIncrRebaseMerged(e *denseIncrEdit, rowDelta, regionSegs int) {
	st := e.st
	if !e.boundaryCp.open() || (e.mergeSegDelta == 0 && e.mergePreEnd == e.boundaryCp.openEnd) {
		return
	}
	mergedNext := e.boundaryCp.rowStart + int32(rowDelta)
	for j := e.i0 + regionSegs; j < len(st.cps); j++ {
		if st.cps[j].rowStart != mergedNext || !st.cps[j].open() {
			break
		}
		if j == e.i0+regionSegs {
			st.cps[j].openEnd = e.mergePreEnd
		}
		st.cps[j].openSegs += e.mergeSegDelta
	}
	if st.endCp.rowStart == mergedNext && st.endCp.open() {
		if e.i0+regionSegs == len(st.cps) {
			st.endCp.openEnd = e.mergePreEnd
		}
		st.endCp.openSegs += e.mergeSegDelta
	}
}

// denseIncrRebuild attempts the edit-incremental dense rebuild: diff the
// segment list against the last build's snapshot, re-run the dense iterate
// over only the changed range from its checkpoint, and splice the resulting
// rows into rowBounds. Returns false whenever the full rebuild must run
// instead — any partial state it may have touched is fully overwritten by
// that rebuild.
func (t *RichText) denseIncrRebuild(width float32) bool {
	st, ok := t.denseIncrGate(width)
	if !ok {
		return false
	}
	e := &denseIncrEdit{st: st, th: t.Theme(), width: width, segs: t.Segments}
	e.i0, e.oi, e.ni, ok = denseIncrDiff(st.snaps, e.segs)
	if !ok {
		return false
	}
	if !t.denseIncrPrepare(e) {
		t.denseIncr = nil
		return false
	}
	if !t.denseIncrConverge(e) || !t.denseIncrBoundary(e) {
		t.denseIncr = nil
		return false
	}

	denseIncrCaptureDirty(e)
	dirtyHiNew := e.dirtyLo + len(e.ctx.bounds)
	t.denseIncrSplice(e)

	if len(st.snaps) != len(e.segs) || len(st.cps) != len(e.segs) || int(st.endCp.rowStart) != len(t.rowBounds) {
		// Bookkeeping drift — never serve from it again; the spliced bounds are
		// still internally consistent, but force a full render + rebuild next.
		t.denseIncr = nil
		t.rowBoundsCacheStamp(width)
		return true
	}

	st.dirty = true
	st.dirtyLo = e.dirtyLo
	st.dirtyHiNew = dirtyHiNew
	st.dirtyHiOld = e.dirtyHiOld
	denseIncrRebuilds++

	// Mirror the full build's cache stamping and post-processing.
	t.rowBoundsCacheStamp(width)
	return true
}

// rowBoundsCacheStamp applies the dense build's post-build stamping, shared by
// the full and incremental paths.
func (t *RichText) rowBoundsCacheStamp(width float32) {
	t.varHeight = t.varHeightEligible()
	t.prefixHeightDirty = true
	t.rowBoundsComputed = t.rowBoundsVersion
	t.rowBoundsWidth = width
	t.rowBoundsTruncation = t.Truncation
	t.rowBoundsWrapping = t.Wrapping
	t.buildRowToLine()
	t.applyHiddenRanges()
}

// refreshIncremental renders only the dirty row range produced by
// denseIncrRebuild: it regenerates the dirty rows' objects, splices them into
// the previous object list, evicts removed segments' visuals and patches the
// layout. Returns false to fall back to the full Refresh; the dirty range is
// consumed either way (the full path rebuilds everything from the new bounds).
func (r *textRenderer) refreshIncremental() bool {
	t := r.obj
	st := t.denseIncr
	if st == nil || !st.dirty {
		return false
	}
	st.dirty = false
	removed := st.removedSegs
	st.removedSegs = st.removedSegs[:0]

	if t.canVirtualize() || t.scr != nil || t.RowHighlighter != nil {
		return false
	}
	if t.isSelectable() && t.selection != nil {
		return false
	}
	bounds := t.rowBounds
	objs := r.origObjs
	if len(objs) == 0 || len(t.rowYPos) != st.oldRowCount || len(t.rowHeight) != st.oldRowCount {
		return false
	}
	rendered := r.Objects()
	if len(rendered) != len(objs) {
		return false
	}
	if st.dirtyLo > st.dirtyHiNew || st.dirtyHiNew > len(bounds) {
		return false
	}

	// Object index of the first dirty row, and consistency of the old list.
	prefixObjs := 0
	for row := 0; row < st.dirtyLo; row++ {
		prefixObjs += len(bounds[row].segments)
	}
	tailObjs := 0
	for row := st.dirtyHiNew; row < len(bounds); row++ {
		tailObjs += len(bounds[row].segments)
	}
	if prefixObjs+st.dirtyOldObjs+tailObjs != len(objs) {
		return false
	}

	// Regenerate the dirty rows' objects — the same per-row body as the full
	// Refresh (minus the virtualised/lazy branches, gated off above).
	newObjs := r.denseIncrRowObjects(bounds, st.dirtyLo, st.dirtyHiNew, st.scratchObj[:0])

	// Splice into the previous object list — both the renderer's slice and
	// origObjs (the full path keeps them as two arrays with equal content).
	objDelta := len(newObjs) - st.dirtyOldObjs
	if objDelta == 0 {
		for j, obj := range newObjs {
			if objs[prefixObjs+j] != obj {
				objs[prefixObjs+j] = obj
				rendered[prefixObjs+j] = obj
			}
		}
	} else {
		spliced := make([]core.CanvasObject, 0, len(objs)+objDelta)
		spliced = append(spliced, objs[:prefixObjs]...)
		spliced = append(spliced, newObjs...)
		spliced = append(spliced, objs[prefixObjs+st.dirtyOldObjs:]...)
		r.SetObjects(spliced)
		orig := make([]core.CanvasObject, len(spliced))
		copy(orig, spliced)
		r.origObjs = orig
	}
	st.scratchObj = newObjs[:0]

	// Evict removed segments' visuals directly instead of the O(document)
	// live-set walk of cleanVisualCache.
	if len(removed) > 0 {
		t.visualMu.Lock()
		for _, seg := range removed {
			delete(t.visualCache, seg)
		}
		t.visualMu.Unlock()
	}

	if !r.layoutIncremental(st) {
		r.Layout(t.Size())
	}
	denseIncrRenders++

	canvas.Refresh(t.super())
	return true
}

// denseIncrRowObjects regenerates the visual objects for rows [lo, hi) — the
// same per-row body as the full Refresh (minus the virtualised/lazy branches,
// which the incremental render gates off).
func (r *textRenderer) denseIncrRowObjects(bounds []rowBoundary, lo, hi int, buf []core.CanvasObject) []core.CanvasObject {
	t := r.obj
	var lastSeg *TextSegment
	var lastRunes []rune
	for row := lo; row < hi; row++ {
		bound := bounds[row]
		for i, seg := range bound.segments {
			if _, ok := seg.(*TextSegment); !ok {
				obj := t.cachedSegmentVisual(seg, 0)
				seg.Update(obj)
				buf = append(buf, obj)
				continue
			}
			reuse := 0
			if i == 0 {
				reuse = bound.firstSegmentReuse
			}
			obj := t.cachedSegmentVisual(seg, reuse)
			seg.Update(obj)
			txt := obj.(*canvas.Text)
			textSeg := seg.(*TextSegment)

			var runes []rune
			if textSeg == lastSeg {
				runes = lastRunes
			} else {
				runes = []rune(textSeg.Text)
				lastSeg, lastRunes = textSeg, runes
			}
			runeLen := len(runes)
			begin := min(bound.begin, runeLen)
			end := min(bound.end, runeLen)

			if i == 0 {
				if len(bound.segments) == 1 {
					txt.Text = string(runes[begin:end])
				} else {
					txt.Text = string(runes[begin:])
				}
			} else if i == len(bound.segments)-1 && len(bound.segments) > 1 {
				txt.Text = string(runes[:end])
			}
			if bound.ellipsis && i == len(bound.segments)-1 {
				txt.Text = txt.Text + "…"
			}
			if concealed(seg) {
				txt.Text = strings.Repeat(passwordChar, len(runes))
			}
			buf = append(buf, txt)
		}
	}
	return buf
}

// layoutIncremental re-lays-out only the dirty rows. It applies when the edit
// left the geometry of every following row unchanged (same row count and an
// identical Y seam) — the common typing case; otherwise it reports false and
// the caller runs the full Layout.
func (r *textRenderer) layoutIncremental(st *denseIncrState) bool {
	t := r.obj
	bounds := t.rowBounds
	if st.dirtyHiNew-st.dirtyLo != st.dirtyHiOld-st.dirtyLo {
		return false // row count changed — every following row moves
	}
	if st.dirtyLo >= len(t.rowYPos) || st.dirtyHiNew > len(bounds) {
		return false
	}
	// A non-inline prefix row's trailing line spacing depends on the (possibly
	// changed) first dirty row — recomputing that seam needs the old bounds,
	// so hand it to the full Layout.
	if st.dirtyLo > 0 {
		prev := bounds[st.dirtyLo-1].segments
		if len(prev) == 0 || !prev[len(prev)-1].Inline() {
			return false
		}
	}

	th := t.Theme()
	innerPadding := th.Size(theme.SizeNameInnerPadding)
	lineSpacing := th.Size(theme.SizeNameLineSpacing)
	xInset := innerPadding - t.inset.Width
	size := t.Size()
	lineWidth := size.Width - xInset*2

	objs := r.origObjs
	oi := 0
	for row := 0; row < st.dirtyLo; row++ {
		oi += len(bounds[row].segments)
	}

	yPos := t.rowYPos[st.dirtyLo]
	for row := st.dirtyLo; row < st.dirtyHiNew; row++ {
		bound := bounds[row]
		rowStartY := yPos

		if t.rowHidden(row) {
			return false // folds are gated off upstream; be safe
		}
		rowObjStart := oi
		consumed, rh := r.layoutOneRow(bound, objs, oi, yPos, xInset, lineWidth, lineSpacing)
		oi += consumed
		yPos += rh

		t.rowYPos[row] = rowStartY
		t.rowHeight[row] = yPos - rowStartY

		if c := oi - rowObjStart; !segmentsVolatile(objs, rowObjStart, c) {
			t.ensureRowHeightCache(lineWidth, th)
			if row >= 0 && row < len(t.rowHeightCache) {
				t.rowHeightCache[row] = t.rowHeight[row]
			}
		}

		lastSeg := bound.segments[len(bound.segments)-1]
		if !lastSeg.Inline() && row < len(bounds)-1 && bounds[row+1].segments[0] != lastSeg {
			yPos += lineSpacing
		}
	}

	// The seam must land exactly where the old layout had the first unchanged
	// row, or every following row would need repositioning — full Layout then.
	if st.dirtyHiNew < len(t.rowYPos) && yPos != t.rowYPos[st.dirtyHiNew] {
		return false
	}
	return true
}
