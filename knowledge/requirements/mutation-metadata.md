---
type: Requirement
title: Metadata Mutation Parity for CLI and MCP
description: "CLI and MCP creation and update must expose type, status, and tags with explicit replacement semantics and valid lifecycle values."
tags: [cli, mcp, mutation, metadata]
generated: { by: agent/mcp, at: "2026-11-21T10:58:13Z" }
status: stable
---

Both CLI and MCP creation and update interfaces expose `type`, `status`, `tags`, `stale_after`, and optional `change_note` with explicit replacement semantics and valid lifecycle values:

- **Status**: Accepts `draft`, `stable`, and `deprecated`; creation defaults to `stable`.
- **Update Semantics**: Only explicitly supplied fields are modified; omitted fields are preserved.
- **Tags**: CLI tags are comma-separated and trimmed (an explicitly empty string clears them); MCP tags are a JSON string array (an empty array clears them).
- **Staleness (`stale_after`)**: Accepts ISO dates (`YYYY-MM-DD`), ISO 8601 timestamps (normalized to UTC date), or relative durations (`14d`, `2w`, `3m`), always persisting an absolute `YYYY-MM-DD` date. Passing `""` removes the key; dates in the past succeed with a warning. Invalid values fail before persistence without modifying the file.
- **Change Notes & Time-Range History**: `create`, `update`, and `relate` accept an optional `change_note` persisted in timestamped `log.md` entries (backward-compatible with legacy date-only entries and `MergeLogContent`). Both CLI (`okf changes --since`) and MCP (`okf_changes`) return identical structured change histories.
- **Validation**: Invalid status, empty updated type, invalid `stale_after`, and malformed or oversized MCP tags fail before persistence. See GitHub issue #38.

# Related Concepts
- [Go Single-Binary CLI & MCP Architecture Decision](../architecture/tooling-decision.md): Specifies lifecycle, staleness, change-log, and tag mutation behavior for the Go CLI and MCP surfaces.
