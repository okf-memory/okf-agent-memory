---
type: Process
title: Knowledge Lifecycle & Review Workflow
description: Operational lifecycle stages and the Read-Before-Write loop for discovering, persisting, and updating project knowledge.
resource: https://github.com/okf-memory/okf-agent-memory
tags: [lifecycle, workflow, review, process]
generated: { by: agent/gemini-3.7-flash, at: 2026-08-27T11:24:00Z }
status: stable
sources:
  - id: convention
    resource: ../../docs/CONVENTION.md
    title: OKF Agent Memory Convention v0.1
    last_modified: 2026-08-27
---

# Knowledge Lifecycle & Review Workflow

Persistent knowledge moves through a well-defined lifecycle to prevent corpus rot and maintain coherence with the principles in [principles](principles.md).[^convention]

## The 7-Stage Lifecycle

```mermaid
flowchart TD
    D[1. Discover<br/>Identify persistent facts/decisions] --> E[2. Evaluate<br/>Check future value]
    E --> S[3. Search Existing Corpus<br/>Query knowledge bundle]
    S --> C[4. Create or Update<br/>Author concept or expand existing]
    C --> P[5. Add Provenance<br/>Fill sources, generated, and citations]
    P --> V[6. Validate<br/>Check syntax & graph connectivity]
    V --> R[7. Revisit & Lifecycle<br/>Verify, update, or deprecate]
```

## The Read-Before-Write Execution Sequence

For any substantial task, an agent follows this protocol:

1. **Identify the task**: Determine scope and goals.
2. **Discover relevant existing knowledge**: Read relevant concepts starting from index files.
3. **Inform execution**: Use loaded knowledge to avoid repeating past mistakes.
4. **Perform the task**: Complete code, research, or operational actions.
5. **Knowledge review**: Ask key reflective questions:
   - What did I learn or discover?
   - Did I make an important decision or change assumptions?
   - Did I create new artifacts or relationships?
6. **Create or update knowledge**: Write or update concepts and append dated entries in `log.md`.
7. **Validate the corpus**: Run conformance verification as described in [architecture/layers](../architecture/layers.md).

Overall roadmap progress is tracked in [roadmap/milestones](../roadmap/milestones.md).

[^convention]: OKF Agent Memory Convention v0.1
