.PHONY: all build test check validate validate-examples validate-all fmt vet lint vuln clean release dist-bundle benchmark help

BIN := bin/okf
BUNDLE := knowledge
DIST_DIR := dist
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.Date=$(DATE)
RELEASE_LDFLAGS := -s -w $(LDFLAGS)

all: help

## build: Compile the standalone Go CLI and MCP server binary
build:
	@mkdir -p bin
	@go build -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/okf

## build-benchmark: Compile bin/okf-benchmark executable
build-benchmark:
	@mkdir -p bin
	@go build -ldflags="$(LDFLAGS)" -o bin/okf-benchmark ./cmd/okf-benchmark

## install: Install the binary to $GOPATH/bin
install:
	go install -ldflags="$(LDFLAGS)" ./cmd/okf

## test: Run all Go unit and integration tests
test:
	@go test -v ./...

## fmt: Format all Go source files with gofumpt
fmt:
	@gofumpt -w -extra .

## vet: Run go vet static analysis
vet:
	@go vet ./...

## lint: Run golangci-lint static analysis
lint:
	@which golangci-lint > /dev/null && golangci-lint run ./... || go vet ./...

## vuln: Run govulncheck vulnerability analysis
vuln:
	@govulncheck ./...

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

## check: Run vet, unit tests, and all bundle validations
check: vet test validate validate-examples

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
	@cp $(BIN) $(DIST_DIR)/okf-starter-pack/bin/okf 2>/dev/null || (mkdir -p $(DIST_DIR)/okf-starter-pack/bin && cp $(BIN) $(DIST_DIR)/okf-starter-pack/bin/okf)
	@tar -czf $(DIST_DIR)/okf-starter-pack-v$(VERSION).tar.gz -C $(DIST_DIR) okf-starter-pack
	@cd $(DIST_DIR) && zip -q -r okf-starter-pack-v$(VERSION).zip okf-starter-pack
	@echo "Created $(DIST_DIR)/okf-starter-pack-v$(VERSION).tar.gz and .zip"

## benchmark: Run local LLM progressive disclosure benchmark suite against LM Studio
benchmark:
	@go run ./cmd/okf-benchmark $(ARGS)

## clean: Remove compiled binaries and release distributions
clean:
	@rm -rf bin $(DIST_DIR)

help:
	@echo "OKF Agent Memory Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  make build             Compile bin/okf executable"
	@echo "  make build-benchmark   Compile bin/okf-benchmark executable"
	@echo "  make install           Install bin/okf to \$$GOPATH/bin"
	@echo "  make test              Run Go unit tests"
	@echo "  make fmt               Format code with gofumpt"
	@echo "  make vet               Run go vet static analysis"
	@echo "  make lint              Run golangci-lint"
	@echo "  make vuln              Run govulncheck vulnerability scanner"
	@echo "  make validate          Run strict OKF v0.2 validation on knowledge/"
	@echo "  make validate-examples Run strict validation on example corpora"
	@echo "  make validate-all      Validate knowledge/ and all examples"
	@echo "  make check             Run vet, tests, and all validations"
	@echo "  make benchmark         Run local LLM benchmark (ARGS=\"-dry-run\" or ARGS=\"-o\")"
	@echo "  make release           Cross-compile binaries for macOS, Linux, and Windows"
	@echo "  make dist-bundle       Build standalone starter pack archives (.tar.gz & .zip)"
	@echo "  make clean             Remove build and dist artifacts"
	@echo "  make help              Display this help message"
