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

## Provenance

Extracted from `trendvidia/fydag` per epic trendvidia/fyne#732. Only `dagwidget`
and its data dependency `dag` moved here; the fydag repo remains a working PoC
and keeps its gRPC/proto/api/app/monitor subsystem.

**Protobuf serialization stayed behind.** `dag/proto.go` and `dag/serial.go`
(`GraphToProto`/`GraphFromProto`/`MarshalGraph`/`UnmarshalGraph`) depended on
fydag's generated gRPC package, which belongs to the PoC's serving subsystem.
Dropping them keeps this module free of grpc/protobuf — `dagwidget` never used
them, only the graph types. Consumers needing persistence should encode the
exported `dag` types themselves.
