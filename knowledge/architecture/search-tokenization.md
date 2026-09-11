---
type: Decision
title: Unicode and Deterministic Search
description: Search tokenizes Unicode letters and digits and resolves equal scores by concept ID for reproducible results.
tags: [search, unicode, determinism, bm25]
generated: { by: agent/codex, at: "2026-09-10T04:09:21Z" }
---

# Unicode and Deterministic Search

Token boundaries use Unicode character classes. Equal rounded scores are ordered by concept ID. See [Go Single-Binary CLI and MCP Architecture Decision](tooling-decision.md).
