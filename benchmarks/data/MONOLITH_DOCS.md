# PROJECT ENGINEERING & ARCHITECTURE STANDARDS (MONOLITH DUMP)

This document contains all engineering standards, ADRs, database rules, security guidelines, and deployment specs for Benchmark Service.

---


## FILE: api/error-handling.md

---
type: Decision
title: RFC 7807 Problem Details HTTP Error Standard
description: Standardized API error response format based on RFC 7807 problem details with typed error domains.
tags: [api, http, rest, errors, adr]
verified: { by: human:api-lead@company.dev, at: 2026-08-25T11:00:00Z }
status: stable
---

# RFC 7807 Problem Details HTTP Error Standard

All REST API endpoints must return structured errors complying with RFC 7807 (`application/problem+json`).

## JSON Payload Structure
```json
{
  "type": "https://api.company.dev/errors/auth-invalid-credentials",
  "title": "Invalid Credentials",
  "status": 401,
  "detail": "The email or password provided does not match our records.",
  "instance": "/api/v1/auth/login",
  "code": "ERR_AUTH_INVALID_CREDENTIALS",
  "timestamp": "2026-09-05T10:00:00Z"
}
```

## Mandatory Error Code Naming
Error codes must follow the exact regular expression: `^ERR_[A-Z]+_[A-Z0-9_]+$`
Examples:
- `ERR_PAYMENT_INSUFFICIENT_FUNDS`
- `ERR_VALIDATION_FIELD_REQUIRED`
- `ERR_RATE_LIMIT_EXCEEDED`


---


## FILE: auth/jwt-strategy.md

---
type: Decision
title: JWT Authentication & Refresh Token Architecture
description: Standardized RSA-256 asymmetric JWT authentication with 15-minute access expiration and sliding refresh tokens.
tags: [auth, security, tokens, adr]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:sec-lead@company.dev, at: 2026-08-20T10:00:00Z }
status: stable
---

# JWT Authentication & Refresh Token Architecture

We standardize on asymmetric RSA-256 (RS256) JWTs for API authentication across all microservices.

## Architecture Specifications
1. **Signing Algorithm**: RS256 with 4096-bit private keys. Public keys are exposed via standard JWKS at `/.well-known/jwks.json`.
2. **Access Token Lifetime**: Exactly **15 minutes**. Never grant access tokens longer than 1 hour.
3. **Refresh Tokens**: Stored in PostgreSQL with SHA-256 hash. Valid for 14 days with sliding expiration on each use.
4. **Transport**: Access tokens in `Authorization: Bearer <token>` header. Refresh tokens exclusively in `httpOnly`, `Secure`, `SameSite=Strict` cookies.
5. **Mandatory Claims**:
   - `sub`: User UUIDv7
   - `iss`: `https://auth.company.dev`
   - `aud`: `https://api.company.dev`
   - `exp`: UNIX timestamp (15m from issuance)
   - `roles`: Array of assigned RBAC role strings.

# Related Concepts
- [Customer Payload Data Encryption Standard](../security/encryption-policy.md): JWT tokens use payload encryption


---


## FILE: billing/stripe-webhooks.md

---
type: Decision
title: Stripe Webhook Handling and Idempotency Architecture
description: Fault-tolerant Stripe event ingestion utilizing Redis distributed locking and cryptographic webhook signature verification.
tags: [billing, payments, stripe, webhooks]
verified: { by: human:fintech-lead@company.dev, at: 2026-08-05T13:00:00Z }
status: stable
---

# Stripe Webhook Handling and Idempotency Architecture

All Stripe webhook event receivers must adhere to this zero-loss processing contract.

## Contract Specifications
1. **Signature Verification**: Must verify `Stripe-Signature` header against endpoint secret using the raw unparsed request body (`webhook.ConstructEvent`).
2. **Idempotency Locking**:
   - Every event has a unique `event.ID`.
   - Before processing, acquire a Redis distributed lock: `SET lock:stripe:<event.id> 1 NX EX 600`.
   - If key exists, return HTTP 200 immediately to prevent duplicate credit card charging.
3. **Async Queue Dispatch**: After signature validation and idempotency check, push event to Kafka topic `billing.events` and return HTTP 200 within 500ms. Heavy processing must never happen synchronously in the HTTP handler.


---


## FILE: caching/redis-tier.md

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


---


## FILE: database/naming-conventions.md

---
type: Decision
title: PostgreSQL Database Schema and Naming Conventions
description: Strict relational schema conventions requiring snake_case naming, pluralized table names, and UUIDv7 primary keys.
tags: [database, postgres, sql, conventions]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:db-architect@company.dev, at: 2026-08-15T09:00:00Z }
status: stable
---

# PostgreSQL Database Schema and Naming Conventions

All PostgreSQL database tables, columns, and indexes must strictly follow these rules:

## Naming Rules
1. **Table Names**: Lowercase, plural nouns in `snake_case` (e.g. `users`, `audit_logs`, `payment_invoices`).
2. **Primary Keys**: Always named `id` with type `uuid` generated via time-ordered **UUIDv7**. Never use serial integers.
3. **Foreign Keys**: Column name must be singular target table name plus `_id` (e.g. `organization_id` referencing `organizations(id)`).
4. **Timestamps**: All tables must include:
   - `created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()`
   - `updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()`
5. **Soft Deletes**: If soft deletion is required, use `deleted_at TIMESTAMPTZ NULL`. Never use boolean `is_deleted`.
6. **Index Naming**:
   - Primary key: `pk_<table_name>`
   - Foreign key index: `idx_<table_name>_<column_name>`
   - Unique index: `uq_<table_name>_<column_name>`

# Related Concepts
- [Redis Distributed Cache-Aside and TTL Strategy](../caching/redis-tier.md): Cache keys mirror database entity names


---


## FILE: deployment/health-checks.md

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


---


## FILE: security/encryption-policy.md

---
type: Decision
title: Customer Payload Data Encryption Standard
description: Standardized AES-GCM-256 encryption with 96-bit random nonces for all sensitive customer data at rest.
tags: [security, encryption, crypto, adr]
generated: { by: agent/cli, at: 2026-09-05T09:33:41Z }
verified: { by: human:ciso@company.dev, at: 2026-09-01T12:00:00Z }
status: stable
---

# Customer Payload Data Encryption Standard

This document establishes the mandatory cryptography standard for storing sensitive customer personal data (PII) and credentials.

## Mandatory Cryptographic Rules
1. **Algorithm**: Must use authenticated encryption: **AES-256-GCM** (`crypto/cipher.NewGCM`).
2. **Nonce / IV**: Exactly **96-bit (12 bytes)** cryptographically secure random nonce generated via `crypto/rand.Read`.
3. **Storage Format**: Nonce must be prepended to the ciphertext: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`.
4. **Header Convention**: All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header: `X-OKF-Encryption-Version: v2`.
5. **Forbidden Algorithms**: **NEVER** use AES-ECB, AES-CBC without HMAC, or raw RSA for payload encryption.
6. **Key Derivation**: Master keys must be 32 bytes (256 bits) sourced from environment variable `ENCRYPTION_MASTER_KEY_V2`.

## Go Implementation Example
```go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "fmt"
    "io"
)

func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, gcm.NonceSize()) // 12 bytes
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return ciphertext, nil
}
```

# Related Concepts
- [RFC 7807 Problem Details HTTP Error Standard](../api/error-handling.md): Encryption errors mapped to RFC 7807


---


## FILE: telemetry/tracing.md

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


---

