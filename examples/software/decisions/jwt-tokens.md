---
type: Decision
title: Ed25519 Stateless JWT Tokens
description: Architectural decision adopting Ed25519-signed stateless JWTs for inter-service API authorization.
tags: [decision, jwt, security, auth]
generated: { by: agent/gemini-3.7-flash, at: 2026-08-27T12:00:00Z }
status: stable
---

# Decision: Ed25519 Stateless JWT Tokens

## Context
Downstream services required fast authorization verification without overwhelming the central database on every HTTP request.

## Decision
The [auth-service](../architecture/auth-service.md) issues Ed25519 asymmetric JWT tokens. Downstream services cache the public key and verify signatures locally without network calls.
