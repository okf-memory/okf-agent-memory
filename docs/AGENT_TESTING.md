# Multi-Agent Testing & Evaluation Scenarios (Phase 7)

This document specifies the evaluation protocol, automated test scenarios, and compatibility benchmarks for AI agents operating on **OKF Agent Memory** repositories.

---

## 1. Objective & The Definition of Done

The primary goal of OKF Agent Memory is domain-neutral persistent intelligence:

> **The Ultimate Test:** Can a completely new AI agent enter an existing project and continue the work effectively using the repository and its OKF knowledge corpus alone—without access to past conversation transcripts?

To satisfy this, an agent must reliably execute the **10-Step Definition of Done**:

1. Discover existing project knowledge via `okf search` or root `knowledge/index.md`.
2. Understand relevant concepts and their link graphs.
3. Work on a coding or domain task.
4. Detect newly emerged durable facts or architectural decisions.
5. Search before writing to avoid concept duplication.
6. Create or update concepts with structured frontmatter.
7. Preserve provenance (`generated:` attribution).
8. Preserve history in `log.md` and parent indexes.
9. Validate the resulting bundle (`okf validate --strict`).
10. Hand over clean context to subsequent sessions.

---

## 2. Test Scenarios Suite

| ID | Scenario Name | Test Objective | Pass Criteria |
| :--- | :--- | :--- | :--- |
| **TC-01** | **New Project Bootstrap** | Agent is instructed to scaffold memory in a fresh repository. | `okf bootstrap` executed; valid `knowledge/` bundle, `AGENTS.md`, and skills created; `validate` returns 0 errors. |
| **TC-02** | **Existing Project Discovery** | Agent is asked a question about past architecture without context. | Agent queries `okf search`, locates the relevant concept, and cites it in the answer. |
| **TC-03** | **Repeated Task (Anti-Duplication)** | Agent is instructed to update a database timeout that already exists in memory. | Agent uses `okf update` (or modifies existing concept); does NOT create `timeout-v2.md` or duplicates. |
| **TC-04** | **Contradiction Detection** | Agent is given requirements conflicting with a documented architectural decision. | Agent flags the conflict against memory before overwriting or proceeding blindly. |
| **TC-05** | **Human Correction Preservation** | A human adds a `verified:` block or manual override. | Agent preserves the `verified:` block without overwriting it as machine-generated. |
| **TC-06** | **Multi-Session Evolution** | 5 consecutive independent tasks simulating real development across sessions. | Knowledge graph grows organically without broken links, orphaned concepts, or drift. |
| **TC-07** | **Large Corpus Progressive Disclosure** | Large bundle (100+ concepts). | Agent queries BM25 and reads specific files instead of attempting to load all 100+ files into LLM context. |

---

## 3. Detailed Scenario Walkthroughs & Evaluation Rubrics

### Scenario 1: New Project Bootstrap (`TC-01`)
* **Prompt**: *"Initialize a new Go project and set up persistent memory for AI agents."*
* **Expected Agent Behavior**:
  1. Invokes `okf bootstrap . --name "<Project>"` or `make dist-bundle`.
  2. Runs `okf validate knowledge --strict`.
  3. Commits `knowledge/`, `AGENTS.md`, and `.agents/skills/okf-memory/`.

### Scenario 2: Existing Project Discovery (`TC-02`)
* **Prompt**: *"What authentication scheme is used by our API Gateway?"*
* **Expected Agent Behavior**:
  1. Does NOT guess or state "I don't have that information in my context".
  2. Executes `okf search "authentication API gateway" knowledge`.
  3. Inspects `okf show <concept-id> knowledge`.
  4. Returns the exact factual answer with concept reference.

### Scenario 3: Repeated Task & Anti-Duplication (`TC-03`)
* **Prompt**: *"We increased the Redis session TTL from 15 minutes to 30 minutes. Please record this in memory."*
* **Expected Agent Behavior**:
  1. Executes `okf search "Redis session TTL" knowledge`.
  2. Discovers existing `decisions/session-ttl.md`.
  3. Updates the existing concept (`okf update decisions/session-ttl --desc "..."`).
  4. Appends a change entry in `knowledge/log.md`.

### Scenario 4: Human Correction Preservation (`TC-05`)
* **Given Concept**:
  ```yaml
  ---
  type: Architecture
  title: Storage Engine
  status: stable
  verified:
    - by: human/lead-architect
      at: 2026-08-20T09:00:00Z
  ---
  ```
* **Prompt**: *"Add support for BadgerDB as secondary storage."*
* **Expected Agent Behavior**:
  1. Updates the body or tags of the concept.
  2. **Preserves** the existing `verified:` block.
  3. Adds its own `generated:` block without claiming human authority.

---

## 4. Multi-Agent Compatibility Matrix

| Agent Platform | Interface Mode | Setup & Capabilities | Compatibility Status |
| :--- | :--- | :--- | :--- |
| **Antigravity / Gemini 3.7 Flash** | CLI / Go Toolchain + Custom Skills | Full automated bookkeeping, sub-second search, strict validation. | 🟢 **Verified (Reference)** |
| **Claude Code** | Native CLI + Embedded MCP (`okf mcp`) | Fast tool calling via MCP stdio, automated search before write. | 🟢 **Supported** |
| **Cursor / Windsurf** | MCP Server (`okf mcp knowledge`) | Background indexing, in-editor tool invocation for memory actions. | 🟢 **Supported** |
| **OpenAI Codex / CLI** | Shell CLI (`bin/okf`) | Executes commands via `AGENTS.md` instructions. | 🟢 **Supported** |
| **Local LLMs (Ollama / Llama-3)** | Shell CLI (`okf --json`) | Machine-readable JSON output reduces parsing ambiguity for smaller models. | 🟢 **Supported** |

---

## 5. Automated Scenario Test Suite

Automated end-to-end scenario simulations are run as part of the Go test suite:

```bash
# Run all scenario integration tests
go test -v ./pkg/okf -run TestScenario
```

This verifies that the entire lifecycle (bootstrap $\rightarrow$ discovery $\rightarrow$ mutation $\rightarrow$ relationship $\rightarrow$ conflict $\rightarrow$ validation) executes deterministically without error.
