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
