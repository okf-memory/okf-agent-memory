## 2026-09-26
* **Update**: Linked `requirements/mutation-metadata.md` to `architecture/tooling-decision.md` (Specifies lifecycle and tag mutation behavior for the Go CLI and MCP surfaces.).
* **Creation**: Documented concept `requirements/mutation-metadata.md` (Metadata Mutation Parity for CLI and MCP).

## 2026-09-25
* **Refactoring**: Decoupled CLI subcommand implementations and command registry into `internal/cli` to keep `cmd/okf` lean and encapsulate command parsing.
* **Update**: Synchronized `code_refs` in `architecture/governance-model.md` and `architecture/zero-knowledge-vault-sync.md`.

## 2026-09-23
* **Release**: Published version v0.4.3 — Cross-platform absolute path evasion defense (`IsAbsPath`), multi-line YAML frontmatter smuggling defense in concept bodies, CLI validate findings taxonomy separation and summary counter reconciliation, and automated security audit discovery expansion.
* **Update**: Updated concept `convention/release-procedure.md`.
* **Update**: Updated concept `roadmap/milestones.md` adding Phase 18 (Cross-Platform Boundary Hardening & Validation Taxonomy).
* **Update**: Updated concept `convention/security-audit.md`.

## 2026-09-19
* **Release**: Published version v0.4.2 — MCP security hardening, argument boundary integrity and DoS length capping (`getStringArg`), 4MB stream-limiting stdin reader to prevent OOM attacks, copy-on-write concept update cache integrity, Unicode zero-width/BiDi Trojan Source protection with international character preservation, concept body frontmatter smuggling defense, directory nesting depth caps (`MaxConceptDirectoryDepth = 8`), and universal cross-platform backslash traversal sanitation.
* **Update**: Updated concept `roadmap/milestones.md` adding Phase 17 (MCP Boundary Integrity & Security Hardening).

## 2026-09-18
* **Update**: Linked `roadmap/milestones.md` to `convention/release-procedure.md` (Releases execute the milestones defined in the roadmap).
* **Update**: Linked `convention/contributing.md` to `convention/release-procedure.md` (Release preparation, quality gates, and git tagging procedure).
* **Creation**: Documented concept `convention/release-procedure.md` (Release Procedure & Distribution Runbook).
* **Release**: Published version v0.4.1 — Strict MCP specification conformance with object outputSchema and structuredContent payload (OpenCode & Pi agent), atomic filesystem write operations, index broken link detection, bidirectional index drift validation, and crash consistency test suites.
* **Update**: Updated concept `roadmap/milestones.md` adding Phase 16 (Strict MCP Conformance & Storage Hardening).
* **Update**: Updated concept `convention/security-audit.md` reframing Google Jules continuous audit workflow to defensive QA and negative unit testing.

## 2026-09-17
* **Release**: Published version v0.4.0 — Zero-Knowledge Vault Synchronization, client-side AES-256-GCM envelope encryption, Argon2id KDF, CAS blind sync protocol, atomic head concurrency control, 3-way reconcile engine, and OKF Memory Hub CLI suite with Bearer token authentication.
* **Update**: Updated concept `roadmap/milestones.md`.
* **Update**: Updated concept `architecture/zero-knowledge-vault-sync.md`.
* **Update**: Updated concept `convention/coding-standards.md`.
* **Update**: Updated concept `convention/contributing.md`.

## 2026-09-16
* **Release**: Published version v0.3.1 — Dual-Memory Agent Architecture (DMAA) empirical benchmark suite (`okf-benchmark`), CLI subcommand help handlers (`okf <subcommand> --help`), pre-flight GPU warmup ping, automated Mermaid diagram sanitizer, AAG v0.1 skill refactoring, and mutation performance hardening.

## 2026-09-15
* **Release**: Published version v0.3.0 — Agent Action Grammar (AAG) RFC & AST Linter (`AAG-001`–`AAG-005`), Dual-Memory Agent Architecture (DMAA), Multi-Domain Codex Scaffolding (`okf agents init`), SSoT Tool Symlinks (`okf agents link`), `okf validate --agents`, and Google Jules security remediation.
* **Update**: Updated `knowledge/roadmap/milestones.md` marking Phase 13 (DMAA & Agent Action Grammar) as completed.
* **Update**: Linked `convention/dual-memory-architecture.md` to `architecture/layers.md` (Defines Layer 1 push working memory and Layer 2 pull domain memory).
* **Update**: Linked `convention/dual-memory-architecture.md` to `convention/principles.md` (Specializes behavioral invariants into two cognitive memory layers).
* **Creation**: Documented concept `convention/dual-memory-architecture.md` (Dual-Memory Agent Architecture & Agent Action Grammar).
* **Documentation**: Reorganized `docs/` hierarchy into categorized subdirectories (`guides/`, `spec/`, `security/`, `project/`, `releases/`) and created central `docs/README.md` index. Added `docs/spec/DUAL_MEMORY_AGENT_ARCHITECTURE_RFC.md`, `docs/spec/AGENT_ACTION_GRAMMAR_RFC.md`, `docs/guides/AGENT_INSTRUCTION_BEST_PRACTICES.md`, and `docs/guides/LLM_INSTRUCTION_PATTERNS_CHEATSHEET.md`.

## 2026-09-14
* **Update**: Linked `architecture/zero-knowledge-vault-sync.md` to `architecture/layers.md` (Zero-knowledge sync extends the tooling and storage layers with client-side cryptography.).
* **Creation**: Documented concept `architecture/zero-knowledge-vault-sync.md` (Zero-Knowledge Vault Cryptography and Blind Sync Architecture).

## 2026-09-12
* **Release**: Published version v0.2.0 — Epistemic Governance & Code-to-Knowledge Binding Release.
* **Infrastructure**: Automated GitHub Actions release pipeline to ingest release notes from `docs/releases/${VERSION}.md` with fail-fast CI gate.
* **Community**: Achieved 100% GitHub Community Health with Contributor Covenant v2.1 `CODE_OF_CONDUCT.md`, `CONTRIBUTORS.md` acknowledgements, interactive avatar grid, GitHub Sponsors configuration, and Dependabot automation.
* **Update**: Updated `knowledge/roadmap/milestones.md` marking Phase 12 (Governance & Code Binding) as completed.

## 2026-09-11
* **Release**: Prepared major milestone Release v0.2.0 — Epistemic Governance & Code-to-Knowledge Binding Release integrating 3-tier authority model (constraint, hold, context), code_refs and scoped pre-edit discovery (--for-path in CLI and MCP), automated dogfooding parity test (TestDogfoodingAssetDrift), path traversal hardening (CWE-22), and community contributions from @krakozavr, @denis-samatov (#14-#17), and @dajiaohuang (#19).
* **Update**: Updated concept `convention/coding-standards.md` establishing the embedded asset synchronization invariant (`make sync-assets` and automated CI drift gate `TestDogfoodingAssetDrift`).
* **Update**: Linked `architecture/layers.md` to `architecture/governance-model.md` (Specifies the 3-tier epistemic governance model and code-to-knowledge binding).
* **Update**: Linked `architecture/governance-model.md` to `architecture/tooling-decision.md` (Implemented in deterministic Go CLI and MCP server).
* **Update**: Linked `architecture/governance-model.md` to `architecture/layers.md` (Defines Layer 2 governance policies and Layer 4 code binding).
* **Creation**: Documented concept `architecture/governance-model.md` (Governance vs. Execution Context and Code Binding).
* **Update**: Established Issue-First contribution policy and GitFlow branching strategy designating `develop` as default integration branch for PRs and reserving `main` strictly for tagged releases in `convention/contributing.md`, `CONTRIBUTING.md`, `.github/pull_request_template.md`, and `docs/RELEASE_PLAYBOOK.md`.

## 2026-09-10
* **Creation**: Documented concept `architecture/cli-argument-boundary.md` (CLI Optional Path Boundary).
* **Creation**: Documented concept `architecture/metadata-roundtrip.md` (Safe Unknown Metadata Round-Trip).
* **Creation**: Documented concept `architecture/relationship-identity.md` (Relationship Identity and Logging).
* **Creation**: Documented concept `architecture/search-tokenization.md` (Unicode and Deterministic Search).

## 2026-09-09
* **Release**: Published version v0.1.5 — Adversarial Security & DRY Hardening Release integrating autonomous Google Jules adversarial security loop, boundary symlink containment, central DRY metadata sanitization, actor whitespace fallback, self-relation loop prevention, and Jules workflow automation (`jules-review`, `jules-merge`).

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
