# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `prism-ml/bonsai-27b`
* **Execution Mode**: `Local On-Device Inference`
* **Host Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Temperature**: `0.10`
* **Timeout**: `3m0s`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 20:36:17

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3058` tokens | `627` tokens | **-79.5%** |
| **Output Tokens (Generated)** | `1060` tokens | `3170` tokens | - |
| **Prefill Latency (TTFT)** | `33290.0 ms` | `7012.8 ms` | **4.7x faster** |
| **Total Turn Time** | `90.64 s` | `178.47 s` | - |
| **Policy Compliance** | `4/4 checks` | `4/4 checks` | 100% Consistent |

### Policy Checks:
* **AES-256-GCM**: ✅ PASS
* **96-bit / 12-byte Nonce**: ✅ PASS
* **X-OKF-Encryption-Version Header**: ✅ PASS
* **No ECB/CBC**: ✅ PASS

---

## 📝 Generated Code Responses

### Run 1: Monolith Context Dump

<details>
<summary>💭 Thought Process (226 tokens)</summary>

The user wants me to implement a Go function for encrypting sensitive customer payloads according to the company's encryption policy documented in `security/encryption-policy.md`.

Key requirements from the policy:
1. Algorithm: AES-256-GCM (`crypto/cipher.NewGCM`)
2. Nonce: Exactly 96-bit (12 bytes) cryptographically secure random nonce via `crypto/rand.Read`
3. Storage Format: Nonce prepended to ciphertext: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`
4. Header Convention: `X-OKF-Encryption-Version: v2`
5. Key Derivation: Master keys from environment variable `ENCRYPTION_MASTER_KEY_V2` (32 bytes)
6. Forbidden: AES-ECB, AES-CBC without HMAC, raw RSA

The policy already provides a Go implementation example. I need to provide the complete implementation including:
- The encryption function
- A decryption function (for completeness, though not explicitly requested)
- Proper error handling
- The metadata header

Let me write the complete Go code.
</details>

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// EncryptionVersion is the mandatory header value for all encrypted payloads.
const EncryptionVersion = "v2"

// EncryptPayload encrypts sensitive customer data using AES-256-GCM with a
// cryptographically secure 96-bit nonce prepended to the ciphertext.
//
// Storage format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
//
// Returns the encrypted blob ready for storage. The caller must also set
// the HTTP/Metadata header: X-OKF-Encryption-Version: v2
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (256 bits), got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate secure nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return append(nonce, ciphertext...), nil
}

// DecryptPayload decrypts an AES-256-GCM encrypted blob. The first 12 bytes
// are extracted as the nonce before decryption.
func DecryptPayload(ciphertext []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (256 bits), got %d", len(key))
	}

	if len(ciphertext) < 12+16 {
		return nil, fmt.Errorf("encrypted payload too short: expected at least 28 bytes (12 nonce + 16 tag), got %d", len(ciphertext))
	}

	nonce := ciphertext[:12]
	gcmCiphertext := ciphertext[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	plaintext, err := gcm.Open(nonce, nonce, gcmCiphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (invalid key or tampered payload): %w", err)
	}

	return plaintext, nil
}

// GetEncryptionMasterKey retrieves the 32-byte master key from the
// ENCRYPTION_MASTER_KEY_V2 environment variable.
func GetEncryptionMasterKey() ([]byte, error) {
	key := os.Getenv("ENCRYPTION_MASTER_KEY_V2")
	if key == "" {
		return nil, fmt.Errorf("ENCRYPTION_MASTER_KEY_V2 environment variable is not set")
	}

	keyBytes := []byte(key)
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_MASTER_KEY_V2 must be exactly 32 bytes (256 bits), got %d", len(keyBytes))
	}

	return keyBytes, nil
}
```

### Run 2: OKF Progressive Disclosure

<details>
<summary>💭 Thought Process (2732 tokens)</summary>

The user wants a Go function to encrypt sensitive customer payloads based on the provided architectural decision document.

Key requirements from the ADR:
1. Algorithm: AES-256-GCM (`crypto/cipher.NewGCM`)
2. Nonce: 96-bit (12 bytes) cryptographically secure random nonce via `crypto/rand.Read`
3. Storage Format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`
4. Header Convention: `X-OKF-Encryption-Version: v2` (though this is for HTTP/Metadata, I should probably include it in the response or as a comment/struct field if relevant, but the function itself just returns bytes. I'll stick to returning the encrypted bytes as specified in the format, but maybe add a struct or comment about the header if needed. The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll include a struct that holds both the encrypted data and the header, or just return the bytes as per the format. Actually, the ADR says "All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header". I'll create a struct `EncryptedPayload` that contains the data and the version header, or just return the bytes. Let's stick to returning the bytes as per the format `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`, but I'll add a comment about the header. Wait, the prompt says "Return the complete Go code with any required metadata headers or nonces." I'll make a function that returns the encrypted bytes, and maybe a helper to format it with headers if needed. Actually, I'll just implement the encryption function as requested, and include the header in a struct or comment. Let's keep it simple: return `[]byte` as per the format, but I'll add a struct `EncryptedEnvelope` that includes the header and data to be safe. Or just follow the exact format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`. I'll stick to returning `[]byte` and mention the header in comments, or return a struct. I'll go with returning `[]byte` as per the ADR's storage format, but I'll add a comment about the header. Actually, the prompt says "with any required metadata headers or nonces". I'll include a struct `EncryptedPayload` that holds the data and the version header.

Wait, the ADR says: "Storage Format: Nonce must be prepended to the ciphertext: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`."
And: "Header Convention: All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header: `X-OKF-Encryption-Version: v2`."

I will implement a function that returns the encrypted bytes in the specified format. I'll also add a struct for the envelope if needed, but the function signature in the example is `func EncryptPayload(plaintext []byte, key []byte) ([]byte, error)`. I'll stick to that but ensure it matches the ADR exactly.

Let's refine:
- Use `crypto/aes`, `crypto/cipher`, `crypto/rand`, `io`
- Key must be 32 bytes. I'll add a check or assume it's provided correctly per ADR rule 6.
- Nonce: 12 bytes from `crypto/rand.Read`
- Seal: `gcm.Seal(nonce, nonce, plaintext, nil)`
- Return: `nonce` concatenated with `ciphertext` (which already includes the 16-byte tag)
- Add header handling: I'll create a struct `EncryptedEnvelope` that contains the data and the version header, or just return bytes. The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll include a struct `EncryptedPayload` that holds the encrypted data and the header string.

Actually, I'll just implement exactly what's asked: a function that encrypts and returns the bytes in the specified format. I'll add comments about the header. To be thorough, I'll define a struct `EncryptedEnvelope` that includes the data and the header, as it's common in Go to bundle metadata.

Wait, the ADR example shows:
```go
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) { ... }
```
I'll follow that signature but ensure it matches the ADR. I'll also add a helper to format with headers if needed, but keep it concise.

Let's write the code:
```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const (
	EncryptionVersionHeader = "X-OKF-Encryption-Version: v2"
)

// EncryptedEnvelope holds the encrypted payload and its required metadata header.
type EncryptedEnvelope struct {
	Data []byte
	Header string
}

// EncryptPayload encrypts sensitive customer data using AES-256-GCM.
// It returns the encrypted bytes in the format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag].
func EncryptPayload(plaintext []byte, key []byte) (*EncryptedEnvelope, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (256 bits)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate cryptographically secure nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	
	// Format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	encryptedData := append(nonce, ciphertext...)

	return &EncryptedEnvelope{
		Data:   encryptedData,
		Header: EncryptionVersionHeader,
	}, nil
}
```

Wait, the ADR says "Return the complete Go code with any required metadata headers or nonces." The example in the ADR returns `[]byte, error`. I'll stick to returning `[]byte` but maybe add the header as a comment or separate function. Actually, it's safer to just return `[]byte` as per the ADR example, and mention the header in comments. But the prompt says "with any required metadata headers or nonces". I'll include a struct to bundle them, as it's more idiomatic for "envelopes". Or I'll just return `[]byte` and add a comment about the header. I think returning `[]byte` is fine, but I'll make sure it's clear.

Actually, the ADR says: "All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header: `X-OKF-Encryption-Version: v2`."
I'll create a function that returns the bytes, and another helper to format it with headers if needed. But to keep it concise, I'll just return the bytes and add a comment. Or I'll follow the exact signature from the ADR example but ensure it matches all rules.

Let's stick to the ADR example signature: `func EncryptPayload(plaintext []byte, key []byte) ([]byte, error)`
I'll add the header as a constant and maybe return it separately or just document it. The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll include a struct `EncryptedPayload` that holds both, as it's cleaner.

Actually, I'll just output the code exactly matching the ADR's example but enhanced with proper error handling and comments about the header. I'll also add a helper to format the envelope if needed, but keep it simple.

Wait, the prompt says "Keep your internal thinking concise and directly output the complete Go code implementation."
I will output just the Go code.

Let's refine the code to be production-ready and strictly follow the ADR:
- AES-256-GCM
- 96-bit nonce via `crypto/rand.Read`
- Format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`
- Header: `X-OKF-Encryption-Version: v2`
- Key from env var mentioned in ADR, but function takes key as param. I'll add a comment about `ENCRYPTION_MASTER_KEY_V2`.

I'll write it out carefully.
One thing: `gcm.Seal` returns ciphertext + tag. So `append(nonce, ciphertext...)` is correct.
I'll add a struct for the envelope to satisfy "metadata headers".

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const EncryptionVersionHeader = "X-OKF-Encryption-Version: v2"

// EncryptedEnvelope represents a securely encrypted customer payload with required metadata.
type EncryptedEnvelope struct {
	Data   []byte // Format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	Header string // HTTP/Metadata header for versioning
}

// EncryptPayload encrypts sensitive customer data using AES-256-GCM.
// Key must be 32 bytes (sourced from ENCRYPTION_MASTER_KEY_V2 in production).
func EncryptPayload(plaintext []byte, key []byte) (*EncryptedEnvelope, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (256 bits)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes (96-bit)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate cryptographically secure nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	
	// Storage Format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	encryptedData := append(nonce, ciphertext...)

	return &EncryptedEnvelope{
		Data:   encryptedData,
		Header: EncryptionVersionHeader,
	}, nil
}
```

This matches all requirements. It's concise, follows the ADR strictly, handles the nonce correctly, uses AES-256-GCM, returns the correct format, and includes the header as requested. I'll output this directly.
One minor point: The ADR example returns `[]byte, error`. I changed it to return a struct. That's fine and actually better for "metadata headers". I'll stick with it. If they strictly want `[]byte`, I can adjust, but the prompt says "with any required metadata headers or nonces", so a struct is appropriate. I'll keep it.
All good. Output matches request.✅
Proceeds. 
Self-Correction/Verification during thought:
- Check nonce size: `gcm.NonceSize()` is 12 for AES-GCM. Correct.
- Check `io.ReadFull`: correct for cryptographically secure random bytes.
- Check format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`. `gcm.Seal` returns ciphertext+tag. `append(nonce, ciphertext...)` matches. Correct.
- Check header: `X-OKF-Encryption-Version: v2`. Included. Correct.
- Check key length: 32 bytes. Checked. Correct.
- Forbidden algorithms: Not used. Correct.
- Code is complete and ready.
Output matches exactly.✅
I will generate the response now. 
Note: I'll make sure the code block is properly formatted and contains only the Go code as requested. I'll add minimal comments to explain adherence to the ADR. All constraints met. 
Proceeds. 
[Output Generation] -> *Self-Correction/Refinement during generation prep*: 
The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll stick to the struct approach as it cleanly bundles data and header. It's idiomatic Go. I'll output exactly that. No extra fluff.
Done. 
Output matches the refined code.✅
</details>

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const EncryptionVersionHeader = "X-OKF-Encryption-Version: v2"

// EncryptedEnvelope holds the encrypted payload and its required metadata header.
type EncryptedEnvelope struct {
	Data   []byte // Format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	Header string // HTTP/Metadata header for versioning compliance
}

// EncryptPayload encrypts sensitive customer data using AES-256-GCM.
// Key must be exactly 32 bytes (sourced from ENCRYPTION_MASTER_KEY_V2 in production).
func EncryptPayload(plaintext []byte, key []byte) (*EncryptedEnvelope, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes (256 bits)")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GCM cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes (96-bit)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate cryptographically secure nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	
	// Storage Format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	encryptedData := append(nonce, ciphertext...)

	return &EncryptedEnvelope{
		Data:   encryptedData,
		Header: EncryptionVersionHeader,
	}, nil
}
```
