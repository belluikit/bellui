#!/bin/sh
# Regenerate every protobuf binding in the repo from its .proto source.
#
# Single reproducible entry point for proto codegen (pre-rebrand tracker #905): runs `buf generate`
# in every directory that owns a buf.yaml/buf.gen.yaml pair, then `gofumpt -w`
# on the output, because buf's protoc-gen-go groups imports differently from
# gofumpt (the repo's formatter, enforced in CI) so the generated files are not
# gofumpt-clean straight out of buf.
#
# Requires `buf`, `protoc-gen-go` and `gofumpt` on PATH. Invoked via
# `go generate ./...` (see generate.go) or directly: `sh hack/genproto.sh`.
set -eu

# Directories with their own buf config. The repo root covers internal/; the
# bellui CLI and the x/dag module are separate buf modules (and, for x/dag, a
# separate Go module) so each carries its own pair.
buf_dirs=". cmd/bellui x/dag"

# Resolve the repo root from this script's location so it can be run from
# anywhere (go generate runs it from the repo root already).
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

for tool in buf gofumpt; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "hack/genproto.sh: required tool '$tool' not found on PATH" >&2
		exit 1
	fi
done

for dir in $buf_dirs; do
	(cd "$dir" && buf generate)
done

# gofumpt every generated file so the committed tree matches CI's formatter.
# gofumpt needs more than one pass to converge on buf's protoc-gen-go output
# (reordering the import block on the first pass only exposes the blank-line
# rule between a multi-line var(...) group and the next declaration on the
# next), so format until `gofumpt -l` reports nothing left to change.
files=$(find . -name '*.pb.go')
for _ in 1 2 3 4 5; do
	gofumpt -w $files
	[ -z "$(gofumpt -l $files)" ] && break
done

remaining=$(gofumpt -l $files)
if [ -n "$remaining" ]; then
	echo "hack/genproto.sh: gofumpt did not converge on:" >&2
	echo "$remaining" >&2
	exit 1
fi
