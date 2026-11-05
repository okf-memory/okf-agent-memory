---
type: Convention
title: "Contributor Guidelines & PR Standards"
description: "Engineering standards, zero third-party dependency policy, validation rules, and PR workflow for contributors."
tags: [contributing, contribute, workflow, standards, pr, pull-request]
generated: { by: agent/mcp, at: "2026-11-05T22:08:03Z" }
status: stable
sources:
  - resource: ../../CONTRIBUTING.md
    id: contributing-guide
    title: OKF Agent Memory Contributing Guide
    last_modified: 2026-09-11
---

# Contributor Guidelines & PR Standards

This convention establishes the quality gates, engineering expectations, and verification standards for contributors—both human developers and AI agents—contributing to `okf-agent-memory`.

---

## 1. Non-Negotiable Core Tenets

All contributions must adhere to four foundational constraints:

1. **Issue-First Contribution Policy**:
   - All non-trivial PRs must be preceded by an approved GitHub Issue to ensure architectural alignment and protect maintainer bandwidth.
   - Unsolicited Pull Requests and automated agent/bot sweeps with blank or unedited templates are closed without review.
2. **Zero Third-Party Dependencies**:
   - The core Go library (`pkg/okf`) relies exclusively on the Go standard library.
   - For zero-knowledge cryptographic primitives (`pkg/vault`), only official Go project sub-repositories (`golang.org/x/crypto`) are permitted for Argon2id key derivation.
   - Third-party packages, external vendor libraries, and CGo dependencies are strictly forbidden to ensure zero supply-chain risk, instant compilation, and reliable cross-platform compilation.
3. **OKF v0.2 Specification Conformance**:
   - The project's own `knowledge/` bundle and all bundles scaffolded by `okf bootstrap` or `okf init` must strictly conform to the Open Knowledge Format v0.2 specification.
4. **Walk the Talk (Persistent Knowledge Maintenance)**:
   - Any pull request that alters architecture, introduces CLI flags, adjusts conventions, or resolves non-trivial design decisions must update `knowledge/` and maintain `knowledge/log.md`.

---

## 2. Development & Build Standards

- **Go Version**: Go 1.24+ (or Go 1.26).
- **Formatting**: Go code must be formatted with standard formatting tools (`gofumpt` or `go fmt`).
- **Static Analysis**: Code must pass `go vet ./...` and `make lint` without diagnostics.
- **Testing**: Unit tests in `pkg/okf` must maintain complete coverage and pass with `make test`.

---

## 3. Knowledge Maintenance Rules for PRs

When preparing a Pull Request:
1. **Search Before Authoring**: Run `okf search "<query>"` before adding new concepts to prevent duplicate files.
2. **Deterministic CLI Operations**: Use `okf create`, `okf update`, and `okf relate` to mutate knowledge files and update graph links.
3. **Graph Integrity**: Every new concept must be linked into the inbound/outbound graph. No orphaned concepts are permitted under `--strict` mode.
4. **Validation**: The bundle must pass strict validation before opening a PR:
   ```bash
   make validate
   # or: okf validate knowledge --strict --drift
   ```

---

## 4. Branching Strategy & PR Target

`okf-agent-memory` follows a GitFlow-inspired branching model:
- **`develop` (Default Branch)**: All active development, feature branches, and external Pull Requests must branch from and target `develop`.
- **`main` (Production Releases)**: Strictly reserved for tagged production releases (e.g. `v0.8.0`). Direct PRs against `main` are rejected.

---

## 5. Pre-Submission Quality Gate

Before requesting a review or merging into `develop` (or preparing a release to `main`), verify:
- [ ] `go fmt ./...` produces no diffs.
- [ ] `go vet ./...` and `make lint` report zero issues.
- [ ] `make test` runs with 100% passing unit tests.
- [ ] `make validate` exits with code 0 (`0 errors, 0 warnings, 0 orphans, 0 broken links`).
- [ ] Conventional Commit format used for all commit messages.

---

# Related Concepts
- [Core Memory Principles & Agent Contract](principles.md): Contributors must follow the core memory principles
- [Knowledge Lifecycle & Review Workflow](lifecycle.md): PR workflows must integrate the knowledge review lifecycle
- [5-Layer System Architecture](../architecture/layers.md): Code contributions must adhere to the 5-layer architecture and zero-dependency rule
- [Automated Security Auditing & Defensive Verification Workflow](security-audit.md): Continuous security audit expectations and verification pipeline
- [Engineering & Coding Best Practices (Clean Code, TDD, DRY)](coding-standards.md): Core coding conventions including TDD, Clean Code, DRY, and idiomatic Go
- [Release Procedure & Distribution Runbook](release-procedure.md): Release preparation, quality gates, and git tagging procedure
