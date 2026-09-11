---
type: Decision
title: Relationship Identity and Logging
description: "Relationships are identified by target path and description, making retries idempotent while retaining distinct relationship contexts."
tags: [relationships, idempotency, logging, mutation]
generated: { by: agent/codex, at: "2026-09-10T04:09:21Z" }
---

# Relationship Identity and Logging

An exact target-path and description pair is idempotent regardless of display-title changes. A successful new relationship emits one specific log entry. See [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md).
