# Updating & Conflict Handling Guide

Knowledge bases evolve. As project requirements shift, bugs are discovered, or assumptions change, existing concepts must be updated cleanly without losing historical context or creating duplicate files.

---

## 1. Core Principle: Prefer Update Over Duplication

Whenever new findings relate to an existing concept:
- **Do NOT** create `architecture/auth-v2.md` or `decisions/auth-new.md` unless the paradigm has completely split.
- **Do** update the existing concept (`architecture/auth.md`), document the revision in the body and in `knowledge/log.md`.

---

## 2. Handling Contradictions & Superseded Facts

When new discoveries invalidate an earlier assumption or decision:

1. **Do not silently overwrite the past**: If the previous context explains *why* the old way was chosen, preserve it in a "Historical Context" or "Superseded Decisions" section.
2. **Clarify the Current State**: Make the new active architecture or policy prominent.
3. **Update Metadata**:
   - Change `status: deprecated` if the concept itself is no longer active.
   - Update `stale_after` if a review is needed at a later date.
   - Record `updated` or `generated` timestamp.

### Example: Evolving an Architecture Concept
```markdown
---
okf_version: "0.2"
id: architecture/database
type: Architecture
title: Primary Database Architecture
status: active
generated:
  by: claude-code/v1.0
  at: 2026-09-02T10:00:00Z
---

# Primary Database Architecture

## Current State (2026-09)
The project uses PostgreSQL 16 managed on RDS with read replicas for analytical queries.

## Historical Context (Superseded)
Prior to 2026-09, SQLite was used during prototyping. It was migrated due to concurrency bottlenecks during multi-user testing.
```

---

## 3. Human Override Priority

Human instructions always take precedence over agent inferences:

1. If a human engineer explicitly corrects a stored fact or decision, update the concept immediately.
2. Record `sources: ["human: user-instruction"]` or note the human directive in provenance.
3. **Strict Rule**: An agent MUST NEVER repeatedly restore or re-infer an assumption that a human has explicitly rejected.

---

## 4. Updating Concepts with Tooling

Use `okf update` to mutate metadata, description, and automatically maintain log entries:

```bash
# Human-readable update
okf update architecture/auth knowledge \
  --desc "Updated to RS256 asymmetric keys with JWKS endpoint." \
  --title "Authentication Architecture (RS256)"

# Agent JSON mode
okf update architecture/auth knowledge \
  --desc "Updated to RS256 asymmetric keys with JWKS endpoint." \
  --json
```

**JSON Output Example**:
```json
{
  "status": "updated",
  "id": "architecture/auth",
  "path": "knowledge/architecture/auth.md",
  "concept": {
    "id": "architecture/auth",
    "type": "Architecture",
    "title": "Authentication Architecture (RS256)",
    "description": "Updated to RS256 asymmetric keys with JWKS endpoint.",
    "status": "active"
  }
}
```

---

## 5. Post-Update Checklist

After updating any concept:
- [ ] Has `knowledge/log.md` been updated with a dated entry? (Automatic with `okf update`).
- [ ] Have you checked for orphaned links or concepts that depended on the old behavior?
- [ ] Run `okf validate knowledge --strict --drift` to verify zero errors or broken links.
