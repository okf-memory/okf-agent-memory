---
name: okf-agent-memory
description: Maintain persistent, domain-neutral project memory for AI agents using Open Knowledge Format (OKF) v0.3 bundles and the deterministic okf Go toolchain. Use whenever project knowledge, decisions, runbooks, research, client notes, or domain discoveries must survive conversational resets.
---

# OKF Agent Memory Skill (AAG Spec v0.1)

Teaches AI agents deterministic interaction with an Open Knowledge Format (OKF v0.3) bundle (`knowledge/`).

## 0. Project & Domain Codex
- GOAL: maintain(high_factual_integrity, domain_neutrality, strict_determinism)
- FORMAT: diagrams => ASSERT(syntax == mermaid, ELSE=STOP("Mermaid required; ASCII/box art prohibited."))

## 1. Behavioral Invariants (RFC 2119)
- MUST execute `okf_search(query=keywords, limit=3)` before proposing architecture changes or creating concepts.
- NEVER scan `knowledge/` via `list_dir`, `grep_search`, or raw file readers.
- NEVER forge human verification (`verified:` is human-only; declare `generated: { by: "<actor>", at: "<iso-time>" }`).
- PREFER native `okf_*` MCP tools OVER CLI fallback commands.
- PREFER `okf_update` OVER `okf_create` when mutating existing domain entities.
- NEVER persist scratchpads, raw chain-of-thought, or speculative chatter to knowledge corpus.

## 2. Guard Clauses & Scope Governance
- ON edit(@path/):
    IF first_visit(@path/) => okf_search(for_path=@path/)
    IF governance == "hold" => STOP("Subsystem frozen by governance. Confirm with user.")
    IF governance == "constraint" => MUST adhere to all listed invariants
    IF governance == "context" => proceed with awareness
- ON user_query(architecture | requirements | conventions | domain_facts):
    okf_search(query=keywords, limit=3) => evaluate summary description
    IF relevant => okf_show(concept_id) ONLY on demand

## 3. Completion Pipeline (Sequential Assertion Gates)
1. IF arch_decisions_made => MUST okf_create(architecture/*, type="decision", title=..., desc=...)
2. IF requirements_discovered => MUST okf_update(concept_id)
3. IF concepts_mutated => MUST sync(knowledge/log.md, knowledge/index.md)
4. ASSERT(okf_validate(strict=true, drift=true) == {errors: 0, warnings: 0}, ELSE=fix_before_exit)

---

## 4. Tooling Reference (Dual-Mode: MCP & CLI)

Default bundle: `./knowledge`. For custom paths, pass `bundle="path/to/bundle"`.

| Task | Preferred: Native MCP Tool | Fallback: Deterministic CLI (`--json`) |
| :--- | :--- | :--- |
| **Search Knowledge** | `okf_search(query="<query>", limit=3)` | `okf search "<query>" knowledge --limit 3 --json` |
| **Discover Code Constraints** | `okf_search(for_path="<file-path>")` | `okf search --for-path <file-path> knowledge --json` |
| **Inspect Concept** | `okf_show(concept_id="<id>")` | `okf show <id> knowledge --json` |
| **Query Changes** | `okf_changes(since="<date\|dur>", until="<date\|dur>", limit=50)` | `okf changes knowledge --since <date\|dur> --json` |
| **Create Concept** | `okf_create(concept_id="<id>", type="<type>", title="<title>", description="<desc>", stale_after="<date\|dur>", change_note="<note>")` | `okf create <id> knowledge --type <type> --title "<title>" --desc "<desc>" [--stale-after <val>] [--change-note "<note>"] --json` |
| **Update Concept** | `okf_update(concept_id="<id>", description="<desc>", stale_after="<date\|dur\|\"\">", change_note="<note>")` | `okf update <id> knowledge --desc "<desc>" [--stale-after <val>] [--change-note "<note>"] --json` |
| **Relate Concepts** | `okf_relate(source_id="<src>", target_id="<tgt>", description="<prose>", change_note="<note>")` | `okf relate <src> <tgt> knowledge --desc "<prose>" [--change-note "<note>"] --json` |
| **Validate Bundle** | `okf_validate(strict=true)` | `okf validate knowledge --strict --drift --json` |

---

## 5. Workflow Guides (Progressive Disclosure)

- Discovery & Retrieval: [discovery.md](./discovery.md)
- Persistence Decisions: [remember.md](./remember.md)
- Updating & Mutations: [update.md](./update.md)
- Relationship Topology: [relationships.md](./relationships.md)
- Multi-Domain Reference: [examples.md](./examples.md)
