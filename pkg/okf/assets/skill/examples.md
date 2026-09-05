# OKF Agent Memory Worked Examples

This document demonstrates complete end-to-end memory workflows across three distinct domains: Software Engineering, Coaching Administration, and Literature / Reading History.

---

## Example 1: Software Engineering Workflow

### Goal
An agent investigates a production bug involving connection pool exhaustion, discovers the root cause, persists an Architecture Decision Record (ADR), and links it to the database architecture.

### Step 1: Search Existing Knowledge
```bash
okf search "database connection" knowledge --json
```

### Step 2: Create the ADR Concept
```bash
okf create decisions/adr-008-connection-pooling knowledge \
  --type "Decision" \
  --title "ADR-008: HikariCP Connection Pool Sizing" \
  --desc "Configures HikariCP with max 20 connections and 30s timeout to prevent RDS pool exhaustion." \
  --json
```

### Step 3: Link ADR to Core Database Concept
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
```bash
okf search "cognitive biases" knowledge --json
```
The result returns both the `topics/cognitive-biases` concept and `books/thinking-fast-and-slow` via its outward relationship, answering the question without needing conversation history.

---

## Example 4: Validating the Bundle

Always ensure strict bundle conformance at the end of every workflow:
```bash
okf validate knowledge --strict --drift
```
Output:
```
OKF v0.2 check of "knowledge" (v0.2): 7 concept(s), 0 error(s), 0 warning(s); 0 broken link(s), 0 orphan(s), 0 stale [--strict]. Conformant.
```
