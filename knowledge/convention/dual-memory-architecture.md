---
type: Convention
title: "Dual-Memory Agent Architecture & Agent Action Grammar"
description: Two-layer memory model separating normative working memory (Push/AAG) from semantic domain memory (Pull/OKF).
resource: "https://github.com/okf-memory/okf-agent-memory"
tags: [architecture, dmaa, aag, memory-model, instructions, context-efficiency, domain-neutral]
generated: { by: agent/mcp, at: "2026-10-04T20:09:36Z" }
status: stable
sources:
  - resource: https://github.com/okf-memory/okf-agent-memory/blob/main/docs/spec/DUAL_MEMORY_AGENT_ARCHITECTURE_RFC.md
    id: dmaa-rfc
    title: RFC Dual-Memory Agent Architecture v0.1
    last_modified: 2026-09-15
  - resource: https://github.com/okf-memory/okf-agent-memory/blob/main/docs/spec/AGENT_ACTION_GRAMMAR_RFC.md
    id: aag-rfc
    title: RFC Agent Action Grammar v0.1
    last_modified: 2026-09-15
  - resource: https://github.com/okf-memory/okf-agent-memory/blob/main/docs/guides/AGENT_INSTRUCTION_BEST_PRACTICES.md
    id: best-practices
    title: Best Practices for Agent Instructions in AGENTS.md
    last_modified: 2026-09-15
---

# Dual-Memory Agent Architecture (DMAA) & Agent Action Grammar (AAG)

The **Dual-Memory Agent Architecture (DMAA) v0.1** resolves the context-bloat and attention-drift dilemma across AI agents in all domains through a 2-layer cognitive model.[^dmaa-rfc]

```mermaid
flowchart TD
    subgraph PUSH["1. Normative Working Memory (Push Layer)"]
        direction TB
        C1["Canonical AGENTS.md (capped at 400 tokens)"]
        C2["Domain Codex (Rules, Style, Invariants)"]
        C3["OKF Memory Bridge (Deterministic Triggers)"]
        C4["Agent Action Grammar (AAG) Micro-Syntax"]
    end

    subgraph PULL["2. Semantic Domain Memory (Pull Layer)"]
        direction TB
        O1["OKF v0.2 Knowledge Bundle (knowledge/)"]
        O2["0 Tokens at baseline system prompt"]
        O3["Selective Retrieval via okf_search / okf_show"]
        O4["Persistent Graph of Decisions, Facts & Runbooks"]
    end

    INPUT["User Request"] --> PUSH
    PUSH -->|Enforces Domain Codex & Triggers| AGENT["AI Agent (LLM)"]
    AGENT -->|Selective Retrieval| PULL
    PULL -->|Context & Facts| AGENT
    AGENT --> OUTPUT["Deterministic Response"]
```

## The Universal Composition Model

In DMAA, project-level agent configuration is governed by a universal equation:

$$\text{AGENTS.md} = \text{Domain Codex} + \text{OKF Memory Bridge}$$

1. **Domain Codex (AAG)**: Project-specific behavioral invariants, tone, ethical boundaries, and hard constraints expressed in dense Agent Action Grammar.
2. **OKF Memory Bridge**: Standardized deterministic triggers directing the agent to pull from and synchronize with `knowledge/`.

## Layer 1: Normative Working Memory (Push Layer)
* **Location:** `AGENTS.md` at project root (symlinked to `CLAUDE.md`, `.cursorrules`, etc.).
* **Language:** **Agent Action Grammar (AAG)** — dense ASCII micro-syntax (`=>`, `ASSERT`, `!`, `MUST`, `IF`, `ON`) replacing verbose natural language prose (~78–85% token reduction).[^best-practices]
* **Budget:** Hard cap of **400 tokens** (rule `AAG-005`, `okf agents lint --budget`), measured on the managed block between `<!-- BEGIN OKF AGENT MEMORY -->` and `<!-- END OKF AGENT MEMORY -->`. Content outside the block is reported as total file tokens but is not gated, yet it is still loaded on every request.
* **Responsibility:** Invariant formatting, behavioral guardrails, and deterministic search-before-write triggers.

## Layer 2: Semantic Domain Memory (Pull Layer)
* **Location:** `knowledge/` OKF v0.2 bundle.
* **Footprint:** **0 tokens** in initial system prompt.
* **Responsibility:** Persistent domain facts, architectural decisions, runbooks, experimental logs, client records, and domain taxonomies retrieved on-demand.

## Multi-Domain Applicability

DMAA is completely domain-neutral and serves diverse industries and audiences:
- **Scientific & Academic Research**: Laboratory invariants, citation standards, data integrity gates; hypotheses, methodology logs, and findings stored in `knowledge/`.
- **Enterprise, Legal & Compliance**: Regulatory boundaries, confidentiality rules, audit requirements; corporate policies, contract precedents, and risk assessments in `knowledge/`.
- **Coaching & Consulting**: Pedagogical frameworks, diagnostic posture, conversational guardrails; client session histories and interventions in `knowledge/`.
- **Creative & Technical Writing**: Voice guidelines, stylistic rules, terminology bans; world-building lore, plot outlines, and character sheets in `knowledge/`.
- **Software Engineering**: Architectural guardrails, coding conventions, testing assertions; ADRs, schema designs, and operational runbooks in `knowledge/`.

## Inter-Concept Connections
* Extends the core principles in [principles](principles.md) with compact agent instruction standards.
* Complements the 5-layer architecture in [architecture/layers](../architecture/layers.md).
* Direct link to ecosystem value proposition in [project/value-proposition](../project/value-proposition.md).

[^dmaa-rfc]: RFC Dual-Memory Agent Architecture (DMAA) v0.1
[^best-practices]: Best Practices for Agent Instructions in AGENTS.md

# Related Concepts
- [Core Memory Principles & Agent Contract](principles.md): Specializes behavioral invariants into two cognitive memory layers
- [5-Layer System Architecture](../architecture/layers.md): Defines Layer 1 push working memory and Layer 2 pull domain memory
- [Why OKF Agent Memory (Value Proposition & Selling Points)](../project/value-proposition.md): Value proposition of token efficiency and domain neutrality
