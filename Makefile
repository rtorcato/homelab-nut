# homelab-nut Makefile
#
# Common entry points for local development. CI uses these targets too.

BINARY      := homelab-nut
PKG         := github.com/rtorcato/homelab-nut
CMD_PATH    := ./cmd/homelab-nut
BIN_DIR     := bin
DIST_DIR    := dist

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

GO          ?= go
GO_FLAGS    := -trimpath -ldflags '$(LDFLAGS)'

.PHONY: all build run test lint tidy clean install snapshot todos docs-cli docs-dev docs-build embed-sync help

all: build

## build: compile the binary to bin/$(BINARY)
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_FLAGS) -o $(BIN_DIR)/$(BINARY) $(CMD_PATH)

## run: build and launch the TUI
run: build
	./$(BIN_DIR)/$(BINARY)

## test: run unit tests with race detector
test:
	$(GO) test -race -count=1 ./...

## lint: run golangci-lint (requires golangci-lint installed)
lint:
	golangci-lint run ./...

## tidy: clean up go.mod / go.sum
tidy:
	$(GO) mod tidy

## install: install the binary to $GOBIN (or $GOPATH/bin)
install:
	$(GO) install $(GO_FLAGS) $(CMD_PATH)

## snapshot: build cross-platform snapshot via goreleaser (requires goreleaser)
snapshot:
	goreleaser release --snapshot --clean --skip=publish

## todos: regenerate TODOS.md from GitHub Issues (requires gh + jq)
todos:
	./scripts/gen-todos.sh

## docs-cli: regenerate apps/docs/docs/cli/ from the CLI's cobra tree
docs-cli:
	$(GO) run ./cmd/gen-docs

## docs-dev: run the docs site locally at http://localhost:3000
docs-dev:
	cd apps/docs && pnpm dev

## docs-build: produce a static build of the docs site
docs-build:
	cd apps/docs && pnpm build

## embed-sync: copy /scripts/*.sh into internal/roles/embedded/ so the
## binary's embedded scripts match the canonical ones users run today.
## CI runs this and fails on a non-empty diff.
embed-sync:
	cp scripts/setup-server.sh internal/roles/embedded/setup-server.sh
	cp scripts/setup-client.sh internal/roles/embedded/setup-client.sh

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

## help: list available targets
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} \
		/^## / {sub(/^## /, "", $$0); split($$0, a, ":"); printf "  \033[36m%-12s\033[0m %s\n", a[1], a[2]}' \
		$(MAKEFILE_LIST)
