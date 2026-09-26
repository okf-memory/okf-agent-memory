---
type: Requirement
title: Metadata Mutation Parity for CLI and MCP
description: "CLI and MCP creation and update must expose type, status, and tags with explicit replacement semantics and valid lifecycle values."
tags: [cli, mcp, mutation, metadata]
generated: { by: agent/cli, at: "2026-09-26T08:51:42Z" }
status: stable
---

Both interfaces accept draft, stable, and deprecated statuses; creation defaults to stable. Update changes only fields explicitly supplied. CLI tags are comma-separated and trimmed; an explicitly empty string clears them. MCP tags are a JSON string array; an empty array clears them. Invalid status, empty updated type, and malformed or oversized MCP tags fail before persistence. See GitHub issue #38.

# Related Concepts
- [Go Single-Binary CLI & MCP Architecture Decision](../architecture/tooling-decision.md): Specifies lifecycle and tag mutation behavior for the Go CLI and MCP surfaces.
