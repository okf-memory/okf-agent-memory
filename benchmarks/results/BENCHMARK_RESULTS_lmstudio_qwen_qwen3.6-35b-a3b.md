# Benchmark Results: Monolith vs. OKF Progressive Disclosure

* **Provider**: `LMSTUDIO`
* **Model Tested**: `qwen/qwen3.6-35b-a3b`
* **Execution Mode**: `Local On-Device Inference`
* **Host Hardware**: `Apple M2 Pro (32 GB Unified Memory, macOS)`
* **Temperature**: `0.10`
* **Timeout**: `3m0s`
* **Endpoint**: `http://localhost:1234/v1`
* **Date**: 2026-09-05 19:38:12

| Metric | Monolith Context Dump | OKF Progressive Disclosure | Delta |
| :--- | :--- | :--- | :--- |
| **Input Tokens (Prompt)** | `3058` tokens | `627` tokens | **-79.5%** |
| **Output Tokens (Generated)** | `2382` tokens | `2932` tokens | - |
| **Prefill Latency (TTFT)** | `9282.3 ms` | `2416.4 ms` | **3.8x faster** |
| **Total Turn Time** | `54.63 s` | `56.63 s` | - |
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
<summary>💭 Thought Process (1908 tokens)</summary>

Here's a thinking process:

1.  **Analyze User Request:**
   - **Goal:** Implement a Go function to encrypt sensitive customer payloads for storage.
   - **Constraints:** Follow the company's strict security and encryption policy from the provided documentation.
   - **Output:** Complete Go code with required metadata headers or nonces. Keep internal thinking concise. Directly output the code.

2.  **Review Relevant Documentation:**
   - File: `security/encryption-policy.md`
   - Key requirements:
     - Algorithm: AES-256-GCM
     - Nonce/IV: Exactly 96-bit (12 bytes) cryptographically secure random nonce via `crypto/rand.Read`
     - Storage Format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`
     - Header Convention: `X-OKF-Encryption-Version: v2` (for HTTP/Metadata headers)
     - Forbidden: AES-ECB, AES-CBC without HMAC, raw RSA
     - Key Derivation: Master keys from env var `ENCRYPTION_MASTER_KEY_V2` (32 bytes)
     - Provided Go example shows basic encryption but needs refinement to match all specs exactly.

3.  **Identify Missing/Needed Details in Provided Example:**
   - The provided example doesn't handle the 32-byte key requirement explicitly (it just takes `key []byte`).
   - It doesn't explicitly show the storage format concatenation clearly (though `gcm.Seal(nonce, nonce, plaintext, nil)` does prepend the nonce automatically, I should make it explicit or follow the exact spec).
   - Needs to handle the `X-OKF-Encryption-Version: v2` header convention (probably as a constant or metadata struct).
   - Should include proper error handling, key validation (must be 32 bytes), and maybe a decryption function for completeness (though not strictly requested, it's good practice, but I'll stick to encryption as requested).
   - I'll structure it as a package with clear constants, validation, and the exact encryption function matching the spec.

4.  **Draft Implementation (Mental Refinement):**
   ```go
   package security

   import (
       "crypto/aes"
       "crypto/cipher"
       "crypto/rand"
       "errors"
       "io"
   )

   const (
       EncryptionVersionHeader = "X-OKF-Encryption-Version"
       EncryptionVersionValue  = "v2"
       NonceSize               = 12
       KeySize                 = 32 // 256 bits
   )

   // EncryptPayload encrypts sensitive customer data using AES-256-GCM.
   // It expects a 32-byte master key and returns the encrypted payload
   // formatted as [12-byte Nonce][Ciphertext + 16-byte GCM Tag].
   func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
       if len(key) != KeySize {
           return nil, errors.New("encryption: invalid key size, expected 32 bytes")
       }

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

       // gcm.Seal automatically prepends the nonce to the ciphertext
       // Result: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
       ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
       return ciphertext, nil
   }
   ```

5.  **Cross-Check with Policy:**
   - ✅ Algorithm: AES-256-GCM (`cipher.NewGCM`)
   - ✅ Nonce: 96-bit (12 bytes) via `crypto/rand.Read` (`io.ReadFull(rand.Reader, nonce)`)
   - ✅ Storage Format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]` (handled by `gcm.Seal`)
   - ✅ Header Convention: Defined constants for `X-OKF-Encryption-Version: v2`
   - ✅ Key: 32 bytes validation
   - ✅ Matches provided example but adds validation and constants per spec.
   - ✅ No forbidden algorithms used.
   - ✅ Ready for production use.

   I will wrap it in a clean, production-ready Go package structure, add comments referencing the policy, and ensure it strictly adheres to the RFC. I'll also add a helper for the metadata header as requested ("any required metadata headers").

   One minor point: The policy says "Header Convention: All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header: `X-OKF-Encryption-Version: v2`." This implies that when storing or sending, this header/metadata should be attached. I'll provide a struct or constants to represent this metadata envelope, and show how to use it.

   Let's draft the final code. I'll keep it concise and directly output the code as requested. I'll include the encryption function, constants, and a usage example that demonstrates the header convention.

   Structure:
   - Package `security`
   - Constants for version, nonce size, key size
   - `EncryptPayload` function
   - `EncryptionMetadata` struct/type for the header convention
   - Clear comments mapping to policy points.

   All constraints met. Proceed. 
   Self-Correction/Refinement during thought: 
   The policy says "Header Convention: All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header: `X-OKF-Encryption-Version: v2`." This is likely for HTTP responses or metadata envelopes. I'll define a constant and show how to attach it. The encryption function itself just returns the bytes. I'll add a helper to wrap it with metadata if needed, but the core requirement is the encryption function. I'll stick to the exact function signature requested but add the constants and validation.

   Final code structure:
   ```go
   package security

   import (
       "crypto/aes"
       "crypto/cipher"
       "crypto/rand"
       "errors"
       "io"
   )

   // Policy Constants
   const (
       EncryptionVersionHeader = "X-OKF-Encryption-Version"
       EncryptionVersionValue  = "v2"
       RequiredKeySize         = 32 // 256 bits
       NonceSize               = 12 // 96 bits
   )

   // EncryptPayload encrypts sensitive customer payloads for storage.
   // Follows RFC 7807 error handling and company encryption policy.
   // Returns payload formatted as: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
   func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
       if len(key) != RequiredKeySize {
           return nil, errors.New("security: invalid key size, expected 32 bytes from ENCRYPTION_MASTER_KEY_V2")
       }

       block, err := aes.NewCipher(key)
       if err != nil {
           return nil, err
       }

       gcm, err := cipher.NewGCM(block)
       if err != nil {
           return nil, err
       }

       nonce := make([]byte, gcm.NonceSize())
       if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
           return nil, err
       }

       // gcm.Seal automatically prepends the nonce to the ciphertext
       // Resulting format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
       return gcm.Seal(nonce, nonce, plaintext, nil), nil
   }
   ```
   This matches all requirements. I will output it directly. I'll add a brief usage example showing the header convention attachment to fully satisfy "any required metadata headers". All good.✅
   Output matches response. 
   Proceeds. 
   [Output Generation] -> *Self-Correction/Verification during generation prep*: 
   Check policy: "Key Derivation: Master keys must be 32 bytes (256 bits) sourced from environment variable `ENCRYPTION_MASTER_KEY_V2`." -> I'll add a comment about loading it from env, but the function takes `[]byte` as per standard Go crypto practices. I'll keep it as is.
   Check policy: "Header Convention: All encrypted payload envelopes stored or exchanged must include the HTTP/Metadata header: `X-OKF-Encryption-Version: v2`." -> I'll define a constant and show how to attach it to a metadata envelope struct.
   All constraints met. Output matches.✅
   Generating. 
   [Output] -> *Proceeds*
   *(Done.)*
</details>

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// Policy Constants per encryption-policy.md
const (
	EncryptionVersionHeader = "X-OKF-Encryption-Version"
	EncryptionVersionValue  = "v2"
	RequiredKeySize         = 32 // 256 bits
	NonceSize               = 12 // 96 bits
)

// EncryptionMetadata represents the required envelope for encrypted payloads.
// All stored or exchanged payloads must include this header convention.
type EncryptionMetadata struct {
	Version string `json:"encryption_version"`
	Header  string `json:"-"` // Used for HTTP transport
}

// NewEncryptionMetadata creates a compliant metadata envelope.
func NewEncryptionMetadata() EncryptionMetadata {
	return EncryptionMetadata{
		Version: EncryptionVersionValue,
		Header:  EncryptionVersionHeader,
	}
}

// EncryptPayload encrypts sensitive customer payloads for storage.
// Enforces AES-256-GCM, 96-bit random nonces, and strict key sizing.
// Returns payload formatted exactly as: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
func EncryptPayload(plaintext []byte, key []byte) ([]byte, error) {
	if len(key) != RequiredKeySize {
		return nil, errors.New("security: invalid key size, expected 32 bytes from ENCRYPTION_MASTER_KEY_V2")
	}

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

	// gcm.Seal automatically prepends the nonce to the ciphertext
	// Resulting storage format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
```

### Run 2: OKF Progressive Disclosure

<details>
<summary>💭 Thought Process (2362 tokens)</summary>

Here's a thinking process:

1.  **Analyze User Input:**
   - **Goal:** Implement a Go function to encrypt sensitive customer payloads for storage.
   - **Constraints:** Follow the provided company security/encryption policy strictly.
   - **Output:** Complete Go code with required metadata headers or nonces. Keep internal thinking concise. Directly output the code.
   - **Policy Details (from ADR):**
     - Algorithm: AES-256-GCM (`crypto/cipher.NewGCM`)
     - Nonce: 96-bit (12 bytes) cryptographically secure random via `crypto/rand.Read`
     - Storage Format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`
     - Header: `X-OKF-Encryption-Version: v2` (metadata header)
     - Forbidden: AES-ECB, AES-CBC without HMAC, raw RSA
     - Key: 32 bytes from `ENCRYPTION_MASTER_KEY_V2` env var
     - Provided example code is a good starting point but needs to be expanded to match all requirements (header, env var handling, proper error handling, maybe a struct for the envelope).

2.  **Identify Key Requirements for Implementation:**
   - Read master key from environment variable `ENCRYPTION_MASTER_KEY_V2`
   - Validate key length (must be 32 bytes for AES-256)
   - Generate 12-byte nonce using `crypto/rand`
   - Encrypt using AES-256-GCM
   - Prepend nonce to ciphertext
   - Return/encode the result with metadata header `X-OKF-Encryption-Version: v2`
   - Provide a complete, production-ready Go function/package structure.

3.  **Draft - Mental Refinement (Go Code):**
   I need to create a package (e.g., `security`) with a function `EncryptPayload`.
   It should:
   - Get the key from env
   - Validate it
   - Create GCM
   - Generate nonce
   - Seal
   - Combine nonce + ciphertext
   - Return the combined bytes
   - Maybe return a struct that includes the header for HTTP/metadata contexts, or just return the bytes and document the header requirement. The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll make it return a struct containing the encrypted payload and the header, or just handle the header as a constant/metadata field. Let's create a clear API.

   Structure:
   ```go
   package security

   import (
       "crypto/aes"
       "crypto/cipher"
       "crypto/rand"
       "errors"
       "io"
       "os"
   )

   const EncryptionVersionHeader = "X-OKF-Encryption-Version"
   const EncryptionVersionValue = "v2"

   type EncryptedPayload struct {
       Header map[string]string
       Data   []byte
   }

   func EncryptPayload(plaintext []byte) (*EncryptedPayload, error) {
       keyStr := os.Getenv("ENCRYPTION_MASTER_KEY_V2")
       if keyStr == "" {
           return nil, errors.New("ENCRYPTION_MASTER_KEY_V2 environment variable is not set")
       }
       key := []byte(keyStr)
       if len(key) != 32 {
           return nil, errors.New("master key must be exactly 32 bytes (256 bits)")
       }

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

       // Seal appends ciphertext and tag to nonce
       ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

       return &EncryptedPayload{
           Header: map[string]string{
               EncryptionVersionHeader: EncryptionVersionValue,
           },
           Data: ciphertext,
       }, nil
   }
   ```

4.  **Check against ADR:**
   - Algorithm: AES-256-GCM? Yes.
   - Nonce: 96-bit (12 bytes) via `crypto/rand`? Yes.
   - Storage Format: `[12-byte Nonce][Ciphertext + 16-byte GCM Tag]`? Yes, `gcm.Seal(nonce, nonce, plaintext, nil)` does exactly this.
   - Header: `X-OKF-Encryption-Version: v2`? Yes, included in metadata.
   - Key: 32 bytes from env? Yes.
   - Forbidden algorithms? Not used.
   - Complete Go code? Yes.

   One minor point: The ADR says "Master keys must be 32 bytes (256 bits) sourced from environment variable `ENCRYPTION_MASTER_KEY_V2`." I should handle the key extraction properly. Also, I'll make sure the code is self-contained and follows best practices (error handling, constants, clear structure).

   I'll refine it to be a single, clean function that returns the encrypted bytes and the header, or just a struct. The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll output a complete package snippet.

   Let's ensure it matches the exact requirements:
   - Uses `crypto/aes`, `crypto/cipher`, `crypto/rand`, `io`
   - Generates 12-byte nonce
   - Prepends nonce to ciphertext
   - Includes header
   - Reads key from env
   - Validates key length

   I'll write it out carefully.

   One thing: `gcm.Seal(nonce, nonce, plaintext, nil)` is the standard Go idiom for this exact format. It modifies the nonce slice to prepend the ciphertext and tag. This matches requirement 3 perfectly.

   I'll add comments referencing the ADR rules for clarity.

   Final check of the code:
   ```go
   package security

   import (
   	"crypto/aes"
   	"crypto/cipher"
   	"crypto/rand"
   	"errors"
   	"io"
   	"os"
   )

   const (
   	EncryptionVersionHeader = "X-OKF-Encryption-Version"
   	EncryptionVersionValue  = "v2"
   	NonceSize               = 12
   	KeySize                 = 32
   )

   // EncryptedPayload represents the standardized storage format for sensitive customer data.
   type EncryptedPayload struct {
   	// Metadata contains required HTTP/metadata headers per ADR
   	Metadata map[string]string
   	// Data contains the [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
   	Data []byte
   }

   // EncryptPayload encrypts sensitive customer payloads using AES-256-GCM.
   // It adheres to the Customer Payload Data Encryption Standard (ADR).
   func EncryptPayload(plaintext []byte) (*EncryptedPayload, error) {
   	// 6. Key Derivation: Source from environment variable
   	keyStr := os.Getenv("ENCRYPTION_MASTER_KEY_V2")
   	if keyStr == "" {
   		return nil, errors.New("ENCRYPTION_MASTER_KEY_V2 environment variable is not set")
   	}
   	key := []byte(keyStr)
   	if len(key) != KeySize {
   		return nil, errors.New("master key must be exactly 32 bytes (256 bits)")
   	}

   	// 1. Algorithm: AES-256-GCM
   	block, err := aes.NewCipher(key)
   	if err != nil {
   		return nil, err
   	}

   	gcm, err := cipher.NewGCM(block)
   	if err != nil {
   		return nil, err
   	}

   	// 2. Nonce: 96-bit (12 bytes) cryptographically secure random
   	nonce := make([]byte, gcm.NonceSize())
   	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
   		return nil, err
   	}

   	// 3. Storage Format: Nonce prepended to ciphertext + GCM tag
   	// gcm.Seal modifies the nonce slice to prepend the ciphertext and 16-byte tag
   	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

   	// 4. Header Convention
   	return &EncryptedPayload{
   		Metadata: map[string]string{
   			EncryptionVersionHeader: EncryptionVersionValue,
   		},
   		Data: ciphertext,
   	}, nil
   }
   ```

   This matches all constraints perfectly. It's concise, production-ready, and directly implements the ADR. I will output just the code as requested, with minimal surrounding text. The prompt says "Keep your internal thinking concise and directly output the complete Go code implementation." I'll follow that strictly.
   One minor adjustment: I'll make sure imports are correct and the code is ready to copy-paste. I'll also add a brief usage example if needed, but the prompt asks for the function. I'll stick to the function and types.

   All constraints met:
   - AES-256-GCM: Yes
   - 96-bit nonce via crypto/rand: Yes
   - Format: [12-byte Nonce][Ciphertext + 16-byte GCM Tag]: Yes
   - Header: X-OKF-Encryption-Version: v2: Yes
   - Key from env: Yes
   - Complete Go code: Yes

   Ready. Output matches response. 
   I will produce the code block directly.✅
   Self-Correction/Verification during thought:
   - Check `gcm.Seal` signature: `func (aead Cipher) Seal(dst, nonce, plaintext, additionalData []byte) []byte`
   - `gcm.Seal(nonce, nonce, plaintext, nil)` is correct. It appends ciphertext and tag to the nonce slice, returning the new slice. This exactly matches requirement 3.
   - Key validation: 32 bytes required for AES-256. Checked.
   - Header: Included in struct. Checked.
   - All good. Output matches. 
   Proceeds. 
   Output Generation. 
   [Done] 
   *Self-Correction/Refinement during generation prep*: 
   The prompt says "Return the complete Go code with any required metadata headers or nonces." I'll make sure the code is a single, complete package snippet that can be dropped into a project. I'll add a quick `DecryptPayload`? No, not requested. Just encryption. I'll stick to exactly what's asked.
   I will output the code now. 
   Structure:
   ```go
   package security
   ...
   ```
   Matches perfectly.✅
   Output matches response. 
   [Proceeds]
</details>

```go
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"
)

const (
	// EncryptionVersionHeader matches ADR requirement #4
	EncryptionVersionHeader = "X-OKF-Encryption-Version"
	EncryptionVersionValue  = "v2"
	// ADR requirement #2 & #6
	NonceSize = 12
	KeySize   = 32
)

// EncryptedPayload represents the standardized storage envelope for sensitive customer data.
type EncryptedPayload struct {
	// Metadata contains required HTTP/metadata headers per ADR
	Metadata map[string]string
	// Data contains the [12-byte Nonce][Ciphertext + 16-byte GCM Tag]
	Data []byte
}

// EncryptPayload encrypts sensitive customer payloads using AES-256-GCM.
// It strictly adheres to the Customer Payload Data Encryption Standard (ADR).
func EncryptPayload(plaintext []byte) (*EncryptedPayload, error) {
	// 6. Key Derivation: Source 32-byte master key from environment
	keyStr := os.Getenv("ENCRYPTION_MASTER_KEY_V2")
	if keyStr == "" {
		return nil, errors.New("ENCRYPTION_MASTER_KEY_V2 environment variable is not set")
	}
	key := []byte(keyStr)
	if len(key) != KeySize {
		return nil, errors.New("master key must be exactly 32 bytes (256 bits)")
	}

	// 1. Algorithm: AES-256-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 2. Nonce: 96-bit (12 bytes) cryptographically secure random
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// 3. Storage Format: Nonce prepended to ciphertext + 16-byte GCM tag
	// gcm.Seal modifies the nonce slice to prepend the ciphertext and tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// 4. Header Convention: Attach required metadata header
	return &EncryptedPayload{
		Metadata: map[string]string{
			EncryptionVersionHeader: EncryptionVersionValue,
		},
		Data: ciphertext,
	}, nil
}
```
