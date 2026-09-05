---
type: Decision
title: Go Single-Binary CLI & MCP Architecture Decision
description: Architectural decision to implement the deterministic OKF tooling layer as a standalone Go binary with dual CLI and MCP support.
resource: https://github.com/okf-memory/okf-agent-memory
tags: [decision, architecture, go, cli, mcp, tooling]
generated: { by: agent/gemini-3.7-flash, at: 2026-08-27T11:39:00Z }
status: stable
sources:
  - id: layers
    resource: layers.md
    title: 5-Layer System Architecture
  - id: milestones
    resource: ../roadmap/milestones.md
    title: Project Roadmap & Development Milestones
---

# Go Single-Binary CLI & MCP Architecture Decision

## Context & Problem Statement

The OKF Agent Memory system requires a deterministic tooling layer (Layer 4 in [layers](layers.md)) to handle parsing, validation, search, and bundle modifications without burdening the LLM with syntax or serialization errors.

The choice of implementation technology directly affects execution speed, cross-environment portability, and friction when integrated with diverse AI agent runtimes.

## Evaluated Alternatives

1. **Go (Single Binary, Compiled)**: Zero dependencies, instant startup (<5ms), portable binary distribution, embedded MCP server.
2. **Python (`pip` / `venv`)**: High accessibility for data/AI users, but runtime dependency overhead, virtualenv management issues, and slow CLI cold starts (100-300ms).
3. **Node.js / TypeScript**: Easy integration with web visualizers, but requires host Node/Deno runtimes.

## Decision & Agreed Design

The project adopts **Go** as the foundation for the `okf` core library, CLI, and MCP server:

- **Dual-Interface Distribution**: A single executable provides both standard CLI commands (`okf validate`, `okf search --json`, `okf create`) and a native Model Context Protocol server via `okf mcp` over stdio.
- **In-Memory BM25 & Graph Search**: Fast lexical and structural search across frontmatter tags, descriptions, titles, and graph connections without requiring external vector databases or heavy models.
- **Automated Bookkeeping**: `okf create` and `okf update` maintain `log.md` and `index.md` entries by default to eliminate manual bookkeeping drift, with opt-out flags (`--no-log`, `--no-index`).

## Consequences & Next Steps

This decision governs Phase 3 (Go Library), Phase 4 (CLI), and Phase 5 (MCP Interface) of the [roadmap/milestones](../roadmap/milestones.md).
