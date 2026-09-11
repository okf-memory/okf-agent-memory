---
type: Decision
title: CLI Optional Path Boundary
description: "CLI commands consume an optional bundle or target path only from the first remaining argument, preserving all subsequent flag values."
tags: [cli, arguments, flags, tooling]
generated: { by: agent/codex, at: "2026-09-10T04:09:20Z" }
---

# CLI Optional Path Boundary

Optional paths are positional-only and must precede flags. Flag values stay with the command FlagSet. See [Go Single-Binary CLI and MCP Architecture Decision](tooling-decision.md).
