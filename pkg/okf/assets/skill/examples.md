# OKF Agent Memory Worked Examples

This document demonstrates complete end-to-end memory workflows across three distinct domains: Software Engineering, Coaching Administration, and Literature / Reading History.

---

## Example 1: Software Engineering Workflow

### Goal
An agent is tasked with modifying database connection settings. Before editing code, it queries active governance for the database subsystem, investigates a connection pool exhaustion bug, persists an Architecture Decision Record (ADR) with `governance: constraint` and `code_refs`, and links it to the database architecture.

### Step 1: Pre-Edit Scope & Governance Check
Before touching code, discover governing constraints or active freezes:
- **Native MCP (Preferred)**: `okf_search(for_path="services/database/pool.go")`
- **CLI Fallback**:
  ```bash
  okf search --for-path services/database/pool.go knowledge --json
  ```

### Step 2: Search Existing Knowledge
- **Native MCP (Preferred)**: `okf_search(query="database connection", limit=3)`
- **CLI Fallback**:
  ```bash
  okf search "database connection" knowledge --limit 3 --json
  ```

### Step 3: Create the ADR Concept
- **Native MCP (Preferred)**:
  ```json
  // Tool: okf_create
  {
    "concept_id": "decisions/adr-008-connection-pooling",
    "type": "Decision",
    "title": "ADR-008: HikariCP Connection Pool Sizing",
    "description": "Configures HikariCP with max 20 connections and 30s timeout to prevent RDS pool exhaustion."
  }
  ```
- **CLI Fallback**:
  ```bash
  okf create decisions/adr-008-connection-pooling knowledge \
    --type "Decision" \
    --title "ADR-008: HikariCP Connection Pool Sizing" \
    --desc "Configures HikariCP with max 20 connections and 30s timeout to prevent RDS pool exhaustion." \
    --json
  ```

### Step 4: Link ADR to Core Database Concept
- **Native MCP (Preferred)**:
  ```json
  // Tool: okf_relate
  {
    "source_id": "decisions/adr-008-connection-pooling",
    "target_id": "architecture/database",
    "description": "configures connection pool parameters for primary database"
  }
  ```
- **CLI Fallback**:
  ```bash
  okf relate decisions/adr-008-connection-pooling architecture/database knowledge \
    --desc "configures connection pool parameters for primary database" \
    --json
  ```

### Resulting Concept File (`knowledge/decisions/adr-008-connection-pooling.md`):
```markdown
---
okf_version: "0.2"
id: decisions/adr-008-connection-pooling
type: Decision
title: "ADR-008: HikariCP Connection Pool Sizing"
description: "Configures HikariCP with max 20 connections and 30s timeout to prevent RDS pool exhaustion."
governance: constraint
code_refs:
  - "services/database/pool.go"
  - "pkg/db/**"
status: active
generated:
  by: claude-code/v1.0
  at: "2026-09-02T10:00:00Z"
sources:
  - "incident/2026-09-01-outage.md"
---

# ADR-008: HikariCP Connection Pool Sizing

## Context
During the 2026-09-01 traffic spike, backend instances opened >500 idle connections, causing RDS PostgreSQL to exceed max connection limits.

## Decision
1. Cap maximum pool size at 20 connections per pod.
2. Set connection timeout to 30,000ms.
3. Enable leak detection threshold at 60,000ms.

## Relationships
- [Primary Database Architecture](../architecture/database.md): configures connection pool parameters for primary database
```

---

## Example 2: Coaching & Consulting Workflow

### Goal
A coaching assistant maintains persistent memory of client profiles, session notes, and evolving milestones across multiple weeks.

### Step 1: Create Client Profile
```bash
okf create clients/jane-doe knowledge \
  --type "Client" \
  --title "Jane Doe — Leadership Coaching" \
  --desc "VP of Engineering focused on executive communication and delegation."
```

### Step 2: Log Session Note & Link to Client
```bash
okf create sessions/2026-09-02-jane-doe knowledge \
  --type "Session" \
  --title "Session 4: Delegation Frameworks" \
  --desc "Reviewed 70/20/10 delegation model and established weekly 1:1 agenda."

okf relate sessions/2026-09-02-jane-doe clients/jane-doe knowledge \
  --desc "coaching session record"
```

---

## Example 3: Literature & Reading History

### Goal
A personal knowledge agent records book takeaways, connects themes, and answers cross-concept queries.

### Step 1: Record Book Notes
```bash
okf create books/thinking-fast-and-slow knowledge \
  --type "Book" \
  --title "Thinking, Fast and Slow (Daniel Kahneman)" \
  --desc "Explores dual-system cognitive architecture: System 1 (fast, intuitive) and System 2 (slow, deliberate)."
```

### Step 2: Relate Book to Topic
```bash
okf create topics/cognitive-biases knowledge \
  --type "Topic" \
  --title "Cognitive Biases & Decision Science" \
  --desc "Systematic patterns of deviation from norm or rationality in judgment."

okf relate books/thinking-fast-and-slow topics/cognitive-biases knowledge \
  --desc "foundational text on dual-process theory and heuristics"
```

### Step 3: Discover Across the Knowledge Graph
When asked *"Which books discuss cognitive biases?"*, the agent runs:
- **Native MCP (Preferred)**: `okf_search(query="cognitive biases")`
- **CLI Fallback**:
  ```bash
  okf search "cognitive biases" knowledge --json
  ```
The result returns both the `topics/cognitive-biases` concept and `books/thinking-fast-and-slow` via its outward relationship, answering the question without needing conversation history.

---

## Example 4: Validating the Bundle

Always ensure strict bundle conformance at the end of every workflow:
- **Native MCP (Preferred)**:
  ```json
  // Tool: okf_validate
  {
    "strict": true
  }
  ```
- **CLI Fallback**:
  ```bash
  okf validate knowledge --strict --drift
  ```
Output:
```
OKF v0.2 check of "knowledge" (v0.2): 7 concept(s), 0 error(s), 0 warning(s); 0 broken link(s), 0 orphan(s), 0 stale [--strict]. Conformant.
```
