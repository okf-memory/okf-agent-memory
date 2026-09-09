.PHONY: all build build-benchmark install test fmt vet lint vuln audit-security jules-list jules-review jules-merge validate validate-examples validate-all check release dist-bundle benchmark clean help

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

## check: Run the complete CI/local pipeline (fmt, vet, lint, test, validate-all)
check: fmt vet lint test validate-all

## test: Run all Go unit and integration tests
test:
	@go test -v ./...

## fmt: Format all Go source files with gofumpt / gofmt
fmt:
	@which gofumpt > /dev/null && gofumpt -w -extra . || gofmt -w -s .

## vet: Run go vet static analysis
vet:
	@go vet ./...

## lint: Run golangci-lint static analysis (falls back to go vet)
lint:
	@which golangci-lint > /dev/null && golangci-lint run ./... || go vet ./...

## audit-security: Run automated security analysis (gosec and govulncheck)
audit-security:
	@echo "==> Running security checks..."
	@which gosec > /dev/null && gosec -quiet -exclude=G301,G306 -exclude-dir=examples ./... || echo "gosec: optional (install via: go install github.com/securego/gosec/v2/cmd/gosec@latest)"
	@which govulncheck > /dev/null && govulncheck ./... || echo "govulncheck: optional (install via: go install golang.org/x/vuln/cmd/govulncheck@latest)"

## vuln: Run govulncheck vulnerability scanner directly
vuln:
	@govulncheck ./...

## jules-list: List open Google Jules security audit branches on origin
jules-list:
	@./scripts/jules-flow.sh list

## jules-review: Build, test, and review latest or specified Jules branch (BRANCH=<name>) in an isolated worktree
jules-review:
	@./scripts/jules-flow.sh review $(BRANCH)

## jules-merge: Merge verified Jules remediation branch into develop (BRANCH=<name>)
jules-merge:
	@./scripts/jules-flow.sh merge $(BRANCH)

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

## validate-examples: Validate all bundled example corpora
validate-examples: build
	@$(BIN) validate examples/software --strict
	@$(BIN) validate examples/coaching --strict
	@$(BIN) validate examples/books --strict

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

## dist-bundle: Package a complete, ready-to-use starter pack archive (.tar.gz and .zip)
dist-bundle: build
	@mkdir -p $(DIST_DIR)/okf-starter-pack
	@$(BIN) bootstrap $(DIST_DIR)/okf-starter-pack --name "Project"
	@mkdir -p $(DIST_DIR)/okf-starter-pack/bin && cp $(BIN) $(DIST_DIR)/okf-starter-pack/bin/okf
	@tar -czf $(DIST_DIR)/okf-starter-pack-v$(VERSION).tar.gz -C $(DIST_DIR) okf-starter-pack
	@cd $(DIST_DIR) && zip -q -r okf-starter-pack-v$(VERSION).zip okf-starter-pack
	@echo "Created $(DIST_DIR)/okf-starter-pack-v$(VERSION).tar.gz and .zip"

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
	@echo "  make audit-security    Run automated security audit (gosec, govulncheck)"
	@echo "  make jules-list        List open Jules security branches on origin"
	@echo "  make jules-review      Run isolated build & test of latest Jules branch"
	@echo "  make jules-merge       Merge verified Jules branch into develop"
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
	@echo "  make dist-bundle       Build starter pack archives (.tar.gz & .zip)"
	@echo "  make clean             Remove build and dist artifacts"
	@echo "  make help              Display this help message"
