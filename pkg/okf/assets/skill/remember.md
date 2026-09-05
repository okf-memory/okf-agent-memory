# Evaluation & Persistence Guide (What to Remember)

Persistent memory must be high-signal and durable. This guide provides actionable criteria for deciding what information belongs in the OKF knowledge base versus what should be discarded as ephemeral conversation.

---

## 1. The Core Test: Future Value

Before creating or persisting a concept, ask:

> **"Would a future agent or human engineer starting with a blank context window reasonably benefit from knowing this?"**

If the answer is **No**, do not write it to persistent memory.

---

## 2. What to Remember vs. What to Discard

| Category | ✅ PERSIST in `knowledge/` | ❌ DISCARD (Do NOT Persist) |
| :--- | :--- | :--- |
| **Decisions** | Architectural choices, selected libraries, rejected alternatives with trade-offs. | Momentary thoughts, trivial variable naming deliberations. |
| **Requirements** | Business rules, security boundaries, performance SLAs, platform targets. | Transient prompt instructions ("make it shorter", "fix typo"). |
| **Discoveries** | Non-obvious quirks, library workarounds, undocumented API behaviors. | Standard syntax explanations, things already in official docs. |
| **Domain Facts** | Client profiles, coaching milestones, book reading notes, entity models. | General public trivia easily retrieved from the web. |
| **Bugs & Fixes** | Root causes of complex regressions, subtle race conditions. | Simple syntax errors fixed in 1 step during normal coding. |
| **User Directives** | Explicit project owner guidelines, persistent style constraints. | Social pleasantries ("Hello", "Thank you", "Good job"). |
| **Conversations** | Never. (Corpus is not a transcript). | Raw chain-of-thought, scratchpads, debugging logs. |

---

## 3. Domain Neutrality & Concept Types

OKF v0.2 is completely domain-neutral. You can define and use semantic types that fit the project domain:

- **Software Engineering**: `Decision`, `Architecture`, `Fact`, `Requirement`, `Component`, `Bug`
- **Coaching & Consulting**: `Client`, `Session`, `Goal`, `Observation`, `Process`, `ActionItem`
- **Literature & Research**: `Book`, `Author`, `Review`, `Topic`, `Paper`, `Citation`
- **General Projects**: `Fact`, `Policy`, `Resource`, `Event`, `Task`

Agents must not reject a concept merely because its type is novel or project-specific.

---

## 4. Trust, Provenance, and Uncertainty

### Provenance Rules
1. **Declare AI Generation**: When an agent authors a concept, always include:
   ```yaml
   generated:
     by: "<agent-name>/<version>" # e.g. "claude-code/v1.0" or "antigravity/v2.0"
     at: "2026-09-02T10:00:00Z"
   ```
2. **Never Forge Human Verification**: The `verified:` frontmatter is reserved strictly for explicit human review. Agents must NEVER mark their own outputs as human-verified.
3. **Record Sources**: When knowledge comes from an external document, commit, or URL, list it in `sources: [...]`.

### Preserving Uncertainty (Inferences vs. Facts)
Do not convert an educated guess into an established fact without qualification:
- ❌ *Incorrect*: "The database crashes under 1,000 req/s because of connection pool exhaustion." (untested assumption)
- ✅ *Correct*: "Based on load test logs and high connection wait times, the agent infers that pool exhaustion caused the crash. Needs human/benchmark confirmation."

---

## 5. Creating Concepts with Tooling

Use `okf create` to scaffold conformant concepts with automatic frontmatter, timestamps, and log bookkeeping:

```bash
# Human-readable create
okf create architecture/caching knowledge \
  --type "Architecture" \
  --title "Distributed Caching Strategy" \
  --desc "Redis cluster configuration for session storage and query caching."

# Agent JSON mode
okf create architecture/caching knowledge \
  --type "Architecture" \
  --title "Distributed Caching Strategy" \
  --desc "Redis cluster configuration for session storage and query caching." \
  --json
```

**JSON Output Example**:
```json
{
  "status": "created",
  "id": "architecture/caching",
  "path": "knowledge/architecture/caching.md",
  "concept": {
    "id": "architecture/caching",
    "type": "Architecture",
    "title": "Distributed Caching Strategy",
    "description": "Redis cluster configuration for session storage and query caching.",
    "status": "active"
  }
}
```

---

## 6. End-of-Task Knowledge Checklist

After completing any significant coding or research session, ask:
1. Did I make or encounter an architectural choice? $\rightarrow$ Record in `knowledge/architecture/` or `knowledge/decisions/`.
2. Did I discover a non-obvious solution or constraint? $\rightarrow$ Record under relevant concept.
3. Did I write raw scratchpads or chat noise? $\rightarrow$ **Clean up and delete them.**
4. Did I run `okf validate knowledge --strict`? $\rightarrow$ Ensure 0 errors.
