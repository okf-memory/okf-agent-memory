---
type: Decision
title: "OKF v0.3 Conformance, Lifecycle Forwarding & Freshness Architecture"
description: "Architectural design for OKF v0.3 conformance, v0.2 fallbacks, trust tiers, unified file-relative path resolution, and computed Git freshness signals."
tags: [okf-v0.3, conformance, trust-tiers, paths, extensions, staleness]
generated: { by: agent/mcp, at: "2026-11-21T10:57:41Z" }
status: stable
---

# OKF v0.3 Conformance, Lifecycle Forwarding & Freshness Architecture

`okf-agent-memory` implements full **Open Knowledge Format (OKF) v0.3** conformance while retaining backward-compatible fallbacks for existing v0.2 bundles (`okf_version: "0.2"` or omitted).

## 1. Root `index.md` Metadata & Version Detection
- Root `index.md` frontmatter fields (`okf_version`, `title`, `description`, `license`) and unknown extension keys are parsed into first-class `Bundle` metadata and preserved verbatim across `init`, `create`, `update`, and `sync`.
- `(*Bundle).IsV02OrEarlier()` gates v0.2 compatibility fallbacks without breaking existing bundles.

## 2. Collision-Free `x-` Extensions & Fault-Tolerant Parsing
- Extension keys (`x-<producer>-<name>`) are supported at root frontmatter and nested levels (`generated`, `verified`, `sources`, `parameters`, `executor`, `attester`) and survive `Parse -> Edit -> Serialize` roundtrips losslessly via `Extra` maps.
- Markdown files without frontmatter or without `type:` (e.g. `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `SKILL.md`) are skipped cleanly without errors or orphan warnings; non-Markdown files are indexed as linkable bundle assets (`Bundle.Assets`).
- Consumer parsing handles `null`, empty strings, and scalar-where-list-expected inputs gracefully while `okf validate` surfaces `ParseWarnings`.

## 3. Lifecycle Forwarding (`superseded_by`)
- Deprecated concepts may declare `superseded_by` pointing to their replacement.
- `superseded_by` is indexed as a directed graph edge (preventing false orphan reports), validated for target existence and cycles (`detectSupersededCycle`), and followed safely by `okf show` and `okf search` (`ResolveSuperseded`, `SupersededFrom`).

## 4. Trust Tiers (`verified.at >= generated.at`) & Actor IDs
- `TrustTier(c)` evaluates only **current** verifications (`verified.at >= generated.at`, or all if `generated.at` is absent): `human:` yields `human-reviewed`, other valid actors (`process:`, `team:`, `<producer>/<version>`) yield `machine-confirmed`, otherwise `unverified`.
- Historical verifications (`verified.at < generated.at`) remain in the file as audit history without emitting a `GateFinding`, while Scorecard Pillar 2 awards 100 points only for current human verification or human authorship.
- `team:` is accepted by `IsValidActor` but never satisfies `IsHumanIdentity`.

## 5. Unified File-Relative Path Resolution (`ResolvePath`)
- A single resolver (`(*Bundle).ResolvePath`) handles both Markdown body links and frontmatter path fields (`resource`, `superseded_by`, `sources[].resource`, `computation`, `executor.resource`, `attester.resource`):
  - RFC 3986 scheme -> external URL
  - Leading `/` -> bundle root
  - Otherwise -> relative to containing file's directory (with bundle-root fallback when `IsV02OrEarlier()` is true)
- Strict CWE-22 path-traversal confinement denies any escape above the bundle root.

## 6. Runtime Staleness & Progressive Git Freshness
- Without adding any frontmatter keys, `okf search`, `okf show`, and MCP `okf_search`/`okf_show` compute `trust_tier` and `staleness` (`last_touched_at`, optional `commit`, `provider`).
- Default mode uses frontmatter `generated.at` / file mtime (`provider: "frontmatter"`) with zero subprocess forks.
- Opt-in `--with-git-freshness` / `with_git_freshness` enriches via a single batched `git log` (`provider: "git"`), and `okf validate --drift-git` flags concepts whose `code_refs` have newer Git commits than the concept's `last_touched_at`.

# Related Concepts
- [5-Layer System Architecture](layers.md): Defines the OKF specification and deterministic tooling layers.
- [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md): Enforces CWE-22 confinement and write protection on protected concepts.
- [Go Single-Binary CLI & MCP Architecture Decision](tooling-decision.md): Hosts the CLI and MCP surfaces implementing v0.3 conformance.
