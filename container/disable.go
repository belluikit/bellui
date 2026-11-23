package container

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
)

// The containers here (AppTabs, DocTabs, InnerWindow) draw their own disabled
// look: their tab strip or window chrome draws the disabled colours, and the
// widgets they disable inside their content draw theirs. Dimming the whole
// subtree would dim those twice (core.DisabledLook, #1223). App content that
// cannot be disabled, a plain Label or a link, is dimmed by the paint walk
// instead, as is everything under a disabled container that has no disabled
// look of its own (#1393).

func (*AppTabs) DrawsDisabledLook() bool     { return true }
func (*DocTabs) DrawsDisabledLook() bool     { return true }
func (*InnerWindow) DrawsDisabledLook() bool { return true }

// DrawsDisabledLook reports that a Clip leaves the look to others: it only
// clips and draws nothing of its own, so under a disabled container the
// widgets inside it are dimmed or not by their own looks (#1393), and a
// table header's text under a Clip is not dimmed a second time.
//
// Since: 1.3
func (*Clip) DrawsDisabledLook() bool { return true }

// A tab of a disabled TabItem draws its label and icon in the disabled
// colour. Before #1218 the walk dimmed it as well, to about 14%.
func (*tabButton) DrawsDisabledLook() bool { return true }

var (
	_ core.Disableable  = (*AppTabs)(nil)
	_ core.Disableable  = (*DocTabs)(nil)
	_ core.Disableable  = (*InnerWindow)(nil)
	_ core.DisabledLook = (*AppTabs)(nil)
	_ core.DisabledLook = (*DocTabs)(nil)
	_ core.DisabledLook = (*InnerWindow)(nil)
)

// disableCascade disables the widgets inside a disabled container, as HTML's
// <fieldset disabled> does (owner, belluikit/player#641, 2026-11-15), and
// remembers which ones it disabled, so that enabling the container brings
// back only those: a widget the app disabled itself stays disabled. It
// follows ButtonGroup and BottomAppBar's FAB.
//
// sync is run whenever the container's state or its content may have changed:
// on Disable and Enable, when its items or content are set, and each time its
// renderer refreshes. So a widget added while the container is disabled is
// disabled, one taken out is released, and one the app enabled again while
// the container is disabled is disabled again. A change deep inside the app's
// content (a widget appended to a box inside a tab) is seen at the
// container's next refresh, not at once: the toolkit has no notice of it.
type disableCascade struct {
	// disabled holds what sync switched off, for it to switch back on.
	disabled map[core.Disableable]struct{}
}

// sync brings the widgets inside roots to the container's state: when on, it
// disables each enabled one and records it; it enables again the recorded
// ones no longer inside, and, when off, all the recorded ones.
func (c *disableCascade) sync(on bool, roots ...core.CanvasObject) {
	if !on {
		for d := range c.disabled {
			d.Enable()
		}
		c.disabled = nil
		return
	}

	found := map[core.Disableable]struct{}{}
	var inside []core.Disableable
	for _, root := range roots {
		eachDisableable(root, func(d core.Disableable) {
			if _, seen := found[d]; !seen {
				found[d] = struct{}{}
				inside = append(inside, d)
			}
		})
	}
	for d := range c.disabled {
		if _, ok := found[d]; !ok {
			delete(c.disabled, d) // taken out while disabled: released
			d.Enable()
		}
	}
	for _, d := range inside {
		if d.Disabled() {
			continue // the app's own, or ours already
		}
		if c.disabled == nil {
			c.disabled = map[core.Disableable]struct{}{}
		}
		c.disabled[d] = struct{}{}
		d.Disable()
	}
}

// eachDisableable calls fn for each core.Disableable in the tree at o, and
// does not look inside one: a disableable widget disables its own parts, and
// the parts' state is its business (a date picker's month button disabled at
// the last month must not be enabled by the container).
//
// It descends through containers and through other widgets' renderers,
// building a renderer not built yet, as the first paint would: a container
// can be disabled before it is drawn (Player disables while it builds the
// tree), and a button inside a card would otherwise be missed.
func eachDisableable(o core.CanvasObject, fn func(core.Disableable)) {
	switch t := o.(type) {
	case nil:
		return
	case core.Disableable:
		fn(t)
	case *bellui.Container:
		for _, child := range t.Objects {
			eachDisableable(child, fn)
		}
	case core.Widget:
		if r := cache.Renderer(t); r != nil {
			for _, child := range r.Objects() {
				eachDisableable(child, fn)
			}
		}
	}
}

// tabsContent returns every tab's content, the selected one and the others.
func tabsContent(t baseTabs) []core.CanvasObject {
	items := t.items()
	content := make([]core.CanvasObject, 0, len(items))
	for _, item := range items {
		if item != nil && item.Content != nil {
			content = append(content, item.Content)
		}
	}
	return content
}
