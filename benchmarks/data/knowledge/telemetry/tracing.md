---
type: Decision
title: OpenTelemetry Distributed Tracing and Context Propagation
description: W3C Trace Context propagation standard with mandated semantic attributes across all RPC and HTTP boundaries.
tags: [telemetry, tracing, opentelemetry, observability]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:sre@company.dev, at: 2026-08-10T15:00:00Z }
status: stable
---

# OpenTelemetry Distributed Tracing Standard

Distributed tracing allows end-to-end request visibility across our microservices architecture.

## Requirements
1. **Context Propagation Protocol**: Mandates **W3C Trace Context** (`traceparent` and `tracestate` HTTP headers).
2. **Span Naming**: Must follow OpenTelemetry Semantic Conventions:
   - HTTP Server: `{http.method} {http.route}` (e.g. `POST /api/v1/checkout`)
   - Database: `{db.operation} {db.name}.{db.table}` (e.g. `SELECT benchmark_db.users`)
3. **Sampling Strategy**: 100% trace sampling in staging; adaptive 5% head-based sampling in production with 100% error capture.

# Related Concepts
- [RFC 7807 Problem Details HTTP Error Standard](../api/error-handling.md): Tracing spans attach to API error responses
