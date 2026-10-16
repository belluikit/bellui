// Package accessibility builds the platform-neutral semantic tree that
// screen-reader backends expose to assistive technologies (pre-rebrand tracker #484).
//
// The semantic layer itself — [core.Accessible], the role vocabulary, state
// flags and actions — lives in the public core package and is always
// compiled. This package adds the geometry: it walks a canvas's rendered
// object trees (content, canvas-drawn menu, overlays) and produces
// [Node] values carrying absolute bounds, label, role and the optional
// value/state/action properties, nested under their closest accessible
// ancestor. Platform backends (NSAccessibility, UIA, ...) translate these
// nodes into their native element trees; none of the code here talks to a
// platform API, which is the seam that keeps backends swappable.
//
// The walker is only compiled under the `accessibility` build tag, mirroring
// upstream Bellui's opt-in mechanism; without the tag this package contributes
// only the [Enabled] constant (false) so untagged builds carry no extra code.
//
// The walker descends both containers and widget renderers, so accessible
// widgets inside scroll areas or composite widgets are found. Descendants of
// a node whose role reports ChildrenPresentational (WAI-ARIA's "children
// presentational": buttons, tabs, sliders, ...) are dropped — those widgets
// are one semantic unit and their internal accessible widgets only paint
// them. Text-bearing roles (labels, text inputs, select) use the selective
// variant instead (NonInteractiveChildrenPresentational, pre-rebrand tracker #522): the internal
// widgets painting their value are dropped, but embedded interactive
// controls — an Entry's ActionItem button, hyperlinks inside rich text —
// stay in the tree, hoisted past any dropped textual wrapper.
//
// # Ordering
//
// Nodes are emitted in tree order — the order children appear in their
// container, parents before children, content roots before menu before
// overlays. This is the logical reading order of the UI and is what
// assistive technologies use for sequential navigation. Under a
// right-to-left layout direction (global, per-canvas, or a
// container.DirectionScope subtree) the emitted order does not change:
// mirroring only moves objects, so bounds reflect the mirrored positions
// (read live from the laid-out tree) while the node order remains the
// reading order. Backends that want visual-order navigation can sort by the
// reported bounds.
package accessibility
