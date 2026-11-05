---
type: Process
title: "Automated Security Auditing & Defensive Verification Workflow"
description: "Proactive continuous security auditing with static analyzers (gosec, govulncheck), adversarial agent reviews, and regression testing."
generated: { by: agent/mcp, at: "2026-11-05T22:08:32Z" }
---

# Automated Security Auditing & Defensive Verification Workflow

## Context & Objectives

To maintain the highest level of defensive assurance for `okf-agent-memory`, automated security auditing is integrated into the local and CI verification pipeline (`make security-audit` and `make check`), complemented by adversarial agent security reviews and regression testing against `docs/security/SECURITY_AUDIT.md`.

---

## 1. Automated Static & Vulnerability Analysis (`make security-audit`)

Every verification run executes static security analysis and vulnerability scanning across all Go packages:

```bash
make security-audit
```

* **`gosec`**: Inspects Go AST for security weaknesses (`-exclude=G301,G306 -exclude-dir=examples ./...`). Every intentional file access in `pkg/okf`, `pkg/sync`, `pkg/vault`, and `pkg/registry` must first pass canonical boundary containment (`ensureWithinRoot` or `validateTreePath`) before `#nosec` annotations are permitted.
* **`govulncheck`**: Verifies the Go standard library toolchain and `golang.org/x/crypto` against the official Go vulnerability database.

---

## 2. Adversarial Security Audit Scope

Security audits target `pkg/okf`, `pkg/sync`, `pkg/vault`, `pkg/registry`, and `internal/cli` across five defensive areas:

* **Path Traversal & Bundle Boundary Containment (CWE-22)**: Canonical path verification (`ensureWithinRoot`, `resolveInBundle`, `validateTreePath`, `ValidateBundleID`) across POSIX and Windows path conventions (`IsAbsPath`).
* **Symlink Escape Prevention (CWE-59)**: Confining bundle roots and internal symlinks strictly to markdown files within the canonical bundle boundary.
* **Concurrent Write Serialization & CAS Integrity (CWE-362)**: Cross-process bundle write locking (`WithBundleLock` via `.okf-write.lock`), transactional RMW lock scopes (`UpdateConcept`, `RelateConcepts`), and optimistic concurrency checks (`ErrConcurrentModification`) preventing silent lost updates.
* **MCP Tool Argument Sanitization & Root Confinement**: Strict parameter length bounds (`getStringArg`), 4MB streaming line limits, server root confinement (`resolveBundleDir`), and human verification provenance guards (`ensureNoForgedHumanVerification`).
* **Resource Bounds & Frontmatter Injection Defense**: Rejecting newline smuggling and nested `---` frontmatter blocks in concept metadata and bodies (`sanitizeConceptMetadata`).

---

## 3. Verification & Regression Testing Pipeline (`make check`)

Every security hardening change or bug fix must include minimal reproducible negative unit tests in `pkg/okf/mutate_security_test.go`, `pkg/okf/concurrency_test.go`, `pkg/okf/mcp_test.go`, or `pkg/sync/*_test.go`, and pass the complete deterministic gate before merge into `develop`:

```bash
make check
```

1. **`make fmt` & `make vet` & `make lint`**: Zero static analysis or linter findings (`golangci-lint`).
2. **`make security-audit`**: Zero `gosec` or `govulncheck` vulnerabilities.
3. **`make test`**: 100% pass rate with the Go race detector enabled (`go test -v -race ./...`).
4. **`make validate-all`**: Zero errors, zero warnings, and zero drift across `knowledge/` and all reference examples.

---

# Related Concepts
- [Bundle Isolation and Mutation Security Boundaries](../architecture/security-boundaries.md): Core defensive security boundaries and concurrency controls
- [MCP Tool Security & Untrusted Agent Input](mcp-agent-safety.md): Sanitization and confinement requirements for agent MCP interfaces
- [Contributor Guidelines & PR Standards](contributing.md): General quality gates and verification standards for pull requests
