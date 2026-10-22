package dag

import (
	"testing"
)

func makeRegistry() *Registry {
	r := NewRegistry()
	r.MustRegister(&Component{
		ID:       "csv_source",
		Name:     "CSV Source",
		Category: "Source",
		Outputs: []PortDef{
			{Name: "rows", Direction: PortOutput, Stream: StreamType{Name: "json/object"}},
		},
		Properties: []PropertyDef{
			{Key: "path", Label: "File Path", Kind: PropertyString, Required: true},
			{Key: "delimiter", Label: "Delimiter", Kind: PropertyString, Default: ","},
		},
	})
	r.MustRegister(&Component{
		ID:       "filter",
		Name:     "Filter",
		Category: "Transform",
		Inputs: []PortDef{
			{Name: "in", Direction: PortInput, Stream: StreamType{Name: "json/object"}},
		},
		Outputs: []PortDef{
			{Name: "out", Direction: PortOutput, Stream: StreamType{Name: "json/object"}},
		},
		Properties: []PropertyDef{
			{Key: "expression", Label: "Expression", Kind: PropertyString, Required: true},
		},
	})
	r.MustRegister(&Component{
		ID:       "console_sink",
		Name:     "Console Sink",
		Category: "Sink",
		Inputs: []PortDef{
			{Name: "in", Direction: PortInput, Stream: StreamType{Name: "*"}},
		},
	})
	r.MustRegister(&Component{
		ID:       "image_source",
		Name:     "Image Source",
		Category: "Source",
		Outputs: []PortDef{
			{Name: "frames", Direction: PortOutput, Stream: StreamType{Name: "image/frame"}},
		},
	})
	return r
}

func TestRegistryGetAndAll(t *testing.T) {
	r := makeRegistry()
	if c := r.Get("csv_source"); c == nil || c.Name != "CSV Source" {
		t.Fatal("expected to find csv_source")
	}
	if c := r.Get("nonexistent"); c != nil {
		t.Fatal("expected nil for nonexistent component")
	}
	all := r.All()
	if len(all) != 4 {
		t.Fatalf("expected 4 components, got %d", len(all))
	}
	// Should be sorted by category then name
	if all[0].Category != "Sink" || all[1].Category != "Source" {
		t.Fatalf("unexpected ordering: %s/%s, %s/%s", all[0].Category, all[0].Name, all[1].Category, all[1].Name)
	}
}

func TestRegistryCategories(t *testing.T) {
	r := makeRegistry()
	cats := r.Categories()
	if len(cats) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(cats))
	}
	// Sorted alphabetically
	if cats[0] != "Sink" || cats[1] != "Source" || cats[2] != "Transform" {
		t.Fatalf("unexpected categories: %v", cats)
	}
}

func TestRegistryByCategory(t *testing.T) {
	r := makeRegistry()
	sources := r.ByCategory("Source")
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
}

func TestStreamTypeCompatibility(t *testing.T) {
	json := StreamType{Name: "json/object"}
	image := StreamType{Name: "image/frame"}
	any := StreamTypeAny

	if !json.Compatible(json) {
		t.Fatal("same types should be compatible")
	}
	if json.Compatible(image) {
		t.Fatal("json and image should not be compatible")
	}
	if !any.Compatible(json) {
		t.Fatal("any should be compatible with json")
	}
	if !json.Compatible(any) {
		t.Fatal("json should be compatible with any")
	}
}

func TestComponentNewNode(t *testing.T) {
	r := makeRegistry()
	c := r.Get("csv_source")
	n := c.NewNode("n1", [2]float32{100, 200})

	if n.ID != "n1" || n.Label != "CSV Source" || n.TypeTag != "csv_source" {
		t.Fatalf("unexpected node: %+v", n)
	}
	if len(n.Outputs) != 1 || n.Outputs[0].TypeTag != "json/object" {
		t.Fatalf("unexpected outputs: %+v", n.Outputs)
	}
	if n.Metadata == nil {
		t.Fatal("expected default properties in metadata")
	}
}

func TestAddEdgeStreamCompatibility(t *testing.T) {
	r := makeRegistry()
	g := NewGraph("test", "Test")
	g.Registry = r

	csvSource := r.Get("csv_source")
	filter := r.Get("filter")
	imageSrc := r.Get("image_source")

	g.AddNode(csvSource.NewNode("csv", [2]float32{0, 0}))
	g.AddNode(filter.NewNode("flt", [2]float32{200, 0}))
	g.AddNode(imageSrc.NewNode("img", [2]float32{0, 200}))

	// Compatible: csv (json/object) -> filter (json/object)
	err := g.AddEdge(&Edge{ID: "e1", Source: PortID{"csv", "rows"}, Target: PortID{"flt", "in"}})
	if err != nil {
		t.Fatalf("compatible edge should succeed: %v", err)
	}

	// Incompatible: image (image/frame) -> filter (json/object)
	// The target is already connected, so this fails either way; e3 below
	// isolates the type check with a fresh filter.
	if err := g.AddEdge(&Edge{ID: "e2", Source: PortID{"img", "frames"}, Target: PortID{"flt", "in"}}); err == nil {
		t.Fatal("incompatible edge to a connected input should fail")
	}
	g.AddNode(r.Get("filter").NewNode("flt2", [2]float32{200, 200}))
	err = g.AddEdge(&Edge{ID: "e3", Source: PortID{"img", "frames"}, Target: PortID{"flt2", "in"}})
	if err == nil {
		t.Fatal("incompatible edge should fail")
	}
}

func TestAddEdgeStreamAnyAcceptsAll(t *testing.T) {
	r := makeRegistry()
	g := NewGraph("test", "Test")
	g.Registry = r

	imageSrc := r.Get("image_source")
	consoleSink := r.Get("console_sink")

	g.AddNode(imageSrc.NewNode("img", [2]float32{0, 0}))
	g.AddNode(consoleSink.NewNode("con", [2]float32{200, 0}))

	// console_sink accepts "*" so any stream should connect
	err := g.AddEdge(&Edge{ID: "e1", Source: PortID{"img", "frames"}, Target: PortID{"con", "in"}})
	if err != nil {
		t.Fatalf("any-type sink should accept any source: %v", err)
	}
}

func TestRegistryDuplicateRegister(t *testing.T) {
	r := NewRegistry()
	c := &Component{ID: "test", Name: "Test", Category: "Test"}
	if err := r.Register(c); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(c); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}
