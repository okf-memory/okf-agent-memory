# Architecture

* [5-Layer System Architecture](layers.md) - Structural separation of concerns across the OKF specification, agent convention, skills, deterministic tooling, and knowledge corpus.
* [Go Single-Binary CLI & MCP Architecture Decision](tooling-decision.md) - Architectural decision to implement the deterministic OKF tooling layer as a standalone Go binary with dual CLI and MCP support.
* [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md) - Defensive security architecture enforcing canonical bundle boundaries, symlink containment, path traversal prevention for local mutations and for untrusted remote hub, sync and registry input, and frontmatter injection defense.
* [Unicode and Deterministic Search](search-tokenization.md) - Search tokenizes Unicode letters and digits, enforces symmetrical tokenized DF/TF with exact matching for short stems, and caps metadata frequencies.
* [Relationship Identity and Logging](relationship-identity.md) - Relationships are identified by target path and description, making retries idempotent while retaining distinct relationship contexts.
* [Safe Unknown Metadata Round-Trip](metadata-roundtrip.md) - Unknown frontmatter keys are serialized deterministically with safe quoting and JSON-compatible scalar and collection preservation.
* [CLI Optional Path Boundary](cli-argument-boundary.md) - CLI commands consume an optional bundle or target path only from the first remaining argument, preserving all subsequent flag values.
* [Governance vs. Execution Context and Code Binding](governance-model.md) - 3-tier epistemic governance model (constraint, hold, context) and code-to-knowledge binding via code_refs.
* [Zero-Knowledge Vault Cryptography and Blind Sync Architecture](zero-knowledge-vault-sync.md) - Client-side zero-knowledge AES-256-GCM envelope encryption, Argon2id KDF, CAS blind storage, 3-way reconcile sync protocol, and Bearer auth.
* [OKF Registry Client, Dependency Locking, and Multi-Scope Vendor Layering](registry-and-vendor-layering.md) - Decentralized package registry client, zero-dependency okf.lock manifest, multi-scope priority layering, and hermetic @ cross-scope URI routing.
* [Retirement of Standalone Starter-Pack Distribution Archives](starter-pack-retirement.md) - Retirement of standalone starter-pack zip and tar.gz archives in favor of okf bootstrap CLI scaffolding.
* [Semantic Merge of Bookkeeping Files During Hub Sync](semantic-bookkeeping-merge.md) - During hub sync, log.md is merged by entry union and index.md by a 3-way merge per concept listing, so concurrent disjoint creates in one folder converge instead of colliding.
* [Bundle Quality Scorecards and Pre-Pull Verification](bundle-quality-scorecards.md) - Establishes a 3-pillar quality scorecard in okf validate, okf pull, and okf restore (Epistemic Maintenance, Human Stewardship, Conformance Integrity) without redundant CLI commands.
* [OKF v0.3 Conformance, Lifecycle Forwarding & Freshness Architecture](okf-v03-conformance.md) - Architectural design for OKF v0.3 conformance, v0.2 fallbacks, trust tiers, unified file-relative path resolution, and computed Git freshness signals.
