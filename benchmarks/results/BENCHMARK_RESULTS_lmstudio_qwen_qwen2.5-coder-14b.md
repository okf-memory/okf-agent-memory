# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `qwen/qwen2.5-coder-14b`
* **Execution Mode**: `Local On-Device Inference`
* **Host Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Temperature**: `0.10`
* **Timeout**: `3m0s`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 21:05:20

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3058` tokens | `627` tokens | **-79.5%** |
| **Output Tokens (Generated)** | `379` tokens | `411` tokens | - |
| **Prefill Latency (TTFT)** | `17994.5 ms` | `3997.3 ms` | **4.5x faster** |
| **Total Turn Time** | `38.05 s` | `24.17 s` | - |
| **Policy Compliance** | `4/4 checks` | `4/4 checks` | 100% Consistent |

### Policy Checks:
* **X-OKF-Encryption-Version Header**: ✅ PASS
* **No ECB/CBC**: ✅ PASS
* **AES-256-GCM**: ✅ PASS
* **96-bit / 12-byte Nonce**: ✅ PASS

---

## 📝 Generated Code Responses

### Run 1: Monolith Context Dump

```go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
)

// EncryptPayload encrypts the given plaintext using AES-256-GCM with a 96-bit nonce.
// It returns the base64-encoded ciphertext prefixed with the nonce and a metadata header.
func EncryptPayload(plaintext []byte, key []byte) (string, string, error) {
    // Ensure the key is 32 bytes for AES-256
    if len(key) != 32 {
        return "", "", fmt.Errorf("encryption key must be 32 bytes long")
    }

    // Create a new AES cipher block
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", "", fmt.Errorf("failed to create AES cipher: %w", err)
    }

    // Create a GCM mode block
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", "", fmt.Errorf("failed to create GCM mode: %w", err)
    }

    // Generate a 96-bit nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", "", fmt.Errorf("failed to generate nonce: %w", err)
    }

    // Encrypt the plaintext
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

    // Encode the ciphertext to base64
    encodedCiphertext := base64.StdEncoding.EncodeToString(ciphertext)

    // Metadata header for encryption version
    metadataHeader := "X-OKF-Encryption-Version: v2"

    return encodedCiphertext, metadataHeader, nil
}
```

### Run 2: OKF Progressive Disclosure

```go
package security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/hex"
    "io"
    "net/http"
)

// EncryptPayload encrypts the given plaintext using AES-256-GCM with a 96-bit nonce.
// It returns the encrypted payload and an error if encryption fails.
func EncryptPayload(plaintext []byte, key []byte) ([]byte, http.Header, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create AES cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
    }
    nonce := make([]byte, gcm.NonceSize()) // 12 bytes
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
    }
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

    // Prepare the HTTP header
    headers := http.Header{}
    headers.Add("X-OKF-Encryption-Version", "v2")

    return ciphertext, headers, nil
}

// Example usage:
/*
func main() {
    key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes key
    plaintext := []byte("Sensitive customer data")

    ciphertext, headers, err := EncryptPayload(plaintext, key)
    if err != nil {
        fmt.Println("Encryption failed:", err)
        return
    }

    fmt.Println("Ciphertext:", hex.EncodeToString(ciphertext))
    for k, v := range headers {
        fmt.Println(k, ":", v[0])
    }
}
*/
```
