# Bellui X — dag

Directed-acyclic-graph widgets for Bellui: an interactive node/edge canvas
(`dagwidget`) over a toolkit-free graph model (`dag`).

```
go get github.com/belluikit/bellui/x/dag
```

## Packages

| Package | Contents |
|---|---|
| `dagwidget` | the interactive DAG widget (nodes, edges, ports, groups, annotations, layout) |
| `dag` | the pure-data graph model — no toolkit dependency |
| `dag/dagpb` | generated wire schema for the graph model (messages only, no services) |

## Persistence

A graph round-trips through the textual PXF format — the same container the
toolkit uses for its own on-disk stores (settings, preferences, app manifests):

```go
data, err := dag.MarshalGraph(g)   // []byte of PXF text
g2, err := dag.UnmarshalGraph(data)
```

`dag.GraphToProto` / `dag.GraphFromProto` expose the underlying `*dagpb.Graph`
mapping for applications that want a different container (binary proto, JSON)
or their own transport.

Loading is **tolerant**: `GraphFromProto` inserts through the validating
mutation methods and drops malformed entries rather than failing the whole
load, so a corrupt payload cannot smuggle in a cycle or a dangling reference.
Only an undecodable payload makes `UnmarshalGraph` return an error.

### Regenerating the schema

`dag/dagpb/dag.proto` is the source of truth; run `sh hack/genproto.sh` from the
bellui repo root (needs `buf`, `protoc-gen-go` and `gofumpt`). The schema
carries **messages only** — that is what keeps this module free of grpc; its
only codegen dependencies are `google.golang.org/protobuf` and
`trendvidia/protowire-go`, both already bellui dependencies.

## Provenance

Extracted from `trendvidia/fydag` per epic trendvidia/fyne#732. Only `dagwidget`
and its data dependency `dag` moved here; the fydag repo remains a working PoC
and keeps its gRPC/api/app/monitor subsystem.

Serialization initially stayed behind, because fydag's graph messages lived in
one monolithic `.proto` alongside its gRPC services (belluikit/bellui#3). The
data-model messages have since been carved out into `dag/dagpb` under a
bellui-owned proto package, restoring all four functions with no grpc in the
dependency graph. Field numbers are inherited verbatim from the fydag schema so
payloads stay wire-compatible; the proto *package* name deliberately differs
(`bellui.dag.v1` vs `fydag`) so an application linking both modules does not
register two messages under the same fully-qualified name.
