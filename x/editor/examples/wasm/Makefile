# Runtime-spike harness for the fycoder editor under GOOS=js/GOARCH=wasm.
# See README.md for the manual test checklist.

GOROOT  := $(shell go env GOROOT)
PORT    ?= 8080

# Build tags. Empty = WebGPU (the default backend). TAGS=gl selects the WebGL
# fallback painter — build/serve both to cover the two web backends.
TAGS    ?=
TAGFLAG := $(if $(TAGS),-tags $(TAGS),)

# This harness is a self-contained nested module (its own go.mod + replace).
# Disable Go workspace mode so an ambient go.work up the tree (e.g. a
# multi-module dev workspace that includes the fycoder root but not this
# directory) can't hijack module resolution — otherwise `go build .` reports
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
build:
	GOOS=js GOARCH=wasm go build $(TAGFLAG) -o editor.wasm .
	@cp "$(GOROOT)/lib/wasm/wasm_exec.js" . 2>/dev/null \
	  || cp "$(GOROOT)/misc/wasm/wasm_exec.js" . \
	  || { echo "could not find wasm_exec.js under $(GOROOT)"; exit 1; }
	@ls -lh editor.wasm | awk '{print "built editor.wasm:", $$5}'

# Build, then serve the directory. Open http://localhost:$(PORT) in a browser.
serve: build
	@echo "open http://localhost:$(PORT)  (Ctrl+C to stop)"
	@python3 -m http.server $(PORT)

tidy:
	go mod tidy

clean:
	rm -f editor.wasm wasm_exec.js
