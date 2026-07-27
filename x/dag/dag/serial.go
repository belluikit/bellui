package dag

import (
	"github.com/trendvidia/protowire-go/encoding/pxf"

	pb "github.com/belluikit/bellui/x/dag/dag/dagpb"
)

// MarshalGraph serializes a graph to PXF text via its proto message. PXF is the
// same textual container the toolkit uses for its own on-disk stores (settings,
// preferences, app manifests).
func MarshalGraph(g *Graph) ([]byte, error) {
	return pxf.Marshal(GraphToProto(g))
}

// UnmarshalGraph deserializes a graph from PXF text. Malformed graph entries
// are dropped by GraphFromProto rather than failing the load; only a payload
// that is not decodable at all returns an error.
func UnmarshalGraph(data []byte) (*Graph, error) {
	var pg pb.Graph
	if err := pxf.Unmarshal(data, &pg); err != nil {
		return nil, err
	}
	return GraphFromProto(&pg), nil
}
