package bellui

// Regenerate the repo's protobuf bindings (settingspb, prefspb, belluiapppb) from
// their .proto sources. Consolidated on buf (#905): the directive runs
// hack/genproto.sh, which invokes `buf generate` (buf.yaml + buf.gen.yaml) and
// then `gofumpt -w` on the generated *.pb.go, since buf's protoc-gen-go output
// is not gofumpt-clean. Requires buf, protoc-gen-go and gofumpt on PATH.
//
//go:generate sh hack/genproto.sh
