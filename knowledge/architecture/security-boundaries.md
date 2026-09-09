---
type: Decision
title: Bundle Isolation and Mutation Security Boundaries
description: Defensive security architecture enforcing canonical bundle boundaries, symlink containment, path traversal prevention, and frontmatter injection defense.
generated: { by: agent/mcp, at: 2026-09-08T07:51:29Z }
---

# Bundle Isolation and Mutation Security Boundaries

## Context & Problem Statement

OKF Agent Memory is designed to be written, updated, and queried by autonomous AI agents operating across IDEs and CI/CD pipelines. Because AI agent tool calls may execute untrusted or adversarial prompts, the deterministic tooling layer must treat all mutation arguments and bundle directories as untrusted inputs.

## Security Controls & Defense-in-Depth

The tooling layer enforces defensive confinement across four critical choke-points:

### 1. Canonical Bundle Confinement (`ensureWithinRoot`)
All file reads in `LoadBundle` and writes in `SaveConcept`, `UpdateParentIndex`, and `AppendLogEntry` resolve symlinks and compare canonical paths against the bundle root via `filepath.Rel`.
- Traversal via relative parent references (`..`) or absolute paths escaping the bundle directory is strictly denied.
- Root-reserved files (`index.md`, `log.md`, `AGENTS.md`) cannot be overwritten as arbitrary concept documents regardless of letter case (`strings.EqualFold`). Subdirectory concepts (e.g. `architecture/agents.md`) remain permissible.

### 2. Symlink Escape Prevention
To defend against Local File Inclusion (LFI) and Arbitrary File Overwrite:
- `LoadBundle` inspects symlinks with `ensureWithinRoot`. Any symlink whose resolved target escapes the canonical bundle root triggers an immediate traversal error.
- Symlinks must point strictly to markdown (`.md`) files within the bundle; symlinks to directories or non-markdown assets are rejected.
- Writing through symlinks pointing outside the bundle is blocked.

### 3. Frontmatter & Metadata Injection Defense (`sanitizeConceptMetadata`)
Metadata fields (`type`, `title`, `description`, `actor`) are strictly validated prior to serialization:
- Newline characters (`\r`, `\n`) are rejected in scalar metadata fields, preventing YAML attribute smuggling or forged verification states (`verified.by: human`).
- The YAML document delimiter (`---`) is forbidden within metadata values.
- Special scalar characters (`:`, `&`, `"`, etc.) are quoted safely in YAML serialization without escaping HTML entities.

### 4. MCP Server Root Confinement (`resolveBundleDir`)
The stdio Model Context Protocol (MCP) server confines dynamic bundle switching:
- The server initializes with a canonical `rootDir` (governed by project workspace or `OKF_MCP_ROOT`).
- Multi-bundle projects can dynamically select sub-bundles (e.g. `bundle="examples/software"`), but any request escaping the server root is denied before loading.
- Required MCP mutation arguments (`concept_id`, `type`, `title`, `description`) are strictly non-empty.

### 5. Collaborative File Permissions (`0o644` / `0o755`)
OKF bundles are designed for shared Git repository version control:
- Files are created with `0o644` (rw-r--r--) and directories with `0o755` (rwxr-xr-x).
- Static analysis rules intended for secret credential files (such as `gosec G301/G306` requiring `0600`/`0750`) are deliberately excluded, ensuring bundle readability across multi-user environments, CI/CD runners, and Git sub-processes.

## Relationships

- Defined as part of Layer 4 in [layers](layers.md).
- Complements the deterministic single-binary tooling in [tooling-decision](tooling-decision.md).

# Related Concepts
- [MCP Tool Security & Untrusted Agent Input](../convention/mcp-agent-safety.md): Behavioral MCP guidelines complement deterministic boundaries
- [Automated Security Auditing & Jules Remediation Workflow](../convention/security-audit.md): Proactive adversarial auditing and verification pipeline
