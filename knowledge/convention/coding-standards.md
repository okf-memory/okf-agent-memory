---
type: Convention
title: "Engineering & Coding Best Practices (Clean Code, TDD, DRY)"
description: "Core software engineering conventions for agents and humans covering Clean Code, TDD, DRY, idiomatic Go, and zero-dependency design."
generated: { by: agent/mcp, at: "2026-09-08T10:52:57Z" }
---

# Engineering & Coding Best Practices (Clean Code, TDD, DRY)

## Context & Purpose

This convention establishes the core software engineering standards and behavioral coding guarantees for AI agents and human engineers developing the `okf-agent-memory` codebase. Adherence ensures zero-dependency simplicity, predictable performance, and high defensive resilience across platforms.

---

## 1. Test-Driven Development (TDD) & Regression Defense

All feature additions, bug fixes, and security remediations must follow a Test-First workflow:

1. **Red (Reproduce First)**:
   - Before writing or modifying production code, write a minimal, failing unit test in `pkg/okf/*_test.go` or `cmd/okf/*_test.go`.
   - Verify that the test fails for the expected reason (`go test -v -run <TestName> ./...`).
2. **Green (Minimal Fix)**:
   - Implement the most concise, straightforward fix that satisfies the test.
   - Avoid speculative code or unrelated refactoring during the fix step.
3. **Refactor & Adversarial Hardening**:
   - Augment the test suite with adversarial edge cases:
     * Platform paths (Windows backslashes vs. Unix forward slashes via `filepath.ToSlash`).
     * Case insensitivity on APFS/NTFS (`strings.EqualFold`).
     * Whitespace-only strings, trailing slashes, empty inputs.
     * Internal and external symlinks (`ensureWithinRoot`).
4. **Zero Test Regressions**:
   - All tests in `pkg/okf` and `cmd/okf` must pass with `make test`.

---

## 2. Clean Code & Idiomatic Go Standards

Code must adhere to modern idiomatic Go (Go 1.24+ / Go 1.26):

* **Explicit Error Handling**:
  - Never swallow errors silently (e.g. `_ = err` in production logic).
  - Wrap errors with actionable context using `%w`: `fmt.Errorf("failed to parse concept %q: %w", path, err)`.
  - Handle errors immediately at the call site ("guard clause" / early return style) to minimize indentation nesting.
* **Single Responsibility & Function Size**:
  - Functions should perform exactly one task. If a function exceeds ~50 lines or requires multiple levels of nesting, decompose it into focused helper functions.
* **Self-Documenting Naming**:
  - Use clear, descriptive names for functions, types, and variables.
  - Follow Go receiver conventions (e.g. `b *Bundle`, `c *Concept`, `s *mcpServer`).
* **Zero External Dependencies**:
  - The Go library and CLI rely **100% on the Go standard library** (`os`, `io`, `path`, `filepath`, `strings`, `slices`, `time`, `encoding/json`).
  - Never introduce external packages or third-party frameworks.

---

## 3. DRY, KISS & Centralized Choke-Points

Maintain architectural discipline by avoiding both code duplication and over-engineering:

* **Centralize Security & Invariant Choke-Points (DRY)**:
  - Security checks (path containment, symlink traversal, reserved document protection) must live in central choke-points (`ensureWithinRoot`, `ValidateConceptID`, `resolveInBundle`).
  - Do not duplicate ad-hoc sanitization checks across multiple CLI or MCP callers.
* **Keep It Simple (KISS) & Avoid Premature Abstraction (YAGNI)**:
  - Prefer concrete structs and plain functions over deeply nested interfaces or abstract factory patterns.
  - Do not create abstractions for single-use implementations.
* **Platform Portability**:
  - Always use `filepath.Join`, `filepath.Clean`, and `filepath.ToSlash` rather than manual string concatenation with `/` or `\`.

---

## 4. Performance & Resource Bounds

OKF Agent Memory is engineered for high-frequency agent tool calling loops:

* **Microsecond Search Budget**: Maintain the sub-300µs BM25 in-memory search budget. Avoid unnecessary heap allocations in hot search loops.
* **Progressive Disclosure**: Agents and tools must parse full concept bodies only when requested; concept listings and searches rely on lightweight summaries and indices.

---

## 5. Pre-Commit Quality Gate

Before submitting a PR, opening a review, or concluding an agent task, verify:

```bash
make check
```

This runs the complete deterministic pipeline:
1. `make fmt`: Source code formatted with `gofumpt` / `gofmt`.
2. `make vet`: Go vet static diagnostics clean.
3. `make lint`: `golangci-lint` passes with 0 issues.
4. `make test`: 100% unit and scenario test pass rate.
5. `make validate-all`: All OKF bundles (`knowledge/` and all examples) strictly conformant.

---

# Related Concepts
- [Contributor Guidelines & PR Standards](contributing.md): Quality gates and contribution workflow
- [5-Layer System Architecture](../architecture/layers.md): Layered separation of concerns and zero-dependency rule
- [Bundle Isolation and Mutation Security Boundaries](../architecture/security-boundaries.md): Defensive choke-points and path isolation architecture
- [Automated Security Auditing & Jules Remediation Workflow](security-audit.md): Proactive adversarial auditing and verification pipeline
