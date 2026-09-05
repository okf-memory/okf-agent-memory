---
name: okf-agent-memory
description: Maintain persistent, domain-neutral project memory for AI agents using Open Knowledge Format (OKF) v0.2 bundles and the deterministic okf Go toolchain. Use whenever project knowledge, decisions, runbooks, research, client notes, or domain discoveries must survive conversational resets.
---

# OKF Agent Memory Skill

This skill teaches AI agents how to interact with an **Open Knowledge Format (OKF) v0.2** knowledge bundle (by default located at `knowledge/`) as the persistent project memory.

---

## 1. Minimal Agent Contract

Every agent operating in this repository MUST obey the following contract:

1. **Persistent knowledge lives in the OKF corpus**: Conversations are temporary; the `knowledge/` directory survives.
2. **Search Before Write**: Always query existing knowledge before authoring new concepts.
3. **No Blanket Scans**: Never use `list_dir`, `grep`, or dump `knowledge/` in bulk. Query via `okf search` and load concepts on demand via `okf show`.
4. **Prefer Update Over Duplication**: Expand existing concepts when related facts emerge.
5. **No Conversational Noise**: Never store scratchpads, raw chain-of-thought, or speculative chatter.
6. **Preserve Trust & Provenance**: Always record `sources` and `generated: { by, at }`. Never mark AI content as `human:` verified.
7. **End-of-Task Review**: Perform a knowledge review after completing substantial work.
8. **Always Validate**: Ensure `make validate` or `okf validate` passes with 0 errors and 0 warnings.

---

## 2. Tooling Commands Reference

Use the `okf` CLI or Makefile targets for deterministic operations:

| Task | Command | JSON Mode for Agents |
| :--- | :--- | :--- |
| **Search Knowledge** | `okf search "<query>" knowledge` | `okf search "<query>" knowledge --json` |
| **Inspect Concept** | `okf show <concept-id> knowledge` | `okf show <concept-id> knowledge --json` |
| **Create Concept** | `okf create <id> knowledge --type <type> --title "<title>" --desc "<summary>"` | Add `--json` |
| **Update Concept** | `okf update <id> knowledge --desc "<new-summary>"` | Add `--json` |
| **Relate Concepts** | `okf relate <src-id> <target-id> knowledge --desc "<prose>"` | Add `--json` |
| **Validate Bundle** | `okf validate knowledge --strict --drift` | Add `--json` |

---

## 3. Workflow Stages

1. **Discovery & Exploration**: Follow [discovery.md](./discovery.md) to locate relevant existing knowledge without blowing up context.
2. **Evaluation & Persistence**: Follow [remember.md](./remember.md) to decide what to persist vs. discard.
3. **Updating & Conflict Handling**: Follow [update.md](./update.md) when modifying existing concepts.
4. **Relationship Building**: Follow [relationships.md](./relationships.md) to interlink concepts cleanly.
5. **Worked Examples**: Inspect [examples.md](./examples.md) for software, coaching, and literature scenarios.
