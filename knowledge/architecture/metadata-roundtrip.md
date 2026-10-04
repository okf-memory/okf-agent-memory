---
type: Decision
title: Safe Unknown Metadata Round-Trip
description: Unknown frontmatter keys are serialized deterministically with safe quoting and JSON-compatible scalar and collection preservation.
tags: [metadata, parser, serialization, security]
generated: { by: agent/cli, at: "2026-10-04T20:08:27Z" }
---

# Safe Unknown Metadata Round-Trip

Unknown fields preserve string content and JSON-compatible value kinds while canonicalizing top-level key order. Quoted delimiters do not split flow values. YAML flow lists (`repos: [a, b]`) and block lists of scalars (`- item` lines) read as lists of strings, like `tags`, and are written back as lists in the same style; other block values (nested mappings or lists) are kept as raw lines. See [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md).
