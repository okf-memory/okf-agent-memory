---
type: Decision
title: Governance vs. Execution Context and Code Binding
description: "3-tier epistemic governance model (constraint, hold, context) and code-to-knowledge binding via code_refs."
tags: [governance, authority, code-refs, binding, architecture, decision]
generated: { by: agent/cli, at: "2026-09-11T15:37:43Z" }
governance: constraint
code_refs: [pkg/okf/types.go, pkg/okf/search.go, pkg/okf/validator.go, cmd/okf/main.go, cmd/okf/mcp.go]
---

# Governance vs. Execution Context and Code Binding

## Context & Problem Statement

In LLM-assisted software engineering, project knowledge often degrades into an undifferentiated flat list of text documents. When an AI agent prepares to edit a file, it suffers from two major failure modes:
1. **Blind Modification (Lack of Code Binding)**: The agent modifies a source file without realizing that existing architecture decisions (ADRs) or coding conventions explicitly govern that file.
2. **Authority Blindness (Governance Failure)**: The agent cannot distinguish between an informative background fact ("we used to use PostgreSQL in 2023"), an immutable architectural guardrail ("pure standard library Go, zero external dependencies"), and an active freeze ("auth subsystem refactor in progress: do not touch without human signoff").

To solve this, OKF introduces **Code-to-Knowledge Binding (`code_refs`)** and an explicit **3-Tier Epistemic Governance Model (`governance: constraint | hold | context`)**.

---

## The 3-Tier Epistemic Governance Model

Governance defines the **operational authority and bindingness** of a concept over AI agent actions:

| Governance | Agent Authority & Behavioral Directive | Context Injection Behavior |
| :--- | :--- | :--- |
| **`constraint`** | **Mandatory Guardrail / Directive**. The agent **may** modify code, but **must strictly adhere** to the constraints outlined in this document (e.g. zero external dependencies, pure Go). | Injected as a hard constraint into the agent prompt/instruction budget. |
| **`hold`** | **Execution Freeze / Gatekeeper**. The referenced code paths are in flux, undergoing migration, or under security lockdown. The agent **must NOT modify** the referenced code files without explicit live human authorization. | Injected into context as a prominent warning/barrier to halt unapproved edits. |
| **`context`** | **Domain Knowledge / Advisory**. Background information, data models, or system rationale. Non-restrictive; provides domain understanding without limiting actions. | Available for retrieval and reasoning without gating actions. |

### Clean Separation: `status` vs. `governance`

- **`status` (`draft | active | deprecated | superseded`)**: Governs the **document lifecycle** within the OKF specification.
- **`governance` (`constraint | hold | context`)**: Governs the **epistemic authority and permissions** of an AI agent touching code.

### Zero-Boilerplate Implicit Inference

To eliminate YAML boilerplate, concepts derive their governance level automatically if the frontmatter omits `governance`:
1. If `governance` is explicitly declared (`constraint`, `hold`, or `context`), it is respected (normalized to lowercase).
2. Concepts located under `knowledge/convention/` default implicitly to `constraint`.
3. All other concepts (such as `knowledge/architecture/` or `knowledge/project/`) default implicitly to `context`.

---

## Code-to-Knowledge Binding (`code_refs`)

Concepts declare the source files or directories they govern via the `code_refs` frontmatter list:

```yaml
---
id: architecture/governance-model
type: Decision
title: Governance vs. Execution Context and Code Binding
governance: constraint
code_refs:
  - pkg/okf/*.go
  - cmd/okf/main.go
---
```

### Pattern Matching Capabilities

The deterministic matching engine in `pkg/okf/search.go` supports:
- **Exact File Match**: `pkg/okf/types.go`
- **Directory Prefix**: `pkg/okf/` (matches any file under `pkg/okf/`)
- **Standard Glob**: `pkg/okf/*.go` (via `path.Match`)
- **Recursive Wildcard**: `pkg/**/*.go` or `**/*.go`

---

## Pre-Edit Discovery Workflows

Before an AI agent or developer modifies code, they query the knowledge base for all governing rules:

### CLI Usage:
```bash
# Discover all constraints, holds, and context governing a file:
okf search --for-path pkg/okf/types.go

# Output:
# Found 2 matching concept(s) governing 'pkg/okf/types.go' in 'knowledge':
#  1. [constraint] [10.00] architecture/governance-model (Decision)
#     3-tier epistemic governance model (constraint, hold, context) and code-to-knowledge binding via code_refs.
#     Matches: code_refs
```

### MCP Tool Usage:
AI agents call `okf_search` with the optional `for_path` argument:
```json
{
  "name": "okf_search",
  "arguments": {
    "for_path": "pkg/okf/types.go"
  }
}
```

Results are deterministically ordered:
1. `hold` concepts first (highest priority: stop-the-line).
2. `constraint` concepts second (mandatory guardrails).
3. `context` concepts third (informational).
4. Relevance score / tie-breaking by Concept ID.

---

## Code Drift Validation

When running validation with `--drift`, OKF verifies that non-glob paths declared in `code_refs` actually exist in the repository:
```bash
okf validate knowledge --strict --drift
```
If a ref points to a deleted or moved file, a drift warning is raised, ensuring documentation remains synchronized with active code.

---

## Related Concepts

- [5-Layer System Architecture](layers.md): Defines Layer 2 governance policies and Layer 4 code binding.
- [Go Single-Binary CLI & MCP Architecture Decision](tooling-decision.md): Implemented in deterministic Go CLI and MCP server.
