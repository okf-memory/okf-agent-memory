# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `OPENAI`
* **Model Tested**: `gpt-5.6-sol`
* **Execution Mode**: `Remote Cloud API (https://api.openai.com/v1)`
* **Benchmark Runner Client**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Temperature**: `0.10`
* **Endpoint**: `https://api.openai.com/v1`
* **Date**: 2026-09-05 19:10:25

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3034` tokens | `603` tokens | **-80.1%** |
| **Output Tokens (Generated)** | `887` tokens | `816` tokens | - |
| **Prefill Latency (TTFT)** | `9226.8 ms` | `8682.0 ms` | **1.1x faster** |
| **Total Turn Time** | `15.90 s` | `14.93 s` | - |
| **Policy Compliance** | `4/4` passed | `4/4` passed | 100% Consistent |

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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	encryptionKeyEnvironmentVariable = "ENCRYPTION_MASTER_KEY_V2"
	encryptionVersionHeader           = "X-OKF-Encryption-Version"
	encryptionVersion                 = "v2"

	aes256KeySize = 32
	gcmNonceSize  = 12
)

// EncryptedPayload contains the binary storage envelope and its required
// encryption metadata.
//
// Ciphertext has the following binary format:
//
//	[12-byte random nonce][ciphertext][16-byte GCM authentication tag]
type EncryptedPayload struct {
	Ciphertext []byte
	Headers    http.Header
}

// EncryptCustomerPayload encrypts sensitive customer data using AES-256-GCM.
//
// ENCRYPTION_MASTER_KEY_V2 must contain the base64 encoding of exactly 32
// cryptographically random bytes. Ciphertext is returned as raw binary data
// suitable for storage in a PostgreSQL bytea column or equivalent binary store.
func EncryptCustomerPayload(plaintext []byte) (*EncryptedPayload, error) {
	key, err := loadEncryptionKey()
	if err != nil {
		return nil, err
	}
	defer clear(key)

	ciphertext, err := encryptAES256GCM(plaintext, key)
	if err != nil {
		return nil, fmt.Errorf("encrypt customer payload: %w", err)
	}

	headers := make(http.Header)
	headers.Set(encryptionVersionHeader, encryptionVersion)

	return &EncryptedPayload{
		Ciphertext: ciphertext,
		Headers:    headers,
	}, nil
}

func loadEncryptionKey() ([]byte, error) {
	encodedKey, ok := os.LookupEnv(encryptionKeyEnvironmentVariable)
	if !ok || encodedKey == "" {
		return nil, fmt.Errorf(
			"required environment variable %s is not configured",
			encryptionKeyEnvironmentVariable,
		)
	}

	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf(
			"%s must be standard base64: %w",
			encryptionKeyEnvironmentVariable,
			err,
		)
	}

	if len(key) != aes256KeySize {
		clear(key)
		return nil, fmt.Errorf(
			"%s must decode to exactly %d bytes",
			encryptionKeyEnvironmentVariable,
			aes256KeySize,
		)
	}

	return key, nil
}

func encryptAES256GCM(plaintext, key []byte) ([]byte, error) {
	if len(key) != aes256KeySize {
		return nil, errors.New("AES-256 requires a 32-byte key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}
	if gcm.NonceSize() != gcmNonceSize {
		return nil, fmt.Errorf(
			"unexpected GCM nonce size: got %d, want %d",
			gcm.NonceSize(),
			gcmNonceSize,
		)
	}

	// Prefix the ciphertext with its independently generated 96-bit nonce.
	envelope := make([]byte, gcmNonceSize, gcmNonceSize+len(plaintext)+gcm.Overhead())
	nonce := envelope[:gcmNonceSize]

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate encryption nonce: %w", err)
	}

	// Seal appends [ciphertext][16-byte authentication tag] after the nonce.
	envelope = gcm.Seal(envelope, nonce, plaintext, nil)
	return envelope, nil
}
```

`ENCRYPTION_MASTER_KEY_V2` should be configured as the standard Base64 encoding of a cryptographically random 32-byte key. The returned `Ciphertext` already includes the required 12-byte nonce prefix, and `Headers` contains `X-OKF-Encryption-Version: v2`.

### Run 2: OKF Progressive Disclosure

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
)

const (
	masterKeyEnvironmentVariable = "ENCRYPTION_MASTER_KEY_V2"
	encryptionVersionHeader       = "X-OKF-Encryption-Version"
	encryptionVersion             = "v2"

	aes256KeySize = 32
	gcmNonceSize  = 12
	gcmTagSize    = 16
)

// EncryptedEnvelope contains the encrypted storage payload and its mandatory
// metadata. Payload is formatted as:
//
//	[12-byte random nonce][ciphertext][16-byte GCM authentication tag]
type EncryptedEnvelope struct {
	Payload  []byte
	Metadata map[string]string
}

// EncryptSensitiveCustomerPayload encrypts plaintext using AES-256-GCM.
//
// The 32-byte master key is read from ENCRYPTION_MASTER_KEY_V2. The environment
// variable must contain exactly 32 raw bytes. Neither plaintext nor key
// material should be logged by callers.
func EncryptSensitiveCustomerPayload(plaintext []byte) (*EncryptedEnvelope, error) {
	key, err := loadMasterKey()
	if err != nil {
		return nil, err
	}

	// Clear this mutable copy after use.
	defer clear(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize AES-256 cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize AES-GCM: %w", err)
	}
	if gcm.NonceSize() != gcmNonceSize {
		return nil, fmt.Errorf(
			"unexpected AES-GCM nonce size: got %d, want %d",
			gcm.NonceSize(),
			gcmNonceSize,
		)
	}
	if gcm.Overhead() != gcmTagSize {
		return nil, fmt.Errorf(
			"unexpected AES-GCM tag size: got %d, want %d",
			gcm.Overhead(),
			gcmTagSize,
		)
	}

	nonce := make([]byte, gcmNonceSize)
	n, err := rand.Read(nonce)
	if err != nil {
		return nil, fmt.Errorf("generate AES-GCM nonce: %w", err)
	}
	if n != gcmNonceSize {
		return nil, fmt.Errorf(
			"generate AES-GCM nonce: read %d bytes, want %d",
			n,
			gcmNonceSize,
		)
	}

	// Passing nonce as dst prepends it to the authenticated ciphertext and tag.
	// Result: [12-byte nonce][ciphertext][16-byte GCM tag].
	payload := gcm.Seal(nonce, nonce, plaintext, nil)

	return &EncryptedEnvelope{
		Payload: payload,
		Metadata: map[string]string{
			encryptionVersionHeader: encryptionVersion,
		},
	}, nil
}

func loadMasterKey() ([]byte, error) {
	value, present := os.LookupEnv(masterKeyEnvironmentVariable)
	if !present {
		return nil, fmt.Errorf(
			"required environment variable %s is not set",
			masterKeyEnvironmentVariable,
		)
	}

	if len(value) != aes256KeySize {
		return nil, fmt.Errorf(
			"%s must contain exactly %d bytes; got %d",
			masterKeyEnvironmentVariable,
			aes256KeySize,
			len(value),
		)
	}

	key := []byte(value)
	if len(key) != aes256KeySize {
		return nil, errors.New("invalid AES-256 master key length")
	}

	return key, nil
}
```

Persist `EncryptedEnvelope.Payload` as the encrypted value and `EncryptedEnvelope.Metadata` alongside it. For HTTP exchange, copy the metadata entry to the response or request header.
