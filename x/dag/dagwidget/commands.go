package dagwidget

import (
	"encoding/json"

	"github.com/belluikit/bellui/x/dag/dag"
)

// Command represents an undoable graph mutation.
type Command interface {
	Execute()
	Undo()
}

// Failer is optionally implemented by commands whose Execute validates
// through the model and can be rejected. A command reporting Failed after
// Execute mutated nothing: the stack does not record it, and its Undo is
// a no-op.
type Failer interface {
	Failed() bool
}

func failed(cmd Command) bool {
	f, ok := cmd.(Failer)
	return ok && f.Failed()
}

// CommandStack manages undo/redo history.
type CommandStack struct {
	undoStack []Command
	redoStack []Command

	// OnChange fires after every stack operation that mutated the graph: a
	// recorded Do, PushDone, and each successful Undo/Redo. DAGCanvas wires
	// it to OnGraphChanged, making the stack the single notification source
	// for command-driven mutations.
	OnChange func()
}

// NewCommandStack creates an empty command stack.
func NewCommandStack() *CommandStack {
	return &CommandStack{}
}

func (s *CommandStack) notify() {
	if s.OnChange != nil {
		s.OnChange()
	}
}

// Do executes a command and pushes it onto the undo stack. A command that
// reports itself Failed mutated nothing and is not recorded.
func (s *CommandStack) Do(cmd Command) {
	cmd.Execute()
	if failed(cmd) {
		return
	}
	s.undoStack = append(s.undoStack, cmd)
	s.redoStack = nil // clear redo on new action
	s.notify()
}

// Undo reverses the last command.
func (s *CommandStack) Undo() bool {
	if len(s.undoStack) == 0 {
		return false
	}
	cmd := s.undoStack[len(s.undoStack)-1]
	s.undoStack = s.undoStack[:len(s.undoStack)-1]
	cmd.Undo()
	s.redoStack = append(s.redoStack, cmd)
	s.notify()
	return true
}

// Redo re-applies the last undone command.
func (s *CommandStack) Redo() bool {
	if len(s.redoStack) == 0 {
		return false
	}
	cmd := s.redoStack[len(s.redoStack)-1]
	s.redoStack = s.redoStack[:len(s.redoStack)-1]
	cmd.Execute()
	s.undoStack = append(s.undoStack, cmd)
	s.notify()
	return true
}

// CanUndo returns true if there are commands to undo.
func (s *CommandStack) CanUndo() bool { return len(s.undoStack) > 0 }

// CanRedo returns true if there are commands to redo.
func (s *CommandStack) CanRedo() bool { return len(s.redoStack) > 0 }

// PushDone records an already-executed command for undo without calling Execute.
// Used when the action already happened (e.g., drag-to-move updates position live).
func (s *CommandStack) PushDone(cmd Command) {
	s.undoStack = append(s.undoStack, cmd)
	s.redoStack = nil
	s.notify()
}

// UndoAction is a no-arg wrapper for use as a toolbar callback.
func (s *CommandStack) UndoAction() { s.Undo() }

// RedoAction is a no-arg wrapper for use as a toolbar callback.
func (s *CommandStack) RedoAction() { s.Redo() }

// CompoundCommand groups several commands into a single undo/redo step.
// It deliberately does not implement Failer: a compound is always built by a
// gesture that pre-validated its members, so member failure is unreachable —
// a compound of possibly-failing commands would be recorded even if every
// member was rejected.
type CompoundCommand struct {
	Commands []Command
}

func (c *CompoundCommand) Execute() {
	for _, cmd := range c.Commands {
		cmd.Execute()
	}
}

func (c *CompoundCommand) Undo() {
	for i := len(c.Commands) - 1; i >= 0; i-- {
		c.Commands[i].Undo()
	}
}

// AddNodeCommand adds a node to the canvas.
type AddNodeCommand struct {
	Canvas *DAGCanvas
	Node   *dag.Node

	rejected bool
	err      error
}

func (c *AddNodeCommand) Execute() {
	c.err = c.Canvas.addNode(c.Node)
	c.rejected = c.err != nil
}
func (c *AddNodeCommand) Failed() bool { return c.rejected }
func (c *AddNodeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.removeNode(c.Node.ID)
}

// RemoveNodeCommand removes a node and its connected edges. The model also
// cascades connected link edges, group membership, and anchored annotations;
// all are snapshotted on Execute and restored on Undo.
type RemoveNodeCommand struct {
	Canvas       *DAGCanvas
	Node         *dag.Node
	RemovedEdges []*dag.Edge

	oldGroupID         string
	removedLinkEdges   []*dag.LinkEdge
	removedAnnotations []*dag.Annotation
	rejected           bool
}

func (c *RemoveNodeCommand) Execute() {
	g := c.Canvas.Graph
	c.RemovedEdges = nil
	c.removedAnnotations = g.AnnotationsFor(dag.AnchorNode, c.Node.ID)
	for _, e := range g.EdgesForNode(c.Node.ID) {
		c.RemovedEdges = append(c.RemovedEdges, e)
		c.removedAnnotations = append(c.removedAnnotations, g.AnnotationsFor(dag.AnchorEdge, e.ID)...)
	}
	c.removedLinkEdges = g.LinkEdgesForNode(c.Node.ID)
	for _, le := range c.removedLinkEdges {
		c.removedAnnotations = append(c.removedAnnotations, g.AnnotationsFor(dag.AnchorLinkEdge, le.ID)...)
	}
	c.oldGroupID = g.GroupOf(c.Node.ID)
	c.rejected = c.Canvas.removeNode(c.Node.ID) != nil
}

func (c *RemoveNodeCommand) Failed() bool { return c.rejected }

func (c *RemoveNodeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.addNode(c.Node)
	if c.oldGroupID != "" {
		c.Canvas.Graph.AssignNodeToGroup(c.Node.ID, c.oldGroupID)
	}
	for _, e := range c.RemovedEdges {
		c.Canvas.Graph.AddEdge(e)
		ew := NewEdgeWidget(e.ID, e.Source, e.Target, c.Canvas)
		c.Canvas.edges[e.ID] = ew
	}
	for _, le := range c.removedLinkEdges {
		c.Canvas.addLinkEdge(le)
	}
	for _, a := range c.removedAnnotations {
		c.Canvas.addAnnotation(a)
	}
	c.Canvas.syncPortConnections()
	c.Canvas.Refresh()
}

// AddEdgeCommand adds an edge. The edge goes through the model's validating
// AddEdge; if it is rejected the command becomes a no-op, on Undo too.
type AddEdgeCommand struct {
	Canvas *DAGCanvas
	Edge   *dag.Edge

	rejected bool
}

func (c *AddEdgeCommand) Execute() {
	if err := c.Canvas.Graph.AddEdge(c.Edge); err != nil {
		c.rejected = true
		return
	}
	c.rejected = false
	ew := NewEdgeWidget(c.Edge.ID, c.Edge.Source, c.Edge.Target, c.Canvas)
	c.Canvas.edges[c.Edge.ID] = ew
	c.Canvas.syncPortConnections()
	c.Canvas.Refresh()
}

func (c *AddEdgeCommand) Failed() bool { return c.rejected }

func (c *AddEdgeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.Graph.RemoveEdge(c.Edge.ID)
	delete(c.Canvas.edges, c.Edge.ID)
	c.Canvas.reconcileWidgets()
	c.Canvas.syncPortConnections()
	c.Canvas.Refresh()
}

// RemoveEdgeCommand removes an edge. Annotations anchored to the edge
// cascade at the model level and are restored on Undo.
type RemoveEdgeCommand struct {
	Canvas *DAGCanvas
	Edge   *dag.Edge

	removedAnnotations []*dag.Annotation
	rejected           bool
}

func (c *RemoveEdgeCommand) Execute() {
	c.removedAnnotations = c.Canvas.Graph.AnnotationsFor(dag.AnchorEdge, c.Edge.ID)
	if err := c.Canvas.Graph.RemoveEdge(c.Edge.ID); err != nil {
		c.rejected = true
		return
	}
	c.rejected = false
	delete(c.Canvas.edges, c.Edge.ID)
	c.Canvas.reconcileWidgets()
	c.Canvas.syncPortConnections()
	c.Canvas.Refresh()
}

func (c *RemoveEdgeCommand) Failed() bool { return c.rejected }

func (c *RemoveEdgeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.Graph.AddEdge(c.Edge)
	ew := NewEdgeWidget(c.Edge.ID, c.Edge.Source, c.Edge.Target, c.Canvas)
	c.Canvas.edges[c.Edge.ID] = ew
	for _, a := range c.removedAnnotations {
		c.Canvas.addAnnotation(a)
	}
	c.Canvas.syncPortConnections()
	c.Canvas.Refresh()
}

// MoveNodeCommand records a node position change.
type MoveNodeCommand struct {
	Canvas *DAGCanvas
	NodeID string
	OldPos [2]float32
	NewPos [2]float32
}

func (c *MoveNodeCommand) Execute() {
	if n := c.Canvas.Graph.Nodes[c.NodeID]; n != nil {
		n.Position = c.NewPos
	}
	c.Canvas.Refresh()
}

func (c *MoveNodeCommand) Undo() {
	if n := c.Canvas.Graph.Nodes[c.NodeID]; n != nil {
		n.Position = c.OldPos
	}
	c.Canvas.Refresh()
}

// ResizeNodeCommand records a node geometry change from a resize-handle drag.
// Position is included because left/top handles move the origin to keep the
// opposite border anchored.
type ResizeNodeCommand struct {
	Canvas  *DAGCanvas
	NodeID  string
	OldPos  [2]float32
	NewPos  [2]float32
	OldSize [2]float32
	NewSize [2]float32
}

func (c *ResizeNodeCommand) Execute() {
	if n := c.Canvas.Graph.Nodes[c.NodeID]; n != nil {
		n.Position = c.NewPos
		n.Size = c.NewSize
	}
	c.Canvas.Refresh()
}

func (c *ResizeNodeCommand) Undo() {
	if n := c.Canvas.Graph.Nodes[c.NodeID]; n != nil {
		n.Position = c.OldPos
		n.Size = c.OldSize
	}
	c.Canvas.Refresh()
}

// AddGroupCommand adds a group to the canvas.
type AddGroupCommand struct {
	Canvas *DAGCanvas
	Group  *dag.Group

	rejected bool
}

func (c *AddGroupCommand) Execute()     { c.rejected = c.Canvas.addGroup(c.Group) != nil }
func (c *AddGroupCommand) Failed() bool { return c.rejected }
func (c *AddGroupCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.removeGroup(c.Group.ID)
}

// RemoveGroupCommand removes a group; the group edges and annotations the
// model cascades are snapshotted on Execute and restored on Undo. Members
// simply become ungrouped, and re-adding the group (which keeps its Members
// slice) restores them.
type RemoveGroupCommand struct {
	Canvas *DAGCanvas
	Group  *dag.Group

	removedEdges       []*dag.GroupEdge
	removedAnnotations []*dag.Annotation
	rejected           bool
}

func (c *RemoveGroupCommand) Execute() {
	edges, anns, err := c.Canvas.Graph.RemoveGroup(c.Group.ID)
	if err != nil {
		c.rejected = true
		return
	}
	c.rejected = false
	c.removedEdges = edges
	c.removedAnnotations = anns
	delete(c.Canvas.groups, c.Group.ID)
	c.Canvas.reconcileWidgets()
	c.Canvas.Refresh()
}

func (c *RemoveGroupCommand) Failed() bool { return c.rejected }

func (c *RemoveGroupCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.addGroup(c.Group)
	for _, ge := range c.removedEdges {
		c.Canvas.addGroupEdge(ge)
	}
	for _, a := range c.removedAnnotations {
		c.Canvas.addAnnotation(a)
	}
	c.Canvas.Refresh()
}

// MoveGroupCommand records a title-bar drag: the group moved together with
// its member nodes.
type MoveGroupCommand struct {
	Canvas    *DAGCanvas
	GroupID   string
	OldPos    [2]float32
	NewPos    [2]float32
	MemberOld map[string][2]float32
	MemberNew map[string][2]float32
}

func (c *MoveGroupCommand) Execute() {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Position = c.NewPos
	}
	for id, pos := range c.MemberNew {
		if n := c.Canvas.Graph.Nodes[id]; n != nil {
			n.Position = pos
		}
	}
	c.Canvas.Refresh()
}

func (c *MoveGroupCommand) Undo() {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Position = c.OldPos
	}
	for id, pos := range c.MemberOld {
		if n := c.Canvas.Graph.Nodes[id]; n != nil {
			n.Position = pos
		}
	}
	c.Canvas.Refresh()
}

// ResizeGroupCommand records a group geometry change from a resize-handle drag.
type ResizeGroupCommand struct {
	Canvas  *DAGCanvas
	GroupID string
	OldPos  [2]float32
	NewPos  [2]float32
	OldSize [2]float32
	NewSize [2]float32
}

func (c *ResizeGroupCommand) Execute() {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Position = c.NewPos
		g.Size = c.NewSize
	}
	c.Canvas.Refresh()
}

func (c *ResizeGroupCommand) Undo() {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Position = c.OldPos
		g.Size = c.OldSize
	}
	c.Canvas.Refresh()
}

// ToggleGroupCollapseCommand switches a group between expanded and collapsed.
type ToggleGroupCollapseCommand struct {
	Canvas    *DAGCanvas
	GroupID   string
	Collapsed bool // state after Execute
}

func (c *ToggleGroupCollapseCommand) Execute() { c.setCollapsed(c.Collapsed) }
func (c *ToggleGroupCollapseCommand) Undo()    { c.setCollapsed(!c.Collapsed) }

func (c *ToggleGroupCollapseCommand) setCollapsed(collapsed bool) {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Collapsed = collapsed
	}
	c.Canvas.Refresh()
}

// AssignNodeToGroupCommand moves a node between groups (or in/out of one);
// empty group IDs mean ungrouped.
type AssignNodeToGroupCommand struct {
	Canvas     *DAGCanvas
	NodeID     string
	OldGroupID string
	NewGroupID string

	rejected bool
}

func (c *AssignNodeToGroupCommand) Execute() {
	c.rejected = c.Canvas.Graph.AssignNodeToGroup(c.NodeID, c.NewGroupID) != nil
	c.Canvas.Refresh()
}

func (c *AssignNodeToGroupCommand) Failed() bool { return c.rejected }

func (c *AssignNodeToGroupCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.Graph.AssignNodeToGroup(c.NodeID, c.OldGroupID)
	c.Canvas.Refresh()
}

// AddGroupEdgeCommand adds a group dependency edge. If the model rejects the
// edge (cycle, duplicate), the command fails and mutates nothing.
type AddGroupEdgeCommand struct {
	Canvas *DAGCanvas
	Edge   *dag.GroupEdge

	rejected bool
}

func (c *AddGroupEdgeCommand) Execute()     { c.rejected = c.Canvas.addGroupEdge(c.Edge) != nil }
func (c *AddGroupEdgeCommand) Failed() bool { return c.rejected }
func (c *AddGroupEdgeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.removeGroupEdge(c.Edge.ID)
}

// RemoveGroupEdgeCommand removes a group dependency edge.
type RemoveGroupEdgeCommand struct {
	Canvas *DAGCanvas
	Edge   *dag.GroupEdge

	rejected bool
}

func (c *RemoveGroupEdgeCommand) Execute()     { c.rejected = c.Canvas.removeGroupEdge(c.Edge.ID) != nil }
func (c *RemoveGroupEdgeCommand) Failed() bool { return c.rejected }
func (c *RemoveGroupEdgeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.addGroupEdge(c.Edge)
}

// AddLinkEdgeCommand adds a link edge. If the model rejects the edge
// (duplicate ID, missing endpoint, self-loop), the command fails and
// mutates nothing.
type AddLinkEdgeCommand struct {
	Canvas *DAGCanvas
	Edge   *dag.LinkEdge

	rejected bool
}

func (c *AddLinkEdgeCommand) Execute()     { c.rejected = c.Canvas.addLinkEdge(c.Edge) != nil }
func (c *AddLinkEdgeCommand) Failed() bool { return c.rejected }
func (c *AddLinkEdgeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.removeLinkEdge(c.Edge.ID)
}

// RemoveLinkEdgeCommand removes a link edge. Annotations anchored to it
// cascade at the model level and are restored on Undo.
type RemoveLinkEdgeCommand struct {
	Canvas *DAGCanvas
	Edge   *dag.LinkEdge

	removedAnnotations []*dag.Annotation
	rejected           bool
}

func (c *RemoveLinkEdgeCommand) Execute() {
	c.removedAnnotations = c.Canvas.Graph.AnnotationsFor(dag.AnchorLinkEdge, c.Edge.ID)
	c.rejected = c.Canvas.removeLinkEdge(c.Edge.ID) != nil
}

func (c *RemoveLinkEdgeCommand) Failed() bool { return c.rejected }

func (c *RemoveLinkEdgeCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.addLinkEdge(c.Edge)
	for _, a := range c.removedAnnotations {
		c.Canvas.addAnnotation(a)
	}
}

// SetLinkEdgeLabelCommand changes a link edge's midpoint label.
type SetLinkEdgeLabelCommand struct {
	Canvas *DAGCanvas
	EdgeID string
	Old    string
	New    string
	OnDone func()
}

func (c *SetLinkEdgeLabelCommand) Execute() { c.setLabel(c.New) }
func (c *SetLinkEdgeLabelCommand) Undo()    { c.setLabel(c.Old) }

func (c *SetLinkEdgeLabelCommand) setLabel(label string) {
	if le := c.Canvas.Graph.LinkEdges[c.EdgeID]; le != nil {
		le.Label = label
	}
	c.Canvas.Refresh()
	if c.OnDone != nil {
		c.OnDone()
	}
}

// AddAnnotationCommand adds an annotation.
type AddAnnotationCommand struct {
	Canvas     *DAGCanvas
	Annotation *dag.Annotation

	rejected bool
}

func (c *AddAnnotationCommand) Execute()     { c.rejected = c.Canvas.addAnnotation(c.Annotation) != nil }
func (c *AddAnnotationCommand) Failed() bool { return c.rejected }
func (c *AddAnnotationCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.removeAnnotation(c.Annotation.ID)
}

// RemoveAnnotationCommand removes an annotation. The Annotation pointer holds
// the full state (text, geometry, anchor) needed to restore it.
type RemoveAnnotationCommand struct {
	Canvas     *DAGCanvas
	Annotation *dag.Annotation

	rejected bool
}

func (c *RemoveAnnotationCommand) Execute() {
	c.rejected = c.Canvas.removeAnnotation(c.Annotation.ID) != nil
}
func (c *RemoveAnnotationCommand) Failed() bool { return c.rejected }
func (c *RemoveAnnotationCommand) Undo() {
	if c.rejected {
		return
	}
	c.Canvas.addAnnotation(c.Annotation)
}

// MoveAnnotationCommand records an annotation position change.
type MoveAnnotationCommand struct {
	Canvas       *DAGCanvas
	AnnotationID string
	OldPos       [2]float32
	NewPos       [2]float32
}

func (c *MoveAnnotationCommand) Execute() {
	if a := c.Canvas.Graph.Annotations[c.AnnotationID]; a != nil {
		a.Position = c.NewPos
	}
	c.Canvas.Refresh()
}

func (c *MoveAnnotationCommand) Undo() {
	if a := c.Canvas.Graph.Annotations[c.AnnotationID]; a != nil {
		a.Position = c.OldPos
	}
	c.Canvas.Refresh()
}

// ResizeAnnotationCommand records an annotation geometry change.
type ResizeAnnotationCommand struct {
	Canvas       *DAGCanvas
	AnnotationID string
	OldPos       [2]float32
	NewPos       [2]float32
	OldSize      [2]float32
	NewSize      [2]float32
}

func (c *ResizeAnnotationCommand) Execute() {
	if a := c.Canvas.Graph.Annotations[c.AnnotationID]; a != nil {
		a.Position = c.NewPos
		a.Size = c.NewSize
	}
	c.Canvas.Refresh()
}

func (c *ResizeAnnotationCommand) Undo() {
	if a := c.Canvas.Graph.Annotations[c.AnnotationID]; a != nil {
		a.Position = c.OldPos
		a.Size = c.OldSize
	}
	c.Canvas.Refresh()
}

// SetAnnotationTextCommand changes an annotation's text.
type SetAnnotationTextCommand struct {
	Canvas       *DAGCanvas
	AnnotationID string
	OldText      string
	NewText      string
	// OnDone is called after execute/undo so the UI can refresh.
	OnDone func()
}

func (c *SetAnnotationTextCommand) Execute() { c.setText(c.NewText) }
func (c *SetAnnotationTextCommand) Undo()    { c.setText(c.OldText) }

func (c *SetAnnotationTextCommand) setText(text string) {
	if a := c.Canvas.Graph.Annotations[c.AnnotationID]; a != nil {
		a.Text = text
	}
	c.Canvas.Refresh()
	if c.OnDone != nil {
		c.OnDone()
	}
}

// SetAnnotationAnchorCommand re-anchors an annotation. Anchor changes go
// through remove+add so the model validates the new target; a rejected
// target fails the command (nothing changed, nothing recorded).
type SetAnnotationAnchorCommand struct {
	Canvas       *DAGCanvas
	AnnotationID string
	OldKind      dag.AnchorKind
	NewKind      dag.AnchorKind
	OldAnchorID  string
	NewAnchorID  string
	OnDone       func()

	rejected bool
}

func (c *SetAnnotationAnchorCommand) Execute()     { c.rejected = !c.setAnchor(c.NewKind, c.NewAnchorID) }
func (c *SetAnnotationAnchorCommand) Failed() bool { return c.rejected }
func (c *SetAnnotationAnchorCommand) Undo() {
	if c.rejected {
		return
	}
	c.setAnchor(c.OldKind, c.OldAnchorID)
}

// setAnchor applies the anchor change and reports whether it took effect.
// OnDone fires either way so the properties form snaps back on rejection.
func (c *SetAnnotationAnchorCommand) setAnchor(kind dag.AnchorKind, anchorID string) (ok bool) {
	defer func() {
		if c.OnDone != nil {
			c.OnDone()
		}
	}()
	a := c.Canvas.Graph.Annotations[c.AnnotationID]
	if a == nil {
		return false
	}
	updated := *a
	updated.AnchorKind = kind
	updated.AnchorID = anchorID
	if err := c.Canvas.Graph.RemoveAnnotation(a.ID); err != nil {
		return false
	}
	ok = true
	if err := c.Canvas.Graph.AddAnnotation(&updated); err != nil {
		// Invalid target — restore the previous annotation unchanged.
		c.Canvas.Graph.AddAnnotation(a)
		ok = false
	}
	c.Canvas.reconcileWidgets()
	c.Canvas.syncFromGraph()
	c.Canvas.Refresh()
	return ok
}

// SetGroupLabelCommand changes a group's label.
type SetGroupLabelCommand struct {
	Canvas  *DAGCanvas
	GroupID string
	Old     string
	New     string
	OnDone  func()
}

func (c *SetGroupLabelCommand) Execute() { c.setLabel(c.New) }
func (c *SetGroupLabelCommand) Undo()    { c.setLabel(c.Old) }

func (c *SetGroupLabelCommand) setLabel(label string) {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Label = label
	}
	c.Canvas.Refresh()
	if c.OnDone != nil {
		c.OnDone()
	}
}

// SetGroupColorCommand changes a group's tint color.
type SetGroupColorCommand struct {
	Canvas  *DAGCanvas
	GroupID string
	Old     string
	New     string
	OnDone  func()
}

func (c *SetGroupColorCommand) Execute() { c.setColor(c.New) }
func (c *SetGroupColorCommand) Undo()    { c.setColor(c.Old) }

func (c *SetGroupColorCommand) setColor(col string) {
	if g := c.Canvas.Graph.Groups[c.GroupID]; g != nil {
		g.Color = col
	}
	c.Canvas.Refresh()
	if c.OnDone != nil {
		c.OnDone()
	}
}

// SetPropertyCommand changes a single property on a node's metadata.
type SetPropertyCommand struct {
	Node     *dag.Node
	Key      string
	OldValue any
	NewValue any
	// OnDone is called after execute/undo so the UI can refresh.
	OnDone func()
}

func (c *SetPropertyCommand) Execute() {
	setMetadataKey(c.Node, c.Key, c.NewValue)
	if c.OnDone != nil {
		c.OnDone()
	}
}

func (c *SetPropertyCommand) Undo() {
	setMetadataKey(c.Node, c.Key, c.OldValue)
	if c.OnDone != nil {
		c.OnDone()
	}
}

func setMetadataKey(n *dag.Node, key string, value any) {
	props := make(map[string]any)
	if n.Metadata != nil {
		json.Unmarshal(n.Metadata, &props)
	}
	if value == nil {
		delete(props, key)
	} else {
		props[key] = value
	}
	data, _ := json.Marshal(props)
	n.Metadata = data
}
