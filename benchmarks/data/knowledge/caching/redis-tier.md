---
type: Decision
title: Redis Distributed Cache-Aside and TTL Strategy
description: Cache-aside architecture utilizing Redis Cluster with structured key namespacing and jittered expiration.
tags: [caching, redis, performance, architecture]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:infra@company.dev, at: 2026-08-18T14:00:00Z }
status: stable
---

# Redis Distributed Cache-Aside and TTL Strategy

We employ a strict Cache-Aside (Lazy Loading) pattern backed by Redis Cluster.

## Key Namespacing Schema
All Redis keys must strictly follow the colon-delimited format:
`svc:<service_name>:<domain_entity>:<entity_id>[:<sub_key>]`

Example:
`svc:user-service:user_profile:018d3b82-94fa-7d9a-b472-358b87e2b101`

## TTL and Thundering Herd Protection
1. **Default TTL**: High-churn data: 5 minutes. Medium-churn data: 1 hour. Configuration data: 24 hours.
2. **Jitter Invalidation**: All TTLs must apply a **±10% random jitter** to prevent cache stampedes (thundering herd problem).
3. **Serialization**: Data must be serialized as compressed JSON or Protobuf. Never store raw Python pickle or PHP serialized objects.

# Related Concepts
- [Stripe Webhook Handling and Idempotency Architecture](../billing/stripe-webhooks.md): Redis used for webhook idempotency locking
