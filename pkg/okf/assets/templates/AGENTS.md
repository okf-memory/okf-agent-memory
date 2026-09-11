# AGENTS.md — Instructions for AI Agents in `{{PROJECT_NAME}}`

> Powered by [OKF Agent Memory](https://github.com/okf-memory/okf-agent-memory) — Open Knowledge Format (OKF) v0.2 persistent project memory for AI agents.

Welcome to the **{{PROJECT_NAME}}** repository. When working in this codebase, you must follow the memory conventions:

<!-- BEGIN OKF AGENT MEMORY -->
---

## 🧠 Persistent Project Memory (OKF v0.2)

1. **Persistent Knowledge Lives in `knowledge/`**:
   - The `knowledge/` directory is an **Open Knowledge Format (OKF) v0.2** bundle.
   - Store durable facts, architectural decisions, and project findings in `knowledge/`. Never store transient conversational noise.

2. **Read Before Write (Search Before Create)**:
   - Before authoring new knowledge or code, query existing memory: `okf search "<query>"` or inspect `knowledge/index.md`.
   - Update existing concepts instead of creating duplicates.

3. **Strict Context & Search-First Retrieval (No Blanket Scans)**:
   - **DO NOT** use `list_dir`, `grep`, or scan `knowledge/` in bulk.
   - Query knowledge via `okf search "<query>" --limit 3 --json` (or `okf_search`) only when relevant or requested.
   - Inspect concept descriptions first and load full concepts only on demand using `okf show <id>`.

4. **Pre-Edit Scope & Governance Check**:
   - Before modifying code in a subsystem or module, query governing concepts:
     `okf search --for-path <path>` (e.g. `okf search --for-path pkg/auth/` or `okf_search(for_path="...")`).
   - Avoid redundant per-file calls if the governing scope has already been evaluated.
   - Respect governance levels: `hold` (active freeze, human confirmation required), `constraint` (mandatory rules), and `context` (advisory background).

5. **Preserve Trust & Provenance**:
   - Agent writes declare `generated: { by: "<agent>", at: "<timestamp>" }`. Never forge human verification (`verified:`).

6. **Tooling & Retrieval (Dual-Mode: MCP & CLI)**:
   - **MCP First**: If `okf_*` tools (`okf_search`, `okf_show`, `okf_create`, etc.) are available, prefer them over CLI commands. By default, they target `./knowledge` (or pass `bundle: "<path>"` for other bundles).
     - Query by text: `okf_search(query="...", limit=3)`
     - Query by code path: `okf_search(for_path="path/to/module/")`
   - **CLI Fallback**:
     - `okf search --for-path <path> [bundle]` — Discover constraints and holds governing code paths
     - `okf search "<query>" [bundle]` — Query memory using in-memory BM25
     - `okf show <id> [bundle]` — Inspect concept details and relationship graph
     - `okf create <id> [bundle] --type <type> --title "<title>" --desc "<desc>"` — Document new fact
     - `okf update <id> [bundle] --desc "<updated-desc>"` — Modify existing concept
     - `okf relate <src> <tgt> [bundle] --desc "<rel>"` — Link concepts together
     - `okf validate [bundle] --strict --drift` — Verify 100% OKF v0.2 conformance

7. **End-of-Task Review Checklist**:
   - Did I make an architectural decision? -> Record under `knowledge/architecture/`
   - Did I add/update concepts? -> Ensure `knowledge/log.md` and parent `index.md` are updated.
   - Did I validate? -> Ensure 0 errors, 0 broken links (`okf validate knowledge --strict --drift`).
<!-- END OKF AGENT MEMORY -->

