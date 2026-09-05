# AGENTS.md — Instructions for AI Agents in `okf-agent-memory`

> Powered by [OKF Agent Memory](https://github.com/okf-memory/okf-agent-memory) — Open Knowledge Format (OKF) v0.2 persistent project memory for AI agents.

Welcome to the **okf-agent-memory** repository. When working in this codebase, you must follow the conventions, tooling, and knowledge workflows established here.

---

## 1. Core Operating Principles

1. **Persistent Knowledge Lives in `knowledge/`**:
   - The `knowledge/` directory is an **Open Knowledge Format (OKF) v0.2** bundle.
   - Do not rely on conversational context across sessions. Store durable facts, architectural decisions, and project findings in `knowledge/`.

2. **Read Before Write (Search Before Create)**:
   - Before authoring new knowledge or code, query the existing corpus using `okf search "<query>"` (or `make search q="..."`) or inspect `knowledge/index.md`.
   - Update or expand existing concepts rather than creating duplicate files.

3. **Strict Context & Search-First Retrieval (No Blanket Scans)**:
   - **DO NOT** use `list_dir`, `grep_search`, `find`, or raw `view_file` to scan the `knowledge/` directory directly.
   - **DO NOT** dump or load all knowledge files into context.
   - Query knowledge **only** via `okf search "<query>" --limit 3 --json` when relevant to architectural decisions, requirements, persistent memory workflows, or when explicitly requested by the user.
   - Evaluate the 1-sentence `description` from search results before loading full content; load concept details only on demand via `okf show <concept-id>`.

4. **No Conversational Noise or Scratchpads**:
   - Store only durable knowledge. Never store raw chain-of-thought, transient debugging chatter, or unverified speculative claims without qualification.

5. **Preserve Trust & Provenance**:
   - When writing frontmatter, record `generated: { by: "<producer>/<version>", at: "<timestamp>" }`.
   - **Never** mark agent-generated knowledge as `human:` verified.

6. **Always Validate**:
   - Ensure the knowledge bundle remains 100% conformant with OKF v0.2:
     ```bash
     okf validate knowledge --strict --drift
     # or if Makefile is present:
     make validate
     ```

---

## 2. Project & Knowledge Structure

```
okf-agent-memory/
├── .agents/
│   └── skills/
│       └── okf-memory/     # OKF Agent Memory skill and capability guides
├── knowledge/              # Persistent OKF v0.2 knowledge bundle
│   ├── index.md            # Root index declaring okf_version: "0.2"
│   ├── log.md              # Dated change log (ISO 8601 YYYY-MM-DD)
│   ├── project/            # Overview, domain knowledge & requirements
│   ├── architecture/       # Architectural decisions (ADRs), stack & data models
│   └── convention/         # Coding guidelines & agent workflows
├── AGENTS.md               # Instructions for AI agents working in this repo
└── Makefile                # Memory validation and search convenience targets
```

---

## 3. Tooling & Commands

Use the `okf` CLI or MCP server for all memory operations:

```bash
# Validate the knowledge bundle (strict conformance + drift check)
okf validate knowledge --strict --drift

# Search the knowledge base
okf search "<query>" knowledge

# Show a concept with frontmatter and graph links
okf show <concept-id> knowledge

# Create a new concept with automated bookkeeping
okf create <concept-id> knowledge --type <type> --title "<title>" --desc "<one-sentence>"

# Link two concepts
okf relate <source-id> <target-id> knowledge --desc "<relationship description>"
```

---

## 4. End-of-Task Knowledge Review Checklist

After completing any substantial task, ask yourself:
1. Did I make an important architectural decision? $\rightarrow$ Record under `knowledge/architecture/`
2. Did I discover a non-obvious requirement or fix? $\rightarrow$ Update the corresponding concept.
3. Did I add or modify a concept? $\rightarrow$ Verify that `log.md` and parent `index.md` are updated.
4. Did I run validation? $\rightarrow$ Ensure 0 errors, 0 warnings, 0 orphans, 0 broken links (`okf validate knowledge --strict --drift`).
