---
okf_version: "0.2"
---

# Benchmark Service Knowledge Base

Persistent architecture decisions and operational guidelines for Benchmark Service.

## Concepts
* [auth/jwt-strategy](auth/jwt-strategy.md) — JWT Authentication & Refresh Token Strategy
* [database/naming-conventions](database/naming-conventions.md) — PostgreSQL Schema & Naming Rules
* [security/encryption-policy](security/encryption-policy.md) — AES-GCM-256 Customer Payload Encryption Standard
* [api/error-handling](api/error-handling.md) — RFC 7807 Problem Details Error Standards
* [caching/redis-tier](caching/redis-tier.md) — Redis Cache-Aside & TTL Invalidation Policy
* [deployment/health-checks](deployment/health-checks.md) — Kubernetes Liveness & Readiness Probes
* [telemetry/tracing](telemetry/tracing.md) — OpenTelemetry Distributed Tracing Standard
* [billing/stripe-webhooks](billing/stripe-webhooks.md) — Stripe Webhook Processing & Idempotency
