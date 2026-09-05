# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `google/gemma-4-12b-qat`
* **Execution Mode**: `Local On-Device Inference (Engine: LM Studio / Metal Acceleration)`
* **Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 18:43:07

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3034` tokens | `603` tokens | **-80.1%** |
| **Output Tokens (Generated)** | `1217` tokens | `1281` tokens | - |
| **Prefill Latency (TTFT)** | `60254.1 ms` | `43938.2 ms` | **1.4x faster** |
| **Total Turn Time** | `120.91 s` | `105.95 s` | - |
| **Policy Compliance** | `4/4` passed | `4/4` passed | 100% Consistent |

### Policy Checks:
* **AES-256-GCM**: ✅ PASS
* **96-bit / 12-byte Nonce**: ✅ PASS
* **X-OKF-Encryption-Version Header**: ✅ PASS
* **No ECB/CBC**: ✅ PASS

---

## 📝 Generated Code Responses

### Run 1: Monolith Context Dump

To comply with the **Customer Payload Data Encryption Standard** (security/encryption-policy.md), I have implemented the encryption logic in Go. 

This implementation ensures:
1. **AES-256-GCM** authenticated encryption.
2. **96-bit (12-byte)** cryptographically secure random nonces.
3. **Prepended Nonce** storage format: `[Nonce][Ciphertext + Tag]`.
4. **Metadata Compliance**: Includes the mandatory `X-OKF-Encryption-Version: v2` header.

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	// EncryptionVersion is the mandatory header for all encrypted payloads
	EncryptionVersion = "v2"
	// MasterKeyEnvVar is the environment variable source for the 32-byte key
	MasterKeyEnvVar = "ENCRYPTION_MASTER_KEY_V2"
)

var (
	ErrInvalidKey       = errors.New("encryption_error: invalid key length (must be 32 bytes)")
	ErrEncryptionFailed = errors.New("encryption_error: failed to encrypt payload")
	ErrDecryptionFailed = errors.New("encryption_error: failed to decrypt payload")
)

// EncryptedPayload represents the standard envelope for sensitive data storage.
type EncryptedPayload struct {
	Ciphertext []byte // Contains [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	Version    string // Mandatory: X-OKF-Encryption-Version
}

// EncryptPayload encrypts sensitive customer data using AES-256-GCM.
// It follows the standard: [12-byte Nonce][Ciphertext + 16-byte GCM Tag].
func EncryptPayload(plaintext []byte, key []byte) (*EncryptedPayload, error) {
	// 1. Validate Key Length (Must be 32 bytes for AES-256)
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	// 2. Create AES Cipher Block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	// 3. Initialize GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	// 4. Generate 96-bit (12 bytes) Nonce
	// Standard GCM nonce size is 12 bytes
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("%w: nonce generation failed: %v", ErrEncryptionFailed, err)
	}

	// 5. Seal the data
	// Seal appends the ciphertext to the nonce prefix: [nonce][ciphertext]
	// This satisfies the requirement: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptedPayload{
		Ciphertext: ciphertext,
		Version:    EncryptionVersion,
	}, nil
}

// DecryptPayload decrypts the payload using the provided key.
func DecryptPayload(encryptedData []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, errors.New("decryption_error: ciphertext too short")
	}

	// Split nonce and actual ciphertext
	nonce := encryptedData[:nonceSize]
	ciphertext := encryptedData[nonceSize:]

	// Open decrypts and verifies the GCM tag
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}
```

### Engineering Notes:
- **Complexity**: $O(n)$ where $n$ is the size of the plaintext.
- **Security**: The `gcm.Seal(nonce, nonce, ...)` pattern is the standard Go idiom for prepending the nonce to the ciphertext, ensuring the output is a single contiguous byte slice ready for database storage.
- **Compliance**: This implementation strictly avoids forbidden algorithms (ECB/CBC) and enforces the 32-byte key requirement for AES-256.
- **Error Handling**: Errors are wrapped to allow for mapping to the `ERR_SECURITY_ENCRYPTION_FAILED` code required by the **RFC 7807 Problem Details** standard.

### Run 2: OKF Progressive Disclosure

To comply with the **Customer Payload Data Encryption Standard (ADR)**, I have implemented the production-ready Go code below. 

This implementation ensures that the 12-byte nonce is prepended to the ciphertext, uses the mandatory AES-256-GCM algorithm, and includes the required metadata header.

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	// EncryptionVersion is the mandatory header for all encrypted payloads
	EncryptionVersion = "v2"
	// KeyEnvVar is the environment variable source for the master key
	KeyEnvVar = "ENCRYPTION_MASTER_KEY_V2"
)

// EncryptedPayload holds the ciphertext and the required metadata header
type EncryptedPayload struct {
	Ciphertext []byte
	Header     string
}

// EncryptCustomerPayload encrypts sensitive data using AES-256-GCM.
// It follows the ADR: [12-byte Nonce][Ciphertext + 16-byte GCM Tag].
func EncryptCustomerPayload(plaintext []byte) (*EncryptedPayload, error) {
	// 1. Retrieve the master key from the environment
	key := []byte(os.Getenv(KeyEnvVar))
	if len(key) != 32 {
		return nil, errors.New("invalid encryption key: ENCRYPTION_MASTER_KEY_V2 must be 32 bytes")
	}

	// 2. Create the AES block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher block: %w", err)
	}

	// 3. Wrap in GCM (Galois/Counter Mode)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 4. Generate a 96-bit (12-byte) cryptographically secure random nonce
	// gcm.NonceSize() defaults to 12 for standard GCM
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// 5. Seal the data
	// The Seal function appends the ciphertext to the first argument (dst).
	// By passing 'nonce' as the dst, we achieve the [Nonce][Ciphertext] format.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptedPayload{
		Ciphertext: ciphertext,
		Header:     fmt.Sprintf("X-OKF-Encryption-Version: %s", EncryptionVersion),
	}, nil
}

// DecryptCustomerPayload decrypts data using the same standard.
// This is provided for completeness to ensure the prepended nonce is handled correctly.
func DecryptCustomerPayload(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Split the nonce and the actual ciphertext
	nonce := ciphertext[:nonceSize]
	actualCiphertext := ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, actualCiphertext, nil)
}
```

### Key Implementation Details:
1.  **Algorithm Compliance**: Uses `crypto/cipher.NewGCM` with `crypto/aes`, satisfying the **AES-256-GCM** requirement.
2.  **Nonce Handling**: It uses `io.ReadFull(rand.Reader, nonce)` to ensure the 12-byte nonce is cryptographically secure.
3.  **Storage Format**: The line `gcm.Seal(nonce, nonce, plaintext, nil)` is a standard Go idiom that efficiently prepends the nonce to the ciphertext in a single memory allocation, matching the ADR requirement: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`.
4.  **Security Guardrails**: 
    *   It explicitly validates that the `ENCRYPTION_MASTER_KEY_V2` is exactly 32 bytes (256 bits).
    *   It avoids all forbidden algorithms (ECB, CBC, raw RSA).
5.  **Metadata**: The `EncryptedPayload` struct ensures that the `X-OKF-Encryption-Version: v2` header is always associated with the resulting ciphertext.

### Usage Example:
```go
func main() {
    // Ensure your environment variable is set:
    // export ENCRYPTION_MASTER_KEY_V2=$(openssl rand -hex 32) 
    // (Note: The env var should be the raw bytes, not a hex string, 
    // unless you add a hex.DecodeString step).

    payload := []byte("sensitive_customer_pii_data")
    encrypted, err := security.EncryptCustomerPayload(payload)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Header: %s\n", encrypted.Header)
    fmt.Printf("Ciphertext (hex): %x\n", encrypted.Ciphertext)
}
```
