# mdserve — build, install, and release.
# Run `make` (or `make help`) to list targets.

BIN     := bin/mdserve
PKG     := .
BRANCH  := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
# Local builds stamp the version from git (e.g. v0.4.1-2-gabc1234-dirty); the
# release workflow overrides it with the exact tag. Override with `make VERSION=...`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.DEFAULT_GOAL := help
.PHONY: help build install run test fmt vet clean push tag release

help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-9s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary to ./bin/mdserve (version-stamped)
	@mkdir -p bin
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)
	@echo "built $(BIN)  $(VERSION)"

install: ## go install (version-stamped) into GOBIN / GOPATH/bin
	go install -trimpath -ldflags '$(LDFLAGS)' $(PKG)
	@echo "installed mdserve  $(VERSION)"

run: ## Build + serve the current directory, opening the browser
	go run $(PKG) serve --open

test: ## Run the test suite
	go test ./...

fmt: ## gofmt the tree
	gofmt -w .

vet: ## go vet
	go vet ./...

clean: ## Remove build output
	rm -rf bin

push: ## Push the current branch + annotated tags to origin
	git push origin $(BRANCH) --follow-tags

tag: ## Create + push an annotated release tag: make tag V=v0.4.2
	@test -n "$(V)" || { echo "usage: make tag V=vX.Y.Z"; exit 1; }
	@echo "$(V)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([.-].*)?$$' || { echo "tag must look like vX.Y.Z"; exit 1; }
	@git diff --quiet && git diff --cached --quiet || { echo "working tree dirty — commit first"; exit 1; }
	git tag -a "$(V)" -m "$(V)"
	git push origin "$(V)"
	@echo "pushed $(V) — GitHub Actions will build the release"

release: ## Push the branch, then cut a release tag (triggers CI): make release V=v0.4.2
	@test -n "$(V)" || { echo "usage: make release V=vX.Y.Z"; exit 1; }
	git push origin $(BRANCH)
	$(MAKE) tag V=$(V)
