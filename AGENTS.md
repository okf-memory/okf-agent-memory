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
   - Query knowledge **only** via `okf search "<query>" --limit 3 --json` (or `okf_search`) when relevant to architectural decisions, requirements, persistent memory workflows, or when explicitly requested by the user.
   - Evaluate the 1-sentence `description` from search results before loading full content; load concept details only on demand via `okf show <concept-id>`.

4. **Pre-Edit Scope & Governance Check**:
   - Before starting substantial work on a subsystem, module, or critical file, query governing concepts once for that target scope:
     `okf search --for-path <path>` (e.g. `okf search --for-path pkg/auth/` or `okf_search(for_path="pkg/auth/")`).
   - Do **NOT** invoke this redundantly for every single file if the governing module/directory has already been evaluated for the task.
   - Respect the returned governance level:
     - `hold`: Active freeze/migration. Do NOT modify the file without explicit user confirmation.
     - `constraint`: Mandatory guardrail. Must adhere to the guidelines when modifying code.
     - `context`: Advisory domain background.

5. **No Conversational Noise or Scratchpads**:
   - Store only durable knowledge. Never store raw chain-of-thought, transient debugging chatter, or unverified speculative claims without qualification.

6. **Preserve Trust & Provenance**:
   - When writing frontmatter, record `generated: { by: "<producer>/<version>", at: "<timestamp>" }`.
   - **Never** mark agent-generated knowledge as `human:` verified.

7. **Always Validate**:
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
│       └── okf-memory/     # Active OKF Agent Memory skill (Single Source of Truth)
├── cmd/
│   ├── okf/                # Standalone CLI and embedded MCP server (`stdio`)
│   └── okf-benchmark/      # LLM benchmark and progressive disclosure test runner
├── pkg/okf/                # Zero-dependency Go core library (parser, validator, BM25, MCP)
│   └── assets/             # Embedded bootstrap templates & skills mirrored via `make sync-assets`
├── knowledge/              # Persistent OKF v0.2 knowledge bundle
│   ├── index.md            # Root index declaring okf_version: "0.2"
│   ├── log.md              # Dated change log (ISO 8601 YYYY-MM-DD)
│   ├── project/            # Overview, domain knowledge & requirements
│   ├── architecture/       # Architectural decisions (ADRs), stack & data models
│   ├── convention/         # Coding guidelines & agent workflows
│   └── roadmap/            # Phased development roadmap & milestones
├── docs/                   # Guides, specifications, CLI reference & release playbook
├── AGENTS.md               # Instructions for AI agents working in this repo
├── CONTRIBUTING.md         # Contribution guidelines & development workflow
└── Makefile                # Memory validation, test, sync-assets & release targets
```

---

## 3. Tooling & Commands (Dual-Mode: MCP & CLI)

Memory operations support both native MCP tools and the deterministic `okf` CLI:

### Option A: Native MCP Tools (Recommended when MCP is active)
If the `okf_*` tools (`okf_search`, `okf_show`, `okf_validate`, `okf_create`, `okf_update`, `okf_relate`) are available in your toolset, **always prefer them** over shell commands. They execute in-process with zero terminal overhead or sandbox prompts.
- Tools default to the project's `./knowledge` bundle.
- Query by text: `okf_search(query="...", limit=3)`.
- Query by code path: `okf_search(for_path="pkg/okf/types.go")` to discover active constraints and holds before editing code.
- For multi-bundle repositories or custom paths, pass the optional `bundle` argument (e.g. `okf_search(query="...", bundle="examples/software")`).

### Option B: `okf` CLI Fallback (Standalone / CI/CD)
When running in environments without the MCP server, use the CLI:

```bash
# Validate the knowledge bundle (strict conformance + drift check)
okf validate knowledge --strict --drift

# Discover constraints and holds governing a file before editing
okf search --for-path <file-path> knowledge

# Search the knowledge base by text
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

Before concluding any substantial task, you MUST systematically verify and complete each item:
- [ ] **Architectural Decision Made?** $\rightarrow$ Record under `knowledge/architecture/` with cross-links.
- [ ] **Non-Obvious Requirement or Fix Discovered?** $\rightarrow$ Update the corresponding concept.
- [ ] **Concept Added or Modified?** $\rightarrow$ Verify that `log.md` and parent `index.md` are updated (automated via `okf create` / `okf relate`).
- [ ] **Skills or Conventions Modified?** $\rightarrow$ Run `make sync-assets` to mirror active skills into `pkg/okf/assets/skill/` (enforced automatically via `TestDogfoodingAssetDrift` in `make check`).
- [ ] **Knowledge Conformance Validated?** $\rightarrow$ Ensure 0 errors, 0 warnings, 0 orphans, 0 broken links (`okf validate knowledge --strict --drift` or `make validate`).
