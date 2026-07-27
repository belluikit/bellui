package bellui

// Regenerate the repo's protobuf bindings (settingspb, prefspb, belluiapppb,
// x/dag's dagpb) from their .proto sources. Consolidated on buf (#905): the
// directive runs hack/genproto.sh, which invokes `buf generate` in every
// directory owning a buf.yaml/buf.gen.yaml pair — the root, cmd/bellui and the
// separate x/dag module — and then `gofumpt -w` on the generated *.pb.go, since
// buf's protoc-gen-go output is not gofumpt-clean. Because the script sweeps the
// whole repo it is the single entry point for every module, including those
// `go generate ./...` cannot reach from here. Requires buf, protoc-gen-go and
// gofumpt on PATH.
//
//go:generate sh hack/genproto.sh
