.PHONY: all validate test search help

OKF_BIN ?= $(shell command -v okf 2>/dev/null || echo "bin/okf")
BUNDLE := knowledge

all: validate

## validate: Run strict OKF v0.2 validation on the knowledge/ bundle
validate:
	@$(OKF_BIN) validate $(BUNDLE) --strict --drift

test: validate

## search: Search project memory (e.g. make search q="auth")
search:
	@$(OKF_BIN) search "$(q)" $(BUNDLE)

help:
	@echo "Project Memory Commands:"
	@echo "  make validate       Validate knowledge/ bundle"
	@echo "  make search q=\"...\" Search project memory"
