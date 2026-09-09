## 2026-09-08
* **Update**: Linked `architecture/layers.md` and `convention/contributing.md` to `convention/coding-standards.md`.
* **Creation**: Documented concept `convention/coding-standards.md` (Engineering & Coding Best Practices (Clean Code, TDD, DRY)).
* **Update**: Linked `convention/contributing.md` to `convention/security-audit.md` (Continuous security audit expectations and integration pipeline).
* **Update**: Updated `architecture/security-boundaries.md` documenting case-insensitive reserved root file protection and collaborative file permission rationale (`0o644`/`0o755`).
* **Creation**: Documented concept `convention/security-audit.md` (Automated Security Auditing & Jules Remediation Workflow).
* **Release**: Published version v0.1.4 — Cross-Platform & Spec Alignment Maintenance Release resolving Windows relative link resolution (#6), accepting spec-valid open actor prefixes without warning (#5), allowing dot-directory bundle root scans (#4), and streamlining the Makefile test and lint pipeline.
* **Release**: Published version v0.1.3 — Security Hardening Release resolving path traversal in concept creation/bookkeeping (reported by @djmaze), symlink following (LFI/overwrite), MCP server root confinement, YAML metadata smuggling, and establishing the continuous adversarial security audit framework.
* **Update**: Linked `architecture/security-boundaries.md` to `convention/mcp-agent-safety.md` (Behavioral MCP guidelines complement deterministic boundaries).
* **Update**: Updated concept `architecture/security-boundaries.md`.
* **Update**: Linked `convention/principles.md` to `convention/mcp-agent-safety.md` (Principles require secure tool interaction and treating agent input as untrusted).
* **Update**: Updated concept `convention/principles.md`.
* **Creation**: Documented concept `convention/mcp-agent-safety.md` (MCP Tool Security & Untrusted Agent Input).
* **Update**: Linked `architecture/layers.md` to `architecture/security-boundaries.md` (Layer 4 tooling enforces security boundaries and bundle isolation).
* **Update**: Updated concept `architecture/layers.md`.
* **Update**: Linked `architecture/tooling-decision.md` to `architecture/security-boundaries.md` (Tooling layer enforces security and bundle containment boundaries).
* **Update**: Updated concept `architecture/tooling-decision.md`.
* **Creation**: Documented concept `architecture/security-boundaries.md` (Bundle Isolation and Mutation Security Boundaries).
* **Security**: Enforced MCP server root directory confinement in `resolveBundleDir` preventing path traversal outside workspace via the `bundle` tool parameter.
* **Security**: Hardened bundle operations and mutations against path traversal and symlink escapes (LFI and arbitrary file write prevention) via canonical boundary verification in `LoadBundle`, `SaveConcept`, `UpdateParentIndex`, and `AppendLogEntry`.
* **Security**: Enforced single-line constraints and frontmatter delimiter sanitization on concept metadata to prevent YAML attribute smuggling.

## 2026-09-06
* **Release**: Published version v0.1.1 resolving MCP server JSON-RPC 2.0 notification compliance, adding dynamic multi-bundle resolution, and documenting Dual-Mode (MCP-First) agent workflows.

## 2026-09-05
* **Release**: Prepared official Release v0.1.0 of OKF Agent Memory (pure Go single binary, sub-300µs BM25 search, embedded stdio MCP server, and 1-step bootstrap).

## 2026-09-04
* **Update**: Updated roadmap milestones in `knowledge/roadmap/milestones.md` marking Phases 8, 10, and 11 as completed.
* **Update**: Linked `convention/principles.md` to `convention/contributing.md` (PR contributors must adhere to these core memory principles).
* **Update**: Updated concept `convention/principles.md`.
* **Update**: Linked `convention/contributing.md` to `architecture/layers.md` (Code contributions must adhere to the 5-layer architecture and zero-dependency rule).
* **Update**: Updated concept `convention/contributing.md`.
* **Update**: Linked `convention/contributing.md` to `convention/lifecycle.md` (PR workflows must integrate the knowledge review lifecycle).
* **Update**: Updated concept `convention/contributing.md`.
* **Update**: Linked `convention/contributing.md` to `convention/principles.md` (Contributors must follow the core memory principles).
* **Update**: Updated concept `convention/contributing.md`.
* **Creation**: Documented concept `convention/contributing.md` (Contributor Guidelines & PR Standards).

## 2026-09-01
* **Decision**: Secured and standardized official project domain `okf-memory.dev` for documentation and ecosystem positioning.
* **Creation**: Added comprehensive competitive comparison against `agent-memory.dev` in `OKF-MEMORY_VS_AGENT-MEMORY.md` and linked in `docs/ALTERNATIVES.md`.
* **Update**: Updated `knowledge/project/overview.md` with official canonical domain `https://okf-memory.dev`.

## 2026-08-28
* **Creation**: Implemented Multi-Agent Testing suite (`docs/AGENT_TESTING.md`) and automated Go integration tests (`pkg/okf/scenarios_test.go`) validating the 10-step Definition of Done.
* **Creation**: Added comprehensive project documentation: `docs/GETTING_STARTED.md`, `docs/CLI.md`, and `docs/SECURITY.md`.
* **Creation**: Added GitHub Actions CI (`.github/workflows/ci.yml`) and automated Release (`.github/workflows/release.yml`) workflows for multi-platform binary compilation and starter-pack packaging.
* **Update**: Enhanced root `Makefile` with `fmt-check`, `vet`, `validate-examples`, and `validate-all` targets.
* **Update**: Synchronized `knowledge/roadmap/milestones.md` status table reflecting completion of Phases 1–6.

## 2026-08-27
* **Creation**: Added `okf bootstrap` command with embedded assets for 1-step project scaffolding, cross-platform release builds, and standalone starter bundle archiving (`make dist-bundle`).
* **Creation**: Created standardized agent skill (`skill/`), added `okf relate` command for relationship linking, finalized Convention v0.1, and built 3 complete example corpora (`examples/software`, `examples/coaching`, `examples/books`).
* **Update**: Reorganized root documentation into `docs/` and created root `README.md` and `AGENTS.md`.
* **Creation**: Implemented Go OKF core library (`pkg/okf`), standalone CLI (`cmd/okf`), and embedded MCP server (`okf mcp`) with in-memory BM25 search and automated bookkeeping.
* **Creation**: Added value proposition & core selling points concept `project/value-proposition.md`.
* **Creation**: Added architectural decision concept `architecture/tooling-decision.md` establishing Go as the single-binary foundation for CLI, In-Memory BM25 search, and built-in MCP server.
* **Update**: Added root `Makefile` with automatic JS-runner detection (`deno`, `node`, `bun`) to run `make validate` against the `knowledge/` bundle.
* **Creation**: Initialized the `okf-agent-memory` persistent knowledge corpus as an OKF v0.2 bundle. Added concepts for project overview, 5-layer architecture, core memory principles, knowledge review lifecycle, and roadmap milestones.
