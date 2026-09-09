---
type: Architecture
title: 5-Layer System Architecture
description: Structural separation of concerns across the OKF specification, agent convention, skills, deterministic tooling, and knowledge corpus.
resource: https://github.com/okf-memory/okf-agent-memory
tags: [architecture, layers, design, tooling]
generated: { by: agent/mcp, at: 2026-09-08T07:42:55Z }
status: stable
sources:
  - resource: ../../docs/CONVENTION.md
    id: convention
    title: OKF Agent Memory Convention v0.1
    last_modified: 2026-08-27
  - resource: ../../docs/ROADMAP.md
    id: roadmap
    title: OKF Agent Memory Project Roadmap
    last_modified: 2026-08-27
---

# 5-Layer System Architecture

OKF Agent Memory strictly separates semantic reasoning from syntax handling through a 5-layer stack.[^roadmap]

```mermaid
flowchart TD
    L1["1. OKF v0.2 Specification<br/>(Normative Markdown & YAML Format)"]
    L2["2. Agent Memory Convention<br/>(Behavioral Rules: Search, Review, Trust)"]
    L3["3. Agent Skill<br/>(LLM Prompts & Operational Workflows)"]
    L4["4. Tooling Layer: Go Library & CLI<br/>(Deterministic Parsing, Serialization, Validation)"]
    L5["5. Project Knowledge Corpus<br/>(knowledge/ OKF Bundle)"]

    L1 --> L2
    L2 --> L3
    L3 --> L4
    L4 --> L5
```

## Layer Responsibilities

### 1. OKF v0.2 Specification
The data representation contract. Defines concept frontmatter (`type`, `sources`, `generated`, `verified`, `status`), index files, and `log.md`.

### 2. Agent Memory Convention
Defines agent behavior around the format without altering OKF syntax.[^convention] Governed by [convention/principles](../convention/principles.md) and [convention/lifecycle](../convention/lifecycle.md).

### 3. Agent Skill
Teaches an AI agent *how* and *when* to execute memory operations (e.g. running search queries, discovering relevant concepts, structuring updates). Connects to the broader project as described in [project/overview](../project/overview.md).

### 4. Tooling Layer (Go Library & CLI)
Ensures format correctness, validates links, maintains index files, and manages frontmatter metadata deterministically without placing parsing burdens on the LLM, as decided in [tooling-decision](tooling-decision.md). Tracked under [roadmap/milestones](../roadmap/milestones.md).

### 5. Project Knowledge Corpus
The version-controlled `knowledge/` directory holding the actual durable concepts, cross-links, and change log.

[^roadmap]: OKF Agent Memory Project Roadmap
[^convention]: OKF Agent Memory Convention v0.1

# Related Concepts
- [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md): Layer 4 tooling enforces security boundaries and bundle isolation
- [Engineering & Coding Best Practices (Clean Code, TDD, DRY)](../convention/coding-standards.md): Coding standards governing Layer 4 Go tooling development
