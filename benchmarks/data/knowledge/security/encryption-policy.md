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
