---
type: Convention
title: Core Memory Principles & Agent Contract
description: Foundational design principles and minimal behavioral guarantees for agents maintaining persistent project memory.
resource: https://github.com/okf-memory/okf-agent-memory
tags: [convention, principles, contract, agent-rules]
generated: { by: agent/cli, at: 2026-09-04T08:43:46Z }
status: stable
sources:
  - resource: ../../docs/CONVENTION.md
    id: convention
    title: OKF Agent Memory Convention v0.1
    last_modified: 2026-08-27
---

# Core Memory Principles & Agent Contract

The Agent Memory Convention enforces ten foundational design principles to ensure project memory remains accurate, compact, and maintainable over time.[^convention]

## The 10 Minimal Agent Guarantees

1. **Persistent knowledge belongs in the OKF corpus**: Conversations reset, but the `knowledge/` bundle survives.
2. **Search before write**: Search existing concepts before authoring to prevent duplication.
3. **Prefer update over duplication**: Expand or revise matching concepts when new facts emerge.
4. **No chain-of-thought storage**: Store only durable facts and decisions; discard transient reasoning.
5. **Preserve provenance**: Track sources, URIs, and dates using standard OKF mechanisms.
6. **Preserve uncertainty & qualify inference**: Distinguish directly observed facts from agent inferences.
7. **Preserve meaningful history**: Track changes via `log.md` and lifecycle metadata rather than silent overwrites.
8. **Review knowledge after substantial work**: Execute the review loop described in [lifecycle](lifecycle.md).
9. **Validate changes**: Ensure bundle conformance using the tooling layer outlined in [architecture/layers](../architecture/layers.md).
10. **Never claim persistence unless confirmed**: Report failures honestly if a disk write or validation fails.

## Domain Neutrality

The convention intentionally avoids prescribing a fixed taxonomy. Projects may define custom semantic types (e.g. `Fact`, `Decision`, `Client`, `Book`, `Runbook`, `Metric`) as needed. This flexibility supports diverse domains as detailed in [project/overview](../project/overview.md).

[^convention]: OKF Agent Memory Convention v0.1

# Related Concepts
- [Contributor Guidelines & PR Standards](contributing.md): PR contributors must adhere to these core memory principles
