---
type: Decision
title: Safe Unknown Metadata Round-Trip
description: Unknown frontmatter keys are serialized deterministically with safe quoting and JSON-compatible scalar and collection preservation.
tags: [metadata, parser, serialization, security]
generated: { by: agent/codex, at: "2026-09-10T04:09:22Z" }
---

# Safe Unknown Metadata Round-Trip

Unknown fields preserve string content and JSON-compatible value kinds while canonicalizing top-level key order. Quoted delimiters do not split flow values. See [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md).
