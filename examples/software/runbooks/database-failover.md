---
type: Runbook
title: Database Read/Write Failover Procedure
description: Standard operational procedure for promoting a replica database when the primary cluster experiences an outage.
tags: [runbook, database, ops, failover]
generated: { by: agent/gemini-3.7-flash, at: 2026-08-27T12:00:00Z }
status: stable
---

# Database Failover Runbook

This procedure is executed when the primary database cluster becomes unreachable for services such as the [auth-service](../architecture/auth-service.md).

## Step-by-Step Procedure

1. Verify primary node health via monitoring dashboard.
2. Promote standby replica: `patronictl -c cluster.yml switchover`.
3. Update connection pool routing and verify authentication token generation in [jwt-tokens](../decisions/jwt-tokens.md).
