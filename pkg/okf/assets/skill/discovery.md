# Discovery & Exploration Guide

Persistent memory is only useful if agents read existing knowledge before acting. This guide explains how to discover, search, and navigate an OKF v0.2 knowledge bundle efficiently without overflowing context windows.

---

## 1. Core Principle: Read Before Write

Before writing code, making architectural choices, or adding new memory concepts:
1. **Search existing memory**: Always query the knowledge base first.
2. **Reuse & Extend**: If a concept already covers the topic, update it instead of creating duplicates.
3. **Verify Context**: Inspect existing constraints, requirements, and past decisions.

```
Incoming Task
     │
     ▼
[ okf search "<query>" ] ──▶ Found matching concept? ──▶ YES ──▶ [ okf show <id> ] ──▶ Update existing
     │                                                                                       │
     ▼ NO                                                                                    │
[ okf create <id> ] ◀────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Progressive Disclosure Pattern

To prevent blowing up the LLM context window with large repositories, follow the **Progressive Disclosure** pattern:

```
Level 1: Bundle Root (knowledge/index.md + log.md)
   │     Quick orientation on bundle scope & recent changes
   ▼
Level 2: Fast In-Memory Search (okf search "<query>")
   │     Targeted BM25 keyword/relevance match across concepts
   ▼
Level 3: Concept Inspection (okf show <id>)
   │     Detailed inspection of frontmatter, body, and direct graph links
   ▼
Level 4: Follow Graph References
         Traverse related concepts only when deeper context is required
```

### Context Rules & Negative Constraints
- **NO Blanket Scans**: Never run `list_dir`, `grep_search`, or `view_file` over the entire `knowledge/` folder.
- **Search First, Load on Demand**: Always run `okf search "<query>" --limit 3 --json` to get lightweight metadata.
- **Selective Inspection**: Read the 1-sentence `description` in search hits first; only fetch the full body with `okf show <id>` if the concept is genuinely needed.
- **Task Relevance**: Do not proactively scan `knowledge/` for trivial code tasks unless relevant to architectural decisions, requirements, or explicitly requested.

---

## 3. Search & Inspection Commands

### Search the Corpus
Execute fast in-memory BM25 searches across concept titles, IDs, descriptions, and bodies:

```bash
# Human-readable search
okf search "authentication jwt" knowledge

# Agent JSON mode (structured output with scores & snippets)
okf search "authentication jwt" knowledge --json
```

**JSON Output Example**:
```json
{
  "query": "authentication jwt",
  "total_hits": 1,
  "results": [
    {
      "id": "architecture/auth",
      "path": "knowledge/architecture/auth.md",
      "title": "Authentication Architecture",
      "type": "Architecture",
      "description": "JWT-based stateless auth mechanism with refresh tokens.",
      "score": 4.82,
      "snippet": "All API endpoints authenticate via standard JWT Bearer tokens..."
    }
  ]
}
```

### Inspect Concept Details
Retrieve a single concept with its metadata, parsed frontmatter, and outward/inward links:

```bash
# Human-readable show
okf show architecture/auth knowledge

# Agent JSON mode
okf show architecture/auth knowledge --json
```

**JSON Output Example**:
```json
{
  "id": "architecture/auth",
  "path": "knowledge/architecture/auth.md",
  "type": "Architecture",
  "title": "Authentication Architecture",
  "description": "JWT-based stateless auth mechanism with refresh tokens.",
  "status": "active",
  "sources": ["docs/RFC-004.md"],
  "generated": {
    "by": "claude-code/v1.0",
    "at": "2026-09-01T10:00:00Z"
  },
  "content": "All API endpoints authenticate via standard JWT Bearer tokens...",
  "relationships": [
    {
      "target": "decisions/jwt-rotation",
      "description": "relies on key rotation policy"
    }
  ]
}
```

---

## 4. Discovery Checklist for Agents

When starting any new task, run through this quick checklist:
- [ ] Have I searched for keywords related to the feature or bug (`okf search "<keywords>"`?
- [ ] Is there an existing decision or constraint that restricts this implementation?
- [ ] Is any relevant concept marked with `status: deprecated` or has an expired `stale_after` date?
- [ ] If found, did I inspect the related concepts linked in its graph?
