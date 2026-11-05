.PHONY: all build build-benchmark install test fmt vet lint vuln security-audit validate validate-examples validate-all check release benchmark clean help sync-assets update-deps update-tools

BIN := bin/okf
BUNDLE := knowledge
DIST_DIR := dist
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.Date=$(DATE)
RELEASE_LDFLAGS := -s -w $(LDFLAGS)

all: help

## ----------------------------------------------------------------------
## Pipeline & Verification
## ----------------------------------------------------------------------

## check: Run the complete CI/local pipeline (fmt, vet, lint, security-audit, test, validate-all)
check: fmt vet lint security-audit test validate-all

## test: Run all Go unit and integration tests (including dogfood asset drift check)
test:
	@go test -v -race ./...

## sync-assets: Synchronize active skill files (.agents/skills/) to embedded bootstrap assets (pkg/okf/assets/skill/)
sync-assets:
	@echo "==> Synchronizing dogfooded skills to embedded bootstrap assets..."
	@mkdir -p pkg/okf/assets/skill
	@cp -R .agents/skills/okf-memory/* pkg/okf/assets/skill/
	@echo "==> Done. Embedded assets in pkg/okf/assets/skill/ are now synchronized."

## fmt: Format all Go source files with gofumpt / gofmt
fmt:
	@which gofumpt > /dev/null && gofumpt -w -extra . || gofmt -w -s .

## vet: Run go vet static analysis
vet:
	@go vet ./...

## lint: Run golangci-lint static analysis (falls back to go vet if not installed)
lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

## security-audit: Run automated security analysis (gosec and govulncheck)
security-audit:
	@echo "==> Running security checks..."
	@which gosec > /dev/null && gosec -quiet -exclude=G301,G306 -exclude-dir=examples ./... || echo "gosec: optional (install via: go install github.com/securego/gosec/v2/cmd/gosec@latest)"
	@which govulncheck > /dev/null && govulncheck ./... || echo "govulncheck: optional (install via: go install golang.org/x/vuln/cmd/govulncheck@latest)"

## vuln: Run govulncheck vulnerability scanner directly
vuln:
	@govulncheck ./...

## ----------------------------------------------------------------------
## Tooling & Dependencies
## ----------------------------------------------------------------------

## update-deps: Update Go module dependencies and development/security tools
update-deps: update-tools
	@echo "==> Updating Go module dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "==> Dependencies updated successfully."

## update-tools: Install or update development and security tools to latest versions
update-tools:
	@echo "==> Updating development and security tools..."
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install mvdan.cc/gofumpt@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "==> Tools updated successfully."

## ----------------------------------------------------------------------
## Build & OKF Knowledge Validation
## ----------------------------------------------------------------------

## build: Compile the standalone Go CLI and MCP server binary
build:
	@mkdir -p bin
	@go build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/okf

## build-benchmark: Compile bin/okf-benchmark executable
build-benchmark:
	@mkdir -p bin
	@go build -ldflags="$(LDFLAGS)" -o bin/okf-benchmark ./cmd/okf-benchmark

## install: Install bin/okf to $GOPATH/bin
install:
	@go install -ldflags="$(LDFLAGS)" ./cmd/okf

## validate: Run strict OKF v0.2 validation on the knowledge/ bundle
validate: build
	@$(BIN) validate $(BUNDLE) --strict --drift

## validate-examples: Validate all bundled example corpora including AGENTS.md governance
validate-examples: build
	@$(BIN) validate --agents --strict examples/software
	@$(BIN) validate --agents --strict examples/coaching
	@$(BIN) validate --agents --strict examples/books

## validate-all: Validate project knowledge and all examples
validate-all: validate validate-examples

## ----------------------------------------------------------------------
## Release & Packaging
## ----------------------------------------------------------------------

## release: Cross-compile binaries for macOS, Linux, and Windows
release:
	@mkdir -p $(DIST_DIR)/bin
	@echo "Building cross-platform binaries (v$(VERSION), commit $(COMMIT))..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/bin/okf-darwin-arm64 ./cmd/okf
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/bin/okf-darwin-amd64 ./cmd/okf
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/bin/okf-linux-amd64 ./cmd/okf
	@GOOS=linux GOARCH=arm64 go build -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/bin/okf-linux-arm64 ./cmd/okf
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/bin/okf-windows-amd64.exe ./cmd/okf
	@GOOS=windows GOARCH=arm64 go build -ldflags="$(RELEASE_LDFLAGS)" -o $(DIST_DIR)/bin/okf-windows-arm64.exe ./cmd/okf
	@echo "Release binaries built in $(DIST_DIR)/bin/"


## benchmark: Run local LLM progressive disclosure benchmark suite against LM Studio
benchmark:
	@go run ./cmd/okf-benchmark $(ARGS)

## clean: Remove compiled binaries and release distributions
clean:
	@rm -rf bin $(DIST_DIR)

## help: Display available targets
help:
	@echo "OKF Agent Memory Makefile"
	@echo ""
	@echo "Pipeline:"
	@echo "  make check             Run complete pipeline (fmt, vet, lint, test, validate-all)"
	@echo "  make test              Run Go unit tests"
	@echo "  make fmt               Format code with gofumpt / gofmt"
	@echo "  make vet               Run go vet static analysis"
	@echo "  make lint              Run golangci-lint (fallback: go vet)"
	@echo "  make security-audit    Run automated security audit (gosec, govulncheck)"
	@echo ""
	@echo "Tooling & Dependencies:"
	@echo "  make update-deps       Update Go dependencies and dev/security tools"
	@echo "  make update-tools      Install or update tools (govulncheck, golangci-lint, gofumpt, gosec)"
	@echo ""
	@echo "Build & Knowledge:"
	@echo "  make build             Compile bin/okf executable"
	@echo "  make install           Install bin/okf to \$$GOPATH/bin"
	@echo "  make validate          Validate knowledge/ bundle (--strict --drift)"
	@echo "  make validate-examples Validate all example corpora"
	@echo "  make validate-all      Validate knowledge/ and all examples"
	@echo ""
	@echo "Distribution:"
	@echo "  make release           Cross-compile binaries for macOS, Linux, and Windows"
	@echo "  make clean             Remove build and dist artifacts"
	@echo "  make help              Display this help message"
