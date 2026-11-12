package common

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// A pop-up that leaves a band of the canvas to the content, as a drawer that
// opens below a header does (#1171), shares the keyboard with the content in
// that band (#1172): a field in the header takes the focus and the keys, and
// Tab visits the band's focusables, then the pop-up's. Nothing the pop-up
// covers is reached.

// bandOf returns the top keyboard-owning overlay's band test, if it leaves a
// band to the content.
func (c *Canvas) bandOf() (core.CanvasObject, driver.InputPassThroughAt, bool) {
	top, mgr := c.overlays.TopFocus()
	if mgr == nil {
		return nil, nil, false
	}
	band, ok := top.(driver.InputPassThroughAt)
	if !ok || !band.PassesInputThroughAt(geo.Position{}) {
		return nil, nil, false // it covers the canvas from its top edge
	}
	return top, band, true
}

// inBand reports whether the centre of the object at pos, of size, lies in
// the band.
func inBand(band driver.InputPassThroughAt, pos geo.Position, size geo.Size) bool {
	return band.PassesInputThroughAt(pos.AddXY(size.Width/2, size.Height/2))
}

// focusAcrossBand focuses obj, an object in the content's band, while the
// pop-up has the keyboard: the content takes it, and the pop-up's focused
// item loses the focus. The way back is leaveBandFor.
func (c *Canvas) focusAcrossBand(obj event.Focusable) bool {
	top, band, ok := c.bandOf()
	if !ok || obj == nil {
		return false
	}
	_, topMgr := c.overlays.TopFocus()
	o, isObj := obj.(core.CanvasObject)
	if !isObj || c.contentFocusMgr == nil {
		return false
	}
	pos := driver.AbsolutePositionForObject(o, c.ObjectTrees())
	if !inBand(band, pos, o.Size()) || !c.contentFocusMgr.Focus(obj) {
		return false
	}
	topMgr.Focus(nil)
	c.bandFocus = top
	return true
}

// stepBand moves the focus dir places along the band's focusables followed by
// the pop-up's, and reports whether there was a band to step through.
func (c *Canvas) stepBand(dir int) bool {
	_, band, ok := c.bandOf()
	if !ok {
		return false
	}
	_, topMgr := c.overlays.TopFocus()
	var chain []event.Focusable
	if c.contentFocusMgr != nil {
		for _, e := range c.contentFocusMgr.Chain() {
			if inBand(band, e.Pos, e.Size) {
				chain = append(chain, e.Focusable)
			}
		}
	}
	for _, e := range topMgr.Chain() {
		chain = append(chain, e.Focusable)
	}
	if len(chain) == 0 {
		return true
	}
	next := chain[0]
	if dir < 0 {
		next = chain[len(chain)-1]
	}
	if cur := c.Focused(); cur != nil {
		for i, f := range chain {
			if f == cur {
				next = chain[(i+dir+len(chain))%len(chain)]
				break
			}
		}
	}
	c.Focus(next)
	return true
}

// leaveBandFor gives the keyboard back to the pop-up when the focus moves
// from the band to anything outside it: an item in the pop-up, or content the
// pop-up covers. The field in the band loses the focus. Covered content is
// then focused as content under a modal pop-up always was, holding the focus
// for when the pop-up goes, with no key reaching it meanwhile.
func (c *Canvas) leaveBandFor(obj event.Focusable) {
	top, band, ok := c.bandOf()
	if !ok || c.bandFocus != top {
		return
	}
	o, isObj := obj.(core.CanvasObject)
	if !isObj || inBand(band, driver.AbsolutePositionForObject(o, c.ObjectTrees()), o.Size()) {
		return
	}
	c.bandFocus = nil
	c.contentFocusMgr.Focus(nil)
}
