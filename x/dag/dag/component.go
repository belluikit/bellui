package dag

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// StreamType describes the data type flowing through a port.
// Two ports are compatible if their StreamTypes are compatible.
type StreamType struct {
	Name   string          `json:"name"`             // e.g. "text/line", "json/object", "bytes"
	Schema json.RawMessage `json:"schema,omitempty"` // optional schema for structured types
}

// StreamTypeAny matches any stream type.
var StreamTypeAny = StreamType{Name: "*"}

// Compatible returns true if data can flow from src to dst.
// Rules: "*" (any) matches everything; otherwise names must match exactly.
func (src StreamType) Compatible(dst StreamType) bool {
	if src.Name == "*" || dst.Name == "*" {
		return true
	}
	return src.Name == dst.Name
}

// PropertyKind describes the type of a component property value.
type PropertyKind int

const (
	PropertyString PropertyKind = iota
	PropertyInt
	PropertyFloat
	PropertyBool
	PropertyEnum
)

// PropertyDef defines a configurable property on a component.
type PropertyDef struct {
	Key         string       `json:"key"`
	Label       string       `json:"label"`
	Kind        PropertyKind `json:"kind"`
	Default     any          `json:"default,omitempty"`
	Required    bool         `json:"required,omitempty"`
	Description string       `json:"description,omitempty"`
	EnumValues  []string     `json:"enumValues,omitempty"` // for PropertyEnum
}

// PortDef defines an input or output port on a component.
type PortDef struct {
	Name        string        `json:"name"`
	Direction   PortDirection `json:"direction"`
	Stream      StreamType    `json:"stream"`
	Required    bool          `json:"required,omitempty"`
	Description string        `json:"description,omitempty"`
}

// Component defines a reusable component type that can be instantiated as nodes.
type Component struct {
	ID          string        `json:"id"`       // unique type identifier, e.g. "csv_source"
	Name        string        `json:"name"`     // display name, e.g. "CSV Source"
	Category    string        `json:"category"` // grouping, e.g. "Source", "Transform", "Sink"
	Description string        `json:"description"`
	Inputs      []PortDef     `json:"inputs"`
	Outputs     []PortDef     `json:"outputs"`
	Properties  []PropertyDef `json:"properties"`
}

// NewNode creates a graph Node from this component definition at the given position.
func (c *Component) NewNode(nodeID string, pos [2]float32) *Node {
	n := &Node{
		ID:       nodeID,
		Label:    c.Name,
		TypeTag:  c.ID,
		Position: pos,
	}
	for _, pd := range c.Inputs {
		n.Inputs = append(n.Inputs, Port{
			Name:      pd.Name,
			Direction: PortInput,
			TypeTag:   pd.Stream.Name,
		})
	}
	for _, pd := range c.Outputs {
		n.Outputs = append(n.Outputs, Port{
			Name:      pd.Name,
			Direction: PortOutput,
			TypeTag:   pd.Stream.Name,
		})
	}
	// Populate default property values into metadata
	props := make(map[string]any)
	for _, p := range c.Properties {
		if p.Default != nil {
			props[p.Key] = p.Default
		}
	}
	if len(props) > 0 {
		data, _ := json.Marshal(props)
		n.Metadata = data
	}
	return n
}

// Registry holds all available component types and validates stream compatibility.
type Registry struct {
	mu         sync.RWMutex
	components map[string]*Component
	// CompatFunc is an optional custom compatibility function.
	// If nil, StreamType.Compatible is used.
	CompatFunc func(src, dst StreamType) bool
}

// NewRegistry creates an empty component registry.
func NewRegistry() *Registry {
	return &Registry{
		components: make(map[string]*Component),
	}
}

// Register adds a component to the registry.
func (r *Registry) Register(c *Component) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.components[c.ID]; exists {
		return fmt.Errorf("component %q already registered", c.ID)
	}
	r.components[c.ID] = c
	return nil
}

// MustRegister is like Register but panics on error.
func (r *Registry) MustRegister(c *Component) {
	if err := r.Register(c); err != nil {
		panic(err)
	}
}

// Get returns a component by ID, or nil if not found.
func (r *Registry) Get(id string) *Component {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.components[id]
}

// All returns all registered components.
func (r *Registry) All() []*Component {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Component, 0, len(r.components))
	for _, c := range r.components {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Categories returns the sorted list of distinct categories.
func (r *Registry) Categories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]bool)
	for _, c := range r.components {
		seen[c.Category] = true
	}
	out := make([]string, 0, len(seen))
	for cat := range seen {
		out = append(out, cat)
	}
	sort.Strings(out)
	return out
}

// ByCategory returns components in the given category.
func (r *Registry) ByCategory(category string) []*Component {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Component
	for _, c := range r.components {
		if c.Category == category {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// StreamsCompatible checks whether an output port's stream type is compatible
// with an input port's stream type, using the custom CompatFunc if set.
func (r *Registry) StreamsCompatible(src, dst StreamType) bool {
	r.mu.RLock()
	fn := r.CompatFunc
	r.mu.RUnlock()
	if fn != nil {
		return fn(src, dst)
	}
	return src.Compatible(dst)
}

// PortStreamType looks up the StreamType for a port on a node,
// using the component definition from the registry.
func (r *Registry) PortStreamType(n *Node, portName string, dir PortDirection) (StreamType, bool) {
	c := r.Get(n.TypeTag)
	if c == nil {
		// Fallback: use the port's TypeTag as the stream name
		ports := n.Inputs
		if dir == PortOutput {
			ports = n.Outputs
		}
		for _, p := range ports {
			if p.Name == portName {
				return StreamType{Name: p.TypeTag}, true
			}
		}
		return StreamType{}, false
	}
	defs := c.Inputs
	if dir == PortOutput {
		defs = c.Outputs
	}
	for _, pd := range defs {
		if pd.Name == portName {
			return pd.Stream, true
		}
	}
	return StreamType{}, false
}
