# Browser harness for the x/editor widget under GOOS=js/GOARCH=wasm.
# See README.md for the manual test checklist.

GOROOT  := $(shell go env GOROOT)
PORT    ?= 8080

# Build tags. Empty = WebGPU (the default backend). TAGS=gl selects the WebGL
# fallback painter — build/serve both to cover the two web backends.
TAGS    ?=
TAGFLAG := $(if $(TAGS),-tags $(TAGS),)

# LOCAL=1 builds against the bellui toolkit in this checkout instead of the
# release go.mod pins, through a generated go.local.mod that adds a replace
# (gitignored, never committed — the same ephemeral replace x_tests.yml uses).
# x/editor itself always comes from the checkout: go.mod replaces it with ../..
LOCAL   ?=
MODFLAG := $(if $(LOCAL),-modfile=go.local.mod -mod=mod,)
REPO    := ../../../..

# This harness is a self-contained nested module (its own go.mod + replace).
# Disable Go workspace mode so an ambient go.work up the tree (e.g. a
# multi-module dev workspace that includes x/editor but not this directory)
# can't hijack module resolution — otherwise `go build .` reports
# "main module (github.com/belluikit/bellui/x/editor) does not contain package
# .../examples/wasm". Exported so every recipe's `go` invocation inherits it.
export GOWORK := off

# Force cgo off. GOOS=js/GOARCH=wasm never uses cgo, but if a machine has
# CGO_ENABLED=1 explicitly set (shell export or `go env -w`), Go honors it for
# the cross-compile and os/user fails to build for js ("undefined: current,
# lookupUser, ..."): the cgo path excludes js and the pure-Go stub is excluded
# by cgo being on. Pinning 0 here keeps the build deterministic across machines.
export CGO_ENABLED := 0

.PHONY: build serve clean tidy

# Build the wasm binary and copy the matching wasm_exec.js out of GOROOT
# (lib/wasm on Go >=1.24, misc/wasm on older toolchains).
build: $(if $(LOCAL),go.local.mod)
	GOOS=js GOARCH=wasm go build $(MODFLAG) $(TAGFLAG) -o editor.wasm .
	@cp "$(GOROOT)/lib/wasm/wasm_exec.js" . 2>/dev/null \
	  || cp "$(GOROOT)/misc/wasm/wasm_exec.js" . \
	  || { echo "could not find wasm_exec.js under $(GOROOT)"; exit 1; }
	@ls -lh editor.wasm | awk '{print "built editor.wasm:", $$5}'

# go.mod plus a replace of bellui onto this checkout, for LOCAL=1.
go.local.mod: go.mod go.sum
	cp go.mod go.local.mod
	cp go.sum go.local.sum
	go mod edit -modfile=go.local.mod -replace github.com/belluikit/bellui=$(REPO)

# Build, then serve the directory. Open http://localhost:$(PORT) in a browser.
serve: build
	@echo "open http://localhost:$(PORT)  (Ctrl+C to stop)"
	@python3 -m http.server $(PORT)

tidy:
	go mod tidy

clean:
	rm -f editor.wasm wasm_exec.js go.local.mod go.local.sum
