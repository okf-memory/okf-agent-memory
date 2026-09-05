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
