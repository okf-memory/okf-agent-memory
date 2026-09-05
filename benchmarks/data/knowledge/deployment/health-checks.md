---
type: Decision
title: Kubernetes Liveness and Readiness Probe Architecture
description: Distinct probe endpoints for Kubernetes container lifecycle management with dependency isolation.
tags: [deployment, kubernetes, k8s, devops]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:devops@company.dev, at: 2026-08-12T16:00:00Z }
status: stable
---

# Kubernetes Liveness and Readiness Probe Architecture

To ensure zero-downtime rolling deployments, all HTTP services must expose distinct health probe endpoints:

## Endpoints
1. **Liveness Probe**: `GET /healthz`
   - Checks only internal process health (goroutines, memory leaks, event loop responsiveness).
   - **MUST NOT** check external dependencies (database, Redis, external APIs).
   - Fails only if the process is deadlocked and needs a container restart.
2. **Readiness Probe**: `GET /readyz`
   - Verifies that the pod can accept live client traffic.
   - Checks connection pool ping to PostgreSQL and Redis.
   - Returns HTTP 503 if database connection is down, causing Kubernetes Service routing to temporarily bypass the pod.

# Related Concepts
- [OpenTelemetry Distributed Tracing and Context Propagation](../telemetry/tracing.md): Probes monitored via distributed tracing
