# Handy commands for working on bellui. `make` (or `make help`) lists them.
#
# The test targets use the tags CI uses, so a local pass means the same thing
# as a CI leg. Variables you can set on the command line:
#
#   P=desktop|android|ios|web   platform for the release check (default desktop)
#   VERSION=v1.3.0              release version (default: the release check's)
#   PKG=./widget/...            packages for test-pkg, test-wasm, stress, bench
#   RUN=TestName                test name pattern for test-pkg, stress, bench
#   COUNT=50                    repetitions for stress and bench
#   X="charts tables"           x/ modules for test-x and release-pins
#   GALLERY=../gallery          gallery checkout for the release check

SHELL := /bin/bash
.DEFAULT_GOAL := help

P       ?= desktop
VERSION ?= v1.3.0
PKG     ?= ./...
RUN     ?= .
COUNT   ?= 50
X       ?= charts dag editor mmedia tables
GALLERY ?= $(abspath ../gallery)
GOBIN   := $(shell go env GOPATH)/bin

# GLFW tests need a display. On Linux without one, run them under the
# repository's race-free Xvfb wrapper, as CI does.
ifeq ($(shell uname -s),Linux)
ifeq ($(DISPLAY),)
XVFB := ./.github/scripts/xvfb-run-safe
endif
endif

# Golden images the full test run rewrites when the renderer's output moves.
GOLDENS := internal/oksvg/testdata internal/rasterx/testdata internal/render/testdata

.PHONY: help
help: ## List the targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

##@ Build

.PHONY: cli
cli: ## Install the bellui command from this checkout into $GOPATH/bin
	cd cmd/bellui && go install .

.PHONY: build
build: ## Build every package of the main module
	go build ./...

##@ Test

.PHONY: test
test: ## Main module, as CI's Linux leg: -race -tags ci (see restore-goldens)
	go test -race -tags ci ./...

.PHONY: test-gl
test-gl: ## Main module with the GL painter, under Xvfb when there is no display
	$(XVFB) go test -race -tags gl,ci ./...

.PHONY: test-mobile
test-mobile: ## Main module as the mobile driver: -tags gl,mobile,ci,migrated_belluido
	$(XVFB) go test -race -tags gl,mobile,ci,migrated_belluido ./...

.PHONY: test-a11y
test-a11y: ## The accessibility bridge packages, as CI runs them
	$(XVFB) go test -race -tags accessibility,gl,ci ./internal/accessibility/... ./core/... ./internal/driver/glfw/

.PHONY: test-pkg
test-pkg: ## One package set: make test-pkg PKG=./lang/ RUN=TestLocale
	go test -race -tags ci -run '$(RUN)' $(PKG)

.PHONY: test-wasm
test-wasm: ## Packages under GOOS=js in Node: make test-wasm PKG=./internal/uidir/
	GOOS=js GOARCH=wasm go test -tags ci -exec "$$(go env GOROOT)/lib/wasm/go_js_wasm_exec" $(PKG)

.PHONY: test-x
test-x: ## x/ modules against this checkout, as CI does: make test-x X=charts
	@set -e; for m in $(X); do \
		echo "== x/$$m"; \
		( cd x/$$m && cp go.mod go.mod.makebak && cp go.sum go.sum.makebak && \
		  trap 'mv go.mod.makebak go.mod; mv go.sum.makebak go.sum' EXIT && \
		  go mod edit -replace github.com/belluikit/bellui=../.. && \
		  $(if $(XVFB),../../$(XVFB) ,)go test -race -tags gl,ci ./... ); \
	done

.PHONY: test-cli
test-cli: ## The bellui command's module
	cd cmd/bellui && go test ./...

.PHONY: stress
stress: ## A test under full CPU load: make stress PKG=./internal/driver/embedded/ RUN=TestNoOSDriver_TouchStopsDragCoast
	@n=$$(getconf _NPROCESSORS_ONLN); echo "loading $$n cores"; \
	for i in $$(seq $$n); do timeout 600 sh -c 'while :; do :; done' & done; \
	go test -race -tags ci -count=$(COUNT) -run '$(RUN)' -v $(PKG) | grep -E '^(--- |ok|FAIL)' | sed 's/ (.*//' | sort | uniq -c; \
	kill $$(jobs -p) 2>/dev/null; wait 2>/dev/null; true

.PHONY: bench
bench: ## Benchmarks for benchstat: make bench PKG=./widget/ RUN=BenchmarkRefresh > new.txt
	go test -tags ci -run '^$$' -bench '$(RUN)' -benchmem -count=$(COUNT) $(PKG)

.PHONY: restore-goldens
restore-goldens: ## Undo golden images a test run rewrote (discards any change to them)
	git checkout -- $(GOLDENS)

##@ Lint (what Static Analysis checks)

.PHONY: fmt
fmt: ## Rewrite files with gofumpt and goimports
	$(GOBIN)/gofumpt -w .
	$(GOBIN)/goimports -w .

.PHONY: fmt-check
fmt-check: ## Report unformatted files, as CI does
	@out="$$($(GOBIN)/gofumpt -l . ; $(GOBIN)/goimports -l .)"; \
	if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## go vet the main module
	go vet ./...

.PHONY: staticcheck
staticcheck: ## staticcheck, with and without -tags gl
	$(GOBIN)/staticcheck ./...
	$(GOBIN)/staticcheck -tags gl ./...

.PHONY: cyclo
cyclo: ## gocyclo over 30, with CI's exclusions
	$(GOBIN)/gocyclo -over 30 -ignore 'internal/oksvg|internal/rasterx|internal/render|^x/' .

.PHONY: changelog
changelog: ## Validate the changelog fragments of both changelogs
	go run ./hack/changelog check

.PHONY: lint
lint: fmt-check vet cyclo changelog ## fmt-check, vet, cyclo and changelog (staticcheck: make staticcheck)

.PHONY: tools
tools: ## Install the lint tools CI uses
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	go install golang.org/x/perf/cmd/benchstat@latest

##@ Release check (hack/releasecheck)

.PHONY: release-setup
release-setup: ## Build what the checks open: make release-setup P=android
	go run ./hack/releasecheck setup -platform $(P) -gallery $(GALLERY) -run

.PHONY: release-run
release-run: ## Walk through the checks and write a report: make release-run P=web
	go run ./hack/releasecheck run -platform $(P)

.PHONY: release-list
release-list: ## Write the whole checklist to release-validation-$(VERSION).md
	go run ./hack/releasecheck list > release-validation-$(VERSION).md
	@echo "wrote release-validation-$(VERSION).md"

.PHONY: release-app
release-app: ## Run the companion app on the desktop
	go run ./hack/releasecheck/app

.PHONY: release-app-a11y
release-app-a11y: ## Run the companion app with the screen-reader bridges
	go run -tags accessibility ./hack/releasecheck/app

.PHONY: release-app-web
release-app-web: cli ## Serve the companion app at http://localhost:8081
	cd hack/releasecheck/app && bellui serve --http-port 8081

.PHONY: gallery
gallery: ## Run the gallery built from this checkout (after release-setup)
	cd $(GALLERY) && go run .

.PHONY: gallery-web
gallery-web: cli ## Serve the gallery built from this checkout at http://localhost:8080
	cd $(GALLERY) && bellui serve

##@ Release (docs/RELEASING.md)

.PHONY: release-status
release-status: ## Open milestone items, open PRs and main's CI
	gh issue list --milestone $(VERSION) --state open
	gh pr list --state open
	gh run list --branch main --limit 8

.PHONY: release-changelog
release-changelog: ## Step 1: fold changelog.d into CHANGELOG.md (deletes the fragments)
	go run ./hack/changelog release $(VERSION)

.PHONY: release-tag-commands
release-tag-commands: ## Steps 3 and 5: print the tag commands (does not run them)
	@echo "# 3. main module, once main's CI is green on the commit:"
	@echo "git tag -a $(VERSION) -m \"$(VERSION)\" && git push origin $(VERSION)"
	@echo "# 5. x/ modules, after release-pins is committed and pushed:"
	@for m in $(X); do echo "git tag -a \"x/$$m/$(VERSION)\" -m \"x/$$m $(VERSION)\" && git push origin \"x/$$m/$(VERSION)\""; done

.PHONY: release-pins
release-pins: ## Step 4: move the x/ pins to the tagged $(VERSION) and verify each
	@set -e; for m in $(X); do \
		echo "== x/$$m"; \
		( cd x/$$m && go get github.com/belluikit/bellui@$(VERSION) && go mod tidy && \
		  go build ./... && go vet ./... && go test ./... ); \
	done

.PHONY: release-smoke
release-smoke: ## Step 6: build a fresh consumer of the published $(VERSION)
	@d=$$(mktemp -d); echo "in $$d"; cd $$d && go mod init smoke >/dev/null 2>&1 && \
	printf '%s\n' 'package main' 'import (' '	"github.com/belluikit/bellui"' '	"github.com/belluikit/bellui/app"' \
		'	"github.com/belluikit/bellui/widget"' ')' 'func main() {' '	a := app.New()' '	w := a.NewWindow("smoke")' \
		'	w.SetContent(widget.NewLabel("hello"))' '	bellui.ShowAndRun(w)' '}' > main.go && \
	go mod edit -require=github.com/belluikit/bellui@$(VERSION) && go mod tidy && go build . && echo "smoke build ok"
