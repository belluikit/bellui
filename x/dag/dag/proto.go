package dag

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/belluikit/bellui/x/dag/dag/dagpb"
)

// GraphToProto converts a graph to its proto message. It is the single source
// of truth for the dag↔proto mapping, shared by PXF file serialization and by
// any transport an application layers on top of the schema.
func GraphToProto(g *Graph) *pb.Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pg := &pb.Graph{Id: g.ID, Name: g.Name, Metadata: metadataToProto(g.Metadata)}
	for _, n := range g.Nodes {
		pn := &pb.Node{
			Id:        n.ID,
			Label:     n.Label,
			TypeTag:   n.TypeTag,
			PositionX: n.Position[0],
			PositionY: n.Position[1],
			SizeW:     n.Size[0],
			SizeH:     n.Size[1],
			Metadata:  metadataToProto(n.Metadata),
		}
		for _, p := range n.Inputs {
			pn.Inputs = append(pn.Inputs, &pb.Port{
				Name:      p.Name,
				Direction: pb.PortDirection_PORT_DIRECTION_INPUT,
				TypeTag:   p.TypeTag,
			})
		}
		for _, p := range n.Outputs {
			pn.Outputs = append(pn.Outputs, &pb.Port{
				Name:      p.Name,
				Direction: pb.PortDirection_PORT_DIRECTION_OUTPUT,
				TypeTag:   p.TypeTag,
			})
		}
		pg.Nodes = append(pg.Nodes, pn)
	}
	for _, e := range g.Edges {
		pg.Edges = append(pg.Edges, &pb.Edge{
			Id:     e.ID,
			Source: &pb.PortID{NodeId: e.Source.NodeID, Name: e.Source.Name},
			Target: &pb.PortID{NodeId: e.Target.NodeID, Name: e.Target.Name},
		})
	}
	for _, grp := range g.Groups {
		pg.Groups = append(pg.Groups, &pb.Group{
			Id:        grp.ID,
			Label:     grp.Label,
			Members:   append([]string(nil), grp.Members...),
			PositionX: grp.Position[0],
			PositionY: grp.Position[1],
			SizeW:     grp.Size[0],
			SizeH:     grp.Size[1],
			Collapsed: grp.Collapsed,
			Color:     grp.Color,
		})
	}
	for _, ge := range g.GroupEdges {
		pg.GroupEdges = append(pg.GroupEdges, &pb.GroupEdge{
			Id:     ge.ID,
			Source: ge.Source,
			Target: ge.Target,
		})
	}
	for _, le := range g.LinkEdges {
		pg.LinkEdges = append(pg.LinkEdges, &pb.LinkEdge{
			Id:     le.ID,
			Source: le.Source,
			Target: le.Target,
			Label:  le.Label,
		})
	}
	for _, a := range g.Annotations {
		pg.Annotations = append(pg.Annotations, &pb.Annotation{
			Id:         a.ID,
			Text:       a.Text,
			PositionX:  a.Position[0],
			PositionY:  a.Position[1],
			SizeW:      a.Size[0],
			SizeH:      a.Size[1],
			AnchorKind: anchorKindToProto(a.AnchorKind),
			AnchorId:   a.AnchorID,
		})
	}
	return pg
}

// GraphFromProto converts a proto message to a graph. Entries are inserted
// through the validating mutation methods; malformed entries are dropped
// rather than failing the whole load, so a corrupt payload cannot smuggle
// in a cycle or a dangling reference.
func GraphFromProto(pg *pb.Graph) *Graph {
	if pg == nil {
		return NewGraph("", "")
	}
	g := NewGraph(pg.Id, pg.Name)
	g.Metadata = metadataFromProto(pg.Metadata)
	for _, pn := range pg.Nodes {
		n := &Node{
			ID:       pn.Id,
			Label:    pn.Label,
			TypeTag:  pn.TypeTag,
			Position: [2]float32{pn.PositionX, pn.PositionY},
			Size:     [2]float32{pn.SizeW, pn.SizeH},
			Metadata: metadataFromProto(pn.Metadata),
		}
		for _, pp := range pn.Inputs {
			n.Inputs = append(n.Inputs, Port{
				Name:      pp.Name,
				Direction: PortInput,
				TypeTag:   pp.TypeTag,
			})
		}
		for _, pp := range pn.Outputs {
			n.Outputs = append(n.Outputs, Port{
				Name:      pp.Name,
				Direction: PortOutput,
				TypeTag:   pp.TypeTag,
			})
		}
		g.AddNode(n)
	}
	// Groups before edges and annotations so membership and anchors resolve;
	// group edges after both groups and node edges so cycle checks see the
	// full picture.
	for _, pgr := range pg.Groups {
		g.AddGroup(&Group{
			ID:        pgr.Id,
			Label:     pgr.Label,
			Members:   append([]string(nil), pgr.Members...),
			Position:  [2]float32{pgr.PositionX, pgr.PositionY},
			Size:      [2]float32{pgr.SizeW, pgr.SizeH},
			Collapsed: pgr.Collapsed,
			Color:     pgr.Color,
		})
	}
	for _, pe := range pg.Edges {
		g.AddEdge(&Edge{
			ID:     pe.Id,
			Source: PortID{NodeID: pe.Source.GetNodeId(), Name: pe.Source.GetName()},
			Target: PortID{NodeID: pe.Target.GetNodeId(), Name: pe.Target.GetName()},
		})
	}
	for _, pge := range pg.GroupEdges {
		g.AddGroupEdge(&GroupEdge{
			ID:     pge.Id,
			Source: pge.Source,
			Target: pge.Target,
		})
	}
	for _, ple := range pg.LinkEdges {
		g.AddLinkEdge(&LinkEdge{
			ID:     ple.Id,
			Source: ple.Source,
			Target: ple.Target,
			Label:  ple.Label,
		})
	}
	for _, pa := range pg.Annotations {
		g.AddAnnotation(&Annotation{
			ID:         pa.Id,
			Text:       pa.Text,
			Position:   [2]float32{pa.PositionX, pa.PositionY},
			Size:       [2]float32{pa.SizeW, pa.SizeH},
			AnchorKind: anchorKindFromProto(pa.AnchorKind),
			AnchorID:   pa.AnchorId,
		})
	}
	return g
}

// metadataToProto converts raw JSON metadata to a proto Struct. Invalid or
// empty metadata becomes nil (dropped), matching the load path's tolerance.
func metadataToProto(md json.RawMessage) *structpb.Struct {
	if len(md) == 0 {
		return nil
	}
	s := &structpb.Struct{}
	if err := protojson.Unmarshal(md, s); err != nil {
		return nil
	}
	return s
}

// metadataFromProto converts a proto Struct back to raw JSON metadata.
func metadataFromProto(s *structpb.Struct) json.RawMessage {
	if s == nil || len(s.Fields) == 0 {
		return nil
	}
	data, err := protojson.Marshal(s)
	if err != nil {
		return nil
	}
	return data
}

func anchorKindToProto(k AnchorKind) pb.AnchorKind {
	switch k {
	case AnchorNode:
		return pb.AnchorKind_ANCHOR_KIND_NODE
	case AnchorEdge:
		return pb.AnchorKind_ANCHOR_KIND_EDGE
	case AnchorGroup:
		return pb.AnchorKind_ANCHOR_KIND_GROUP
	case AnchorLinkEdge:
		return pb.AnchorKind_ANCHOR_KIND_LINK_EDGE
	default:
		return pb.AnchorKind_ANCHOR_KIND_UNSPECIFIED
	}
}

func anchorKindFromProto(k pb.AnchorKind) AnchorKind {
	switch k {
	case pb.AnchorKind_ANCHOR_KIND_NODE:
		return AnchorNode
	case pb.AnchorKind_ANCHOR_KIND_EDGE:
		return AnchorEdge
	case pb.AnchorKind_ANCHOR_KIND_GROUP:
		return AnchorGroup
	case pb.AnchorKind_ANCHOR_KIND_LINK_EDGE:
		return AnchorLinkEdge
	default:
		return AnchorNone
	}
}
