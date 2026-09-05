# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `google/gemma-4-26b-a4b-qat`
* **Execution Mode**: `Local On-Device Inference (Engine: LM Studio / Metal Acceleration)`
* **Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 18:11:47

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3034` tokens | `603` tokens | **-80.1%** |
| **Output Tokens (Generated)** | `1499` tokens | `1399` tokens | - |
| **Prefill Latency (TTFT)** | `36099.3 ms` | `20886.4 ms` | **1.7x faster** |
| **Total Turn Time** | `72.47 s` | `53.54 s` | - |
| **Policy Compliance** | `4/4` passed | `4/4` passed | 100% Consistent |

### Policy Checks:
* **96-bit / 12-byte Nonce**: ✅ PASS
* **X-OKF-Encryption-Version Header**: ✅ PASS
* **No ECB/CBC**: ✅ PASS
* **AES-256-GCM**: ✅ PASS

---

## 📝 Generated Code Responses

### Run 1: Monolith Context Dump

To comply with the **Customer Payload Data Encryption Standard** (as defined in `security/encryption-policy.md`), I have implemented the encryption and decryption logic in Go. 

This implementation uses **AES-256-GCM**, enforces the **96-bit nonce** requirement, follows the mandatory **storage format** `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`, and includes the required **versioning metadata**.

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

// Constants defined by the Engineering Standards
const (
	// EncryptionVersion corresponds to the mandatory X-OKF-Encryption-Version header
	EncryptionVersion = "v2"
	// MasterKeyEnvVar is the required environment variable for the 32-byte key
	MasterKeyEnvVar = "ENCRYPTION_MASTER_KEY_V2"
	// NonceSize is exactly 12 bytes (96 bits) as per policy
	NonceSize = 12
)

var (
	ErrInvalidKeySize = errors.New("invalid key size: must be 32 bytes for AES-256")
	ErrDecryption     = errors.New("decryption failed: integrity check failed or invalid ciphertext")
	ErrKeyMissing     = errors.New("encryption master key not found in environment")
)

// GetMasterKey retrieves the 32-byte key from the environment as required by policy.
func GetMasterKey() ([]byte, error) {
	key := os.Getenv(MasterKeyEnvVar)
	if key == "" {
		return nil, ErrKeyMissing
	}
	keyBytes := []byte(key)
	if len(keyBytes) != 32 {
		return nil, ErrInvalidKeySize
	}
	return keyBytes, nil
}

// EncryptPayload encrypts sensitive data using AES-256-GCM.
// The returned byte slice follows the format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// AES-GCM implementation
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 1. Generate a 96-bit (12-byte) cryptographically secure random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate secure nonce: %w", err)
	}

	// 2. Encrypt and seal. 
	// We pass 'nonce' as the first argument (dst) so the nonce is prepended to the ciphertext.
	// Resulting format: [nonce][ciphertext][tag]
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// DecryptPayload reverses the encryption process.
// It expects the input to follow the format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
func DecryptPayload(encryptedData []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	if len(encryptedData) < NonceSize {
		return nil, ErrDecryption
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Split the nonce and the actual ciphertext
	nonce, ciphertext := encryptedData[:NonceSize], encryptedData[NonceSize:]

	// Decrypt and verify integrity
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryption
	}

	return plaintext, nil
}

// Example usage demonstrating compliance with the standard
func ExampleUsage() {
	// Setup: In production, this is set via environment variables
	os.Setenv(MasterKeyEnvVar, "a-very-secret-32-byte-key-123456")

	// 1. Retrieve Key
	key, err := GetMasterKey()
	if err != nil {
		panic(err)
	}

	// 2. Sensitive Data
	sensitiveData := []byte("customer_pii_data_0123456789")

	// 3. Encrypt
	encrypted, err := EncryptPayload(sensitiveData, key)
	if err != nil {
		panic(err)
	}

	// Note: When storing in a DB or sending via API, 
	// the caller must include the header: X-OKF-Encryption-Version: v2
	fmt.Printf("Encryption Version: %s\n", EncryptionVersion)
	fmt.Printf("Encrypted Payload (hex): %x\n", encrypted)

	// 4. Decrypt
	decrypted, err := DecryptPayload(encrypted, key)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Decrypted Data: %s\n", string(decrypted))
}
```

### Engineering Compliance Checklist:
*   **[✓] Algorithm**: Uses `crypto/cipher.NewGCM` (AES-256-GCM).
*   **[✓] Nonce**: Uses `crypto/rand` to generate exactly 12 bytes (96-bit).
*   **[✓] Storage Format**: The `gcm.Seal(nonce, nonce, ...)` pattern ensures the output is `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`.
*   **[✓] Key Management**: Key is sourced from `ENCRYPTION_MASTER_KEY_V2` and validated for 32-byte length.
*   **[✓] Metadata**: Includes `EncryptionVersion` constant (`v2`) for use in HTTP/Metadata headers.
*   **[✓] Security**: Avoids forbidden algorithms (ECB/CBC) and uses authenticated encryption to prevent tampering.

### Run 2: OKF Progressive Disclosure

To comply with the **Customer Payload Data Encryption Standard**, I have implemented the encryption logic in Go. 

This implementation follows the mandatory AES-256-GCM standard, uses a 96-bit random nonce, prepends the nonce to the ciphertext, and includes a metadata structure to ensure the `X-OKF-Encryption-Version: v2` header requirement is met during exchange.

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
	// EncryptionVersion must be used in the X-OKF-Encryption-Version header
	EncryptionVersion = "v2"
	// KeyEnvVar is the mandatory source for the 32-byte master key
	KeyEnvVar = "ENCRYPTION_MASTER_KEY_V2"
)

// EncryptedEnvelope represents the standardized storage/exchange format
// including the required metadata version.
type EncryptedEnvelope struct {
	Ciphertext []byte
	Version    string
}

// EncryptCustomerPayload encrypts sensitive data using AES-256-GCM.
// It adheres to the company standard: [12-byte Nonce][Ciphertext + 16-byte GCM Tag].
func EncryptCustomerPayload(plaintext []byte) (*EncryptedEnvelope, error) {
	// 1. Retrieve the 32-byte master key from environment
	keyStr := os.Getenv(KeyEnvVar)
	key := []byte(keyStr)
	if len(key) != 32 {
		return nil, errors.New("security violation: ENCRYPTION_MASTER_KEY_V2 must be exactly 32 bytes")
	}

	// 2. Initialize AES-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 3. Generate a 96-bit (12-byte) cryptographically secure random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 4. Seal the data. 
	// We pass 'nonce' as the first argument (dst) so the nonce is prepended to the ciphertext.
	// Result format: [12-byte Nonce][Ciphertext][16-byte Tag]
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptedEnvelope{
		Ciphertext: ciphertext,
		Version:    EncryptionVersion,
	}, nil
}

// DecryptCustomerPayload reverses the encryption process for authorized use.
func DecryptCustomerPayload(encryptedData []byte) ([]byte, error) {
	keyStr := os.Getenv(KeyEnvVar)
	key := []byte(keyStr)
	if len(key) != 32 {
		return nil, errors.New("security violation: invalid master key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Split nonce and ciphertext
	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]

	// Decrypt and verify tag
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (possible tampering): %w", err)
	}

	return plaintext, nil
}
```

### Implementation Details & Compliance Check:

1.  **Algorithm Compliance**: Uses `crypto/cipher.NewGCM` for authenticated encryption (AES-256-GCM).
2.  **Nonce Security**: Uses `crypto/rand.Read` to generate a 96-bit (12-byte) nonce as required.
3.  **Storage Format**: The `gcm.Seal(nonce, nonce, ...)` pattern ensures the nonce is prepended to the ciphertext, resulting in the required `[Nonce][Ciphertext][Tag]` structure.
4.  **Metadata/Header**: The `EncryptedEnvelope` struct provides the `Version` field, which must be mapped to the `X-OKF-Encryption-Version: v2` HTTP header when transmitting this payload via API.
5.  **Key Management**: The function strictly enforces that the key must be sourced from the `ENCRYPTION_MASTER_KEY_V2` environment variable and must be exactly 32 bytes.
6.  **Error Handling**: Errors are wrapped to provide context while ensuring that sensitive cryptographic failures (like authentication tag mismatches) are handled.

### Usage Example:

```go
func main() {
    // Setup environment for demonstration
    os.Setenv("ENCRYPTION_MASTER_KEY_V2", "a-very-secret-32-byte-key-123456")

    payload := []byte("sensitive customer PII data")

    // Encrypt
    envelope, err := EncryptCustomerPayload(payload)
    if err != nil {
        panic(err)
    }

    // When sending via HTTP, you would do:
    // req.Header.Set("X-OKF-Encryption-Version", envelope.Version)
    // req.Body = bytes.NewReader(envelope.Ciphertext)

    // Decrypt
    decrypted, err := DecryptCustomerPayload(envelope.Ciphertext)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Decrypted: %s\n", string(decrypted))
}
```
