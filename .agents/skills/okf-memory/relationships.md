# Relationship Building & Graph Linking Guide

Concepts in an OKF v0.2 bundle do not exist in isolation. Linking related concepts creates a high-signal knowledge graph that allows future agents to discover relevant dependencies, constraints, and historical context.

---

## 1. Core Principle: Semantic Linking Without Fragmentation

Relationships should represent meaningful semantic connections between distinct concepts with independent lifecycles.

### Avoid Artificial Fragmentation
- **Anti-pattern**: Creating 5 separate 1-line concepts (`decision.md`, `rationale.md`, `alternatives.md`, `consequences.md`, `author.md`) and linking them all together.
- **Best Practice**: Keep cohesive units together in one concept file. Only split into separate concepts when entities have independent lifecycles or are referenced across multiple domains.

---

## 2. Common Domain-Neutral Relationship Patterns

| Domain | Source Concept | Target Concept | Relationship Description |
| :--- | :--- | :--- | :--- |
| **Software** | `decisions/adr-001` | `architecture/database` | *"implements PostgreSQL with connection pooling"* |
| **Software** | `bugs/conn-leak` | `architecture/database` | *"affects connection pool eviction logic"* |
| **Software** | `research/grpc-benchmarks` | `decisions/adr-002` | *"justifies gRPC over REST for microservices"* |
| **Coaching** | `sessions/2026-09-02` | `clients/jane-doe` | *"coaching session with Jane Doe"* |
| **Coaching** | `goals/public-speaking` | `clients/jane-doe` | *"target milestone for Jane Doe"* |
| **Literature** | `reviews/thinking-fast` | `books/thinking-fast-and-slow` | *"critical review and chapter notes"* |
| **Literature** | `books/thinking-fast-and-slow`| `topics/cognitive-biases` | *"explores heuristic decision-making"* |

---

## 3. Creating Relationships with Tooling

Use `okf relate` to link two concepts deterministically with automatic frontmatter and markdown updates:

```bash
# Human-readable relate
okf relate decisions/adr-001 architecture/database knowledge \
  --desc "implements connection pooling strategy"

# Agent JSON mode
okf relate decisions/adr-001 architecture/database knowledge \
  --desc "implements connection pooling strategy" \
  --json
```

**JSON Output Example**:
```json
{
  "status": "related",
  "source": "decisions/adr-001",
  "target": "architecture/database",
  "description": "implements connection pooling strategy",
  "updated_files": [
    "knowledge/decisions/adr-001.md"
  ]
}
```

---

## 4. Graph Integrity & Validation

Whenever links are added or removed:

1. **Check for Broken Links**: Every link target must resolve to a valid concept path or ID within the bundle.
2. **Check for Orphaned Concepts**: Ensure key concepts are reachable from index files or related concepts.
3. **Run Strict Validation**:
   ```bash
   okf validate knowledge --strict
   ```
   Strict validation ensures:
   - 0 broken links / dangling pointers
   - 0 orphaned concepts
   - 0 schema or frontmatter errors
