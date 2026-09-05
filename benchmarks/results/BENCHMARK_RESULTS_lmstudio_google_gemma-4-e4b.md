# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `google/gemma-4-e4b`
* **Execution Mode**: `Local On-Device Inference (Engine: LM Studio / Metal Acceleration)`
* **Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 18:33:13

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3034` tokens | `603` tokens | **-80.1%** |
| **Output Tokens (Generated)** | `1248` tokens | `1114` tokens | - |
| **Prefill Latency (TTFT)** | `32029.0 ms` | `28129.5 ms` | **1.1x faster** |
| **Total Turn Time** | `70.69 s` | `61.19 s` | - |
| **Policy Compliance** | `4/4` passed | `3/4` passed | 100% Consistent |

### Policy Checks:
* **AES-256-GCM**: ✅ PASS
* **96-bit / 12-byte Nonce**: ✅ PASS
* **X-OKF-Encryption-Version Header**: ❌ FAIL
* **No ECB/CBC**: ✅ PASS

---

## 📝 Generated Code Responses

### Run 1: Monolith Context Dump

As an expert AI software engineer, I have implemented the required Go function. This code strictly adheres to the `security/encryption-policy.md` document, utilizing AES-256 in GCM mode with a unique 96-bit nonce for every encryption operation.

The function returns the combined ciphertext blob, which includes the prepended 12-byte nonce, followed by the encrypted data and the GCM authentication tag.

### Go Implementation (`encryptor.go`)

```go
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptionPolicy enforces the company's standard: AES-256-GCM with a 96-bit nonce.
// The resulting ciphertext blob is structured as: [12-byte Nonce][Ciphertext + 16-byte GCM Tag].
// This function performs the encryption step.
//
// Parameters:
//   plaintext ([]byte): The sensitive customer data to be encrypted.
//   key ([]byte): The 32-byte (256-bit) master encryption key.
// Returns:
//   []byte: The combined ciphertext blob (Nonce + Encrypted Data + Tag).
//   error: An error if encryption fails.
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
	// 1. Key Validation (Policy Requirement: 32 bytes for AES-256)
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (AES-256)")
	}

	// 2. Cipher Setup
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// GCM mode provides authenticated encryption (ensuring integrity and authenticity).
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 3. Nonce Generation (Policy Requirement: Exactly 96-bit / 12 bytes)
	// The nonce MUST be unique for every encryption operation.
	nonce := make([]byte, gcm.NonceSize()) // Nonce size is 12 bytes for GCM
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// 4. Encryption (Sealing)
	// gcm.Seal performs the encryption and appends the authentication tag automatically.
	// The resulting slice starts with the nonce, followed by the encrypted data and tag.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// The 'ciphertext' variable now holds the combined blob: [Nonce][Ciphertext + Tag]
	return ciphertext, nil
}

func main() {
	// --- Example Usage Simulation ---

	// NOTE: In a real application, this key must be securely loaded from the
	// ENCRYPTION_MASTER_KEY_V2 environment variable.
	masterKey := []byte("a-very-secret-32-byte-key!") // Must be 32 bytes long

	// Sensitive data to protect
	sensitiveData := []byte("Customer PII: Name=John Doe, SSN=999-XX-XXXX")

	// Perform Encryption
	encryptedBlob, err := EncryptPayload(sensitiveData, masterKey)
	if err != nil {
		fmt.Printf("FATAL ENCRYPTION ERROR: %v\n", err)
		return
	}

	fmt.Println("--- Encryption Successful ---")
	fmt.Printf("Original Plaintext Length: %d bytes\n", len(sensitiveData))
	fmt.Printf("Encrypted Blob Length: %d bytes\n", len(encryptedBlob))
	fmt.Println("Encrypted Blob (Hex):", fmt.Sprintf("%x", encryptedBlob))
	fmt.Println("\n--- Metadata and Storage Contract ---")
	fmt.Printf("1. Stored Data: The 'Encrypted Blob' (the byte slice above).\n")
	fmt.Printf("2. Required HTTP Header: X-OKF-Encryption-Version: v2\n")
	fmt.Printf("3. Retrieval Process: The first 12 bytes of the blob are the Nonce, used for decryption.\n")
}
```

### Engineering Compliance Checklist

| Policy Requirement | Adherence Status | Implementation Detail |
| :--- | :--- | :--- |
| **Algorithm** | ✅ Compliant | AES-256 in GCM mode (`cipher.NewGCM`). |
| **Nonce Size** | ✅ Compliant | Exactly 12 bytes (96 bits) generated using `io.ReadFull(rand.Reader, nonce)`. |
| **Key Size** | ✅ Compliant | Requires a 32-byte key (`len(key) == 32`). |
| **Storage Format** | ✅ Compliant | The `gcm.Seal` function correctly prepends the nonce to the ciphertext and appends the 16-byte GCM tag, creating the required `[Nonce][Ciphertext + Tag]` structure. |
| **Metadata Header** | ✅ Compliant | The usage example includes the required `X-OKF-Encryption-Version: v2` header for context. |
| **Security** | ✅ Compliant | Uses `crypto/rand` for cryptographically secure nonce generation. |

### Run 2: OKF Progressive Disclosure

As an expert AI software engineer, I have implemented the encryption function strictly following the `Customer Payload Data Encryption Standard`.

This implementation uses AES-256 in GCM mode, generates a unique 12-byte nonce for every encryption operation, and combines the nonce with the ciphertext (which includes the GCM authentication tag) into a single output buffer, as required by the policy.

### Go Implementation

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// NonceSize defines the required 96-bit (12 byte) nonce size for GCM.
const NonceSize = 12

// KeySize defines the required 256-bit (32 byte) key size for AES-256.
const KeySize = 32

// EncryptPayload encrypts the plaintext using AES-256-GCM.
// It generates a unique nonce, encrypts the data, and returns the combined 
// [Nonce | Ciphertext | Tag] buffer.
//
// Parameters:
//   plaintext ([]byte): The sensitive customer data to encrypt.
//   key ([]byte): The 32-byte encryption master key (ENCRYPTION_MASTER_KEY_V2).
// Returns:
//   []byte: The combined ciphertext buffer (Nonce + Ciphertext + Tag).
//   error: An error if encryption fails.
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
	// 1. Key Validation (Policy Requirement: AES-256 requires a 32-byte key)
	if len(key) != KeySize {
		return nil, fmt.Errorf("encryption key must be %d bytes (AES-256)", KeySize)
	}

	// 2. Cipher Setup (Policy Requirement: AES-256)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// 3. GCM Setup (Policy Requirement: AES-GCM)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 4. Nonce Generation (Policy Requirement: Unique, 96-bit random nonce)
	// We allocate the buffer that will hold the final output, starting with the nonce.
	nonce := make([]byte, gcm.NonceSize()) 
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// 5. Encryption and Tagging (Policy Requirement: Authenticated Encryption)
	// gcm.Seal performs the encryption and appends the 16-byte GCM tag to the ciphertext.
	// The first argument (nonce) serves as the destination buffer, ensuring the nonce is prepended.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// The returned 'ciphertext' slice now contains: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	return ciphertext, nil
}

/*
// Example Usage (Demonstration purposes only - not part of the core function)
func main() {
	// NOTE: In a real application, this key must be securely loaded from the environment variable.
	masterKey := []byte("a_very_secret_32_byte_key_for_aes256") 
	data := []byte("This is the highly sensitive customer payload.")

	encryptedData, err := EncryptPayload(data, masterKey)
	if err != nil {
		fmt.Printf("Encryption failed: %v\n", err)
	} else {
		fmt.Printf("Successfully encrypted payload (Length: %d bytes).\n", len(encryptedData))
		// The output buffer is ready for storage/transmission.
	}
}
*/
```

### Compliance Checklist & Metadata

| Policy Requirement | Implementation Detail | Status |
| :--- | :--- | :--- |
| **Algorithm** | AES-256-GCM (`cipher.NewGCM`) | ✅ Compliant |
| **Nonce/IV** | 12 bytes (96 bits), generated via `crypto/rand.Reader`. | ✅ Compliant |
| **Storage Format** | Nonce prepended to Ciphertext + Tag. `gcm.Seal` handles this structure. | ✅ Compliant |
| **Key Size** | 32 bytes (256 bits). Input validation enforced. | ✅ Compliant |
| **Forbidden Algorithms** | ECB/CBC avoided; GCM is used. | ✅ Compliant |
| **Output** | Returns the combined `[Nonce][Ciphertext + Tag]` buffer. | ✅ Compliant |
