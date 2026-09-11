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
3. **Check Governance by Scope Before Editing**: Query governing concepts via `okf_search(for_path="<path>")` (or `okf search --for-path <path>`) once before starting substantial work on a module/directory to discover constraints or active holds (avoid redundant per-file calls).
4. **No Blanket Scans**: Never use `list_dir`, `grep`, or dump `knowledge/` in bulk. Query via `okf_search` (or `okf search`) and load concepts on demand via `okf_show` (or `okf show`).
5. **Prefer Native MCP Tools**: When available, always prefer `okf_*` MCP tools over CLI commands to minimize token/context overhead and avoid shell prompts.
6. **Prefer Update Over Duplication**: Expand existing concepts when related facts emerge.
7. **No Conversational Noise**: Never store scratchpads, raw chain-of-thought, or speculative chatter.
8. **Preserve Trust & Provenance**: Always record `sources` and `generated: { by, at }`. Never mark AI content as `human:` verified.
9. **End-of-Task Review**: Perform a knowledge review after completing substantial work.
10. **Always Validate**: Ensure `okf_validate(strict=true)` or `okf validate knowledge --strict --drift` passes with 0 errors and 0 warnings.

---

## 2. Tooling Reference (Dual-Mode: MCP & CLI)

Operations support both native MCP tools and deterministic CLI commands. **Always prefer MCP tools when available** because they execute in-process with zero terminal overhead, minimal context consumption, and structured JSON returns.

| Task | Preferred: Native MCP Tool | Fallback: Deterministic CLI (`--json`) |
| :--- | :--- | :--- |
| **Search Knowledge** | `okf_search(query="<query>", limit=3)` | `okf search "<query>" knowledge --limit 3 --json` |
| **Discover Code Constraints** | `okf_search(for_path="<file-path>")` | `okf search --for-path <file-path> knowledge --json` |
| **Inspect Concept** | `okf_show(concept_id="<id>")` | `okf show <id> knowledge --json` |
| **Create Concept** | `okf_create(concept_id="<id>", type="<type>", title="<title>", description="<desc>")` | `okf create <id> knowledge --type <type> --title "<title>" --desc "<desc>" --json` |
| **Update Concept** | `okf_update(concept_id="<id>", description="<desc>", title="<title>")` | `okf update <id> knowledge --desc "<desc>" --json` |
| **Relate Concepts** | `okf_relate(source_id="<src>", target_id="<tgt>", description="<prose>")` | `okf relate <src> <tgt> knowledge --desc "<prose>" --json` |
| **Validate Bundle** | `okf_validate(strict=true)` | `okf validate knowledge --strict --drift --json` |

> [!TIP]
> Both interfaces default to `./knowledge`. For custom or multi-bundle setups, pass the optional `bundle` argument (e.g. `okf_search(query="...", bundle="path/to/bundle")` or `okf search "..." path/to/bundle`).

---

## 3. Workflow Stages

1. **Discovery & Exploration**: Follow [discovery.md](./discovery.md) to locate relevant existing knowledge without blowing up context.
2. **Evaluation & Persistence**: Follow [remember.md](./remember.md) to decide what to persist vs. discard.
3. **Updating & Conflict Handling**: Follow [update.md](./update.md) when modifying existing concepts.
4. **Relationship Building**: Follow [relationships.md](./relationships.md) to interlink concepts cleanly.
5. **Worked Examples**: Inspect [examples.md](./examples.md) for software, coaching, and literature scenarios.
