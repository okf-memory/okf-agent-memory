---
type: Project
title: OKF Agent Memory Overview
description: A domain-neutral persistent project-memory system for AI agents based on the Open Knowledge Format (OKF) v0.2.
resource: https://okf-memory.dev
tags: [agent-memory, okf, architecture, open-standard, okf-memory-dev]
generated: { by: agent/gemini-3.7-flash, at: 2026-09-01T09:43:00Z }
status: stable
sources:
  - id: convention
    resource: ../../docs/CONVENTION.md
    title: OKF Agent Memory Convention v0.1
    last_modified: 2026-08-27
  - id: roadmap
    resource: ../../docs/ROADMAP.md
    title: OKF Agent Memory Project Roadmap
    last_modified: 2026-08-27
  - id: spec
    resource: https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md
    title: Open Knowledge Format (OKF) v0.2 Specification
---

# OKF Agent Memory

The **OKF Agent Memory** project provides an open, vendor-neutral, and domain-agnostic persistent memory layer for AI agents, built directly on Google's Open Knowledge Format (OKF) v0.2.[^spec]

## Problem & Vision

Conversations with AI agents are temporary and ephemeral. When a context window resets or a new session starts, valuable discoveries, architectural decisions, and domain facts are lost unless stored systematically.[^convention]

Traditional solutions often rely on proprietary vector databases, locked memory APIs, or unstructured scratchpads. OKF Agent Memory solves this by treating an OKF knowledge bundle inside the repository (`knowledge/`) as the single source of persistent truth.

```mermaid
flowchart LR
    A[AI Agent] -->|Reads / Queries| K[knowledge/ bundle<br/>OKF v0.2]
    A -->|Discovers & Persists| K
    H[Human Developer] -->|Inspects & Edits| K
```

## Key Capabilities

- **Domain-Neutral**: Functions equally well for software engineering, coaching, research, literature reviews, and operations.[^convention]
- **Core Value Proposition**: Key differentiators and selling points detailed in [value-proposition](value-proposition.md).
- **Deterministic Tooling & Separation of Concerns**: Described in detail in [architecture/layers](../architecture/layers.md).
- **Principled Knowledge Lifecycle**: Defined in [convention/principles](../convention/principles.md) and [convention/lifecycle](../convention/lifecycle.md).
- **Phased Implementation**: Detailed in the [roadmap/milestones](../roadmap/milestones.md).

[^spec]: Open Knowledge Format (OKF) v0.2 Specification
[^convention]: OKF Agent Memory Convention v0.1
