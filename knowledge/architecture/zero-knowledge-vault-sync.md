---
type: Decision
title: Zero-Knowledge Vault Cryptography and Blind Sync Architecture
description: "Client-side zero-knowledge AES-256-GCM envelope encryption, Argon2id KDF, CAS blind storage, 3-way reconcile sync protocol, and Bearer auth."
tags: [vault, crypto, zero-knowledge, sync, cas, reconcile]
generated: { by: agent/gemini, at: "2026-09-17T13:07:28Z" }
governance: constraint
code_refs: [pkg/vault, pkg/sync, internal/cli/hub.go]
---

# Zero-Knowledge Vault Cryptography and Blind Sync Architecture

## Context & Purpose

While OKF Agent Memory operates locally in Git repositories, multi-device agents and human teams require decentralized synchronization across workstations and web environments without trusting cloud infrastructure or compromising confidential memory.

This architectural decision establishes the client-side cryptographic foundation and synchronization protocol for zero-knowledge synchronization between local bundles and the blind OKF Memory Hub.

---

## 1. Cryptographic Envelope (AES-256-GCM)

All concept files, tree manifests, and commit objects are encrypted exclusively on the client before leaving the trusted environment.

### Binary Envelope Specification
```
┌──────────────┬──────────────────┬─────────────────────────────┬──────────────────┐
│ Version (1B) │ Nonce / IV (12B) │ Ciphertext (Variable Länge) │ Auth Tag (16B)   │
│ 0x01         │ 96-bit CSPRNG    │ AES-256-GCM Payload         │ GCM Poly1305 Tag │
└──────────────┴──────────────────┴─────────────────────────────┴──────────────────┘
```

* **Version Authenticated**: The 1-byte header (`0x01`) is bound to the GCM `additionalData`, ensuring that envelope version tampering triggers authentication failure (`ErrAuthFailed`).
* **Content Addressing (CAS)**: The Content-Addressed Storage key for any blob is deterministically `sha256(RawBytes(Envelope))`.
* **Zero-Knowledge Property**: The remote storage server stores only raw binary ciphertext envelopes indexed by SHA-256. It has zero knowledge of concept titles, content, paths, authors, or timestamps.

---

## 2. Key Derivation (Argon2id)

Vault keys are deterministically derived from the user's master password and an unguessable 128-bit Secret Key:

* **Algorithm**: Argon2id
* **Parameters**: $m=64\text{ MB}$ (65536 KiB), $t=3$ iterations, $p=4$ parallelism lanes.
* **Output**: 256-bit (32-byte) AES key.
* **Emergency Kit**: Initializing a vault (`okf hub init-vault`) generates 16 bytes of CSPRNG entropy formatted as grouped Base32 (`XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XX`) and prints a durable physical backup document.

---

## 3. Atomic Head Advance & CAS Deduplication

1. **Change Detection via `plaintext_hash`**: The manifest tree records `plaintext_hash = sha256(plaintext)` for each file. When scanning a bundle, unchanged files are never re-encrypted, preserving CPU and memory budgets.
2. **CAS Deduplication**: Before uploading, clients query `POST /api/v1/vaults/{id}/blobs/check-missing` with candidate blob hashes and upload only missing envelopes via `PUT /blobs/{hash}`.
3. **Compare-and-Swap (CAS)**: Commit heads advance atomically via `POST /commit` specifying `expected_previous_head`. If another client has updated the head, the server rejects the request with `HTTP 409 Conflict`.

---

## 4. 3-Way Reconcile Engine & Collision Forking

When a `409 Conflict` occurs during synchronization:

1. **Disjoint Auto-Merge**: If concurrent clients modified distinct files, the engine performs a fast-forward 3-way merge (`baseTree`, `localTree`, `remoteTree`) without manual interaction.
2. **CLI Collision Failsafe**: If both clients modified the exact same file incompatibly:
   * The remote version is adopted at `<filename>.md`.
   * The local conflicting changes are preserved at `<filename>.conflict-local.md`.
   * **Guarantee**: Zero data loss under all concurrency conditions.

---

## 5. Hub Authentication & Configuration Governance

All remote hub communication supports Bearer token authentication to restrict vault creation and synchronization on private instances:

1. **Token Resolution Precedence**:
   * CLI Flag: `-auth-token <token>` (overrides all defaults)
   * Environment Variable: `OKF_HUB_TOKEN` (recommended for CI/CD and agent daemons)
   * Vault Config: `auth_token` in `.okf-vault.json` (convenience for interactive workstations)
2. **Remote URL Auto-Resolution**:
   * CLI Flag: `-hub <url>`
   * Vault Config: `hub_url` in `.okf-vault.json`
   * Fallback Default: `http://127.0.0.1:8080`
3. **Subcommand Set**: `okf hub init-vault`, `push`, `pull`, `sync`, and embedded `serve`.

---

## Related Concepts

- [5-Layer System Architecture](layers.md): Layered separation of concerns
- [Bundle Isolation and Mutation Security Boundaries](security-boundaries.md): Defensive containment and path traversal protection
