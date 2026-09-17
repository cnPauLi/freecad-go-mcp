# FreeCAD MCP (Go) — build, test and cross-compilation targets.
#
# Every target builds with CGO_ENABLED=0, so the resulting binaries are
# statically linked and depend on no external library. `make dist` produces all
# six release platforms (windows/linux/darwin x amd64/arm64) in ./dist.

BINARY  := freecad-go-mcp
DIST    := dist
TAG     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
VERSION ?= $(patsubst v%,%,$(TAG))

# x86_64 = amd64, arm = arm64.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build dist checksums test fmt vet check clean

all: check dist

build: ## Build for the host platform into ./$(BINARY)
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) .

dist: ## Cross-compile static binaries for all six release platforms
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		goos=$${p%/*}; \
		goarch=$${p#*/}; \
		ext=""; \
		if [ "$$goos" = "windows" ]; then ext=".exe"; fi; \
		out="$(DIST)/$(BINARY)_$(VERSION)_$${goos}_$${goarch}$$ext"; \
		echo "GOOS=$$goos GOARCH=$$goarch -> $$out"; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch \
			go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o "$$out" . || exit 1; \
	done

checksums: dist ## Write dist/SHA256SUMS for the built binaries
	@rm -f $(DIST)/SHA256SUMS
	@cd $(DIST) && if command -v sha256sum >/dev/null 2>&1; then \
		sha256sum * > SHA256SUMS; \
	else \
		shasum -a 256 * > SHA256SUMS; \
	fi
	@echo "wrote $(DIST)/SHA256SUMS"

test:
	go test ./...

fmt: ## Fail if any file is not gofmt-clean
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed for:"; echo "$$unformatted"; exit 1; \
	fi

vet:
	go vet ./...

check: fmt vet test

clean:
	rm -rf $(DIST) $(BINARY)