# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `qwen/qwen3-coder-30b`
* **Execution Mode**: `Local On-Device Inference`
* **Host Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Temperature**: `0.10`
* **Timeout**: `3m0s`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 21:41:33

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3058` tokens | `627` tokens | **-79.5%** |
| **Output Tokens (Generated)** | `320` tokens | `312` tokens | - |
| **Prefill Latency (TTFT)** | `10607.5 ms` | `2458.1 ms` | **4.3x faster** |
| **Total Turn Time** | `17.75 s` | `8.40 s` | - |
| **Policy Compliance** | `4/4 checks` | `4/4 checks` | 100% Consistent |

### Policy Checks:
* **AES-256-GCM**: ✅ PASS
* **96-bit / 12-byte Nonce**: ✅ PASS
* **X-OKF-Encryption-Version Header**: ✅ PASS
* **No ECB/CBC**: ✅ PASS

---

## 📝 Generated Code Responses

### Run 1: Monolith Context Dump

```go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "fmt"
    "io"
)

// EncryptPayload encrypts sensitive customer data using AES-256-GCM
// Returns the encrypted payload with prepended 12-byte nonce
// The caller must ensure the encryption key is sourced from ENCRYPTION_MASTER_KEY_V2
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("failed to create AES cipher: %w", err)
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM: %w", err)
    }
    
    // Generate a 12-byte nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    // Seal appends the ciphertext and GCM tag to the nonce
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    
    return ciphertext, nil
}

// GetEncryptionHeaders returns the required metadata header for encrypted payloads
func GetEncryptionHeaders() map[string]string {
    return map[string]string{
        "X-OKF-Encryption-Version": "v2",
    }
}
```

### Run 2: OKF Progressive Disclosure

```go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "fmt"
    "io"
)

// EncryptPayload encrypts sensitive customer data using AES-256-GCM
// with a 96-bit random nonce and returns the ciphertext with the nonce prepended.
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("failed to create AES cipher: %w", err)
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
    }
    
    // Generate a 12-byte nonce
    nonce := make([]byte, gcm.NonceSize()) // 12 bytes
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    // Seal appends the ciphertext and GCM tag to the nonce
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    
    return ciphertext, nil
}

// GetEncryptionHeaders returns the required metadata headers for encrypted payloads.
func GetEncryptionHeaders() map[string]string {
    return map[string]string{
        "X-OKF-Encryption-Version": "v2",
    }
}
```
