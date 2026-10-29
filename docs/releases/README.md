# Release History

Historical release notes for **OKF Agent Memory**.

| Version | Release Date | Highlights |
| :--- | :--- | :--- |
| [**v0.8.0**](./v0.8.0.md) | 2026-10-29 | Bundle Scope Isolation (`scopes` in root `index.md`), Fail-Closed Scope Validation, Removal of `--scope bundle` Alias (breaking) |
| [**v0.7.2**](./v0.7.2.md) | 2026-10-28 | Remote Path Containment in Hub Sync & Registry Installs (#64), Staged Registry Rollbacks, Symlinked CWD Root Resolution (#62, PR #63) |
| [**v0.7.1**](./v0.7.1.md) | 2026-10-27 | Nearest-First Bare ID Scope Fallback in `show` (#47, PR #61), Custom Metadata Preservation in `sources` (#57), Relative Nested MCP Bundle Resolution (#58), QF1008 Linter Cleanup |
| [**v0.7.0**](./v0.7.0.md) | 2026-10-20 | Bundle Quality Scorecards (Phase 21), Pre-Pull Quality Gating (`--min-score`) with Atomic Rollback, Scorecard JSON Telemetry, Duplicate-Safe Date Heading Mutations |
| [**v0.6.2**](./v0.6.2.md) | 2026-10-19 | Semantic 3-Way `index.md` Merge in Hub Sync (#52, PR #55), Composite BM25 IDF Across Vendor Packages (#54), Numbered `Related Concepts` Heading Support, Starter-Pack Archive Retirement, AAG Commit Codex |
| [**v0.6.1**](./v0.6.1.md) | 2026-10-12 | Hub Sync State Persistence (#51, PR #53), BM25 Scoring Symmetry & Acronym Suppression (#42), Short-Stem Exact Matching (< 4 runes), Metadata TF Caps, Update Tooling Targets |
| [**v0.6.0**](./v0.6.0.md) | 2026-10-04 | Symlinked Bundle Roots (#45), Typed List Frontmatter (#49), List-Aware `--filter` (#48), Strict Filter Keys (breaking: `tag`, `code_ref`, `desc` aliases removed), Human Verification Provenance Guard, Quoted-Key Smuggling Defense |
| [**v0.5.0**](./v0.5.0.md) | 2026-10-01 | OKF Registry, Dependency Locking (okf.lock), Multi-Scope Layering (closes #11), okf pull/restore/vendor |
| [**v0.4.4**](./v0.4.4.md) | 2026-09-27 | Metadata Mutation Parity, Core Decoupling, Query Filters, Drift Link Resolution |
| [**v0.4.3**](./v0.4.3.md) | 2026-09-23 | Absolute Path Evasion Defense, Frontmatter Smuggling Defense, Validate Taxonomy Reconciliation |
| [**v0.4.2**](./v0.4.2.md) | 2026-09-19 | MCP Security Hardening, Boundary Integrity, ReDoS/OOM Protection, Unicode BiDi & Trojan Source Defenses |
| [**v0.4.1**](./v0.4.1.md) | 2026-09-18 | Strict MCP Conformance (OpenCode, Pi Agent), Crash Consistency, Atomic Writes, Index Broken Links, Bidirectional Drift |
| [**v0.4.0**](./v0.4.0.md) | 2026-09-17 | Zero-Knowledge Vault Synchronization, Argon2id KDF, AES-256-GCM Envelope Encryption, 3-Way Reconcile Engine, OKF Hub CLI |
| [**v0.3.1**](./v0.3.1.md) | 2026-09-16 | DMAA Benchmark Engine, CLI Subcommand Help Handlers, Pre-Flight Warmup, Mermaid Sanitizer, AAG Skill Alignment |
| [**v0.3.0**](./v0.3.0.md) | 2026-09-15 | Agent Action Grammar (AAG) RFC & Linter, Dual-Memory Agent Architecture (DMAA), Multi-Domain Codex, SSoT Tool Symlinks |
| [**v0.2.0**](./v0.2.0.md) | 2026-09-12 | 3-Tier Epistemic Governance, `code_refs`, `--for-path`, CWE-22 Security Hardening, Dogfooding Invariants |
| [**v0.1.5**](./v0.1.5.md) | 2026-09-09 | Adversarial Security & DRY Hardening, Google Jules integration, boundary symlink containment |
| [**v0.1.4**](./v0.1.4.md) | 2026-09-08 | Cross-platform Windows relative link resolution, open actor family prefixes, dot-directory scans |
| [**v0.1.3**](./v0.1.3.md) | 2026-09-08 | Security Hardening Release (CWE-22 path traversal prevention, symlink defenses, YAML sanitization) |
| [**v0.1.2**](./v0.1.2.md) | 2026-09-06 | Validator trust-ordering enforcement, opt-in `--stale` gate |
| [**v0.1.1**](./v0.1.1.md) | 2026-09-06 | MCP JSON-RPC 2.0 compliance, dynamic multi-bundle resolution, Homebrew formula automation |
| [**v0.1.0**](./v0.1.0.md) | 2026-09-05 | Initial release of OKF Agent Memory (pure Go engine, sub-300µs BM25 search, embedded stdio MCP) |
