# Security, Privacy & Data Governance Guidelines

This document establishes the normative security, privacy, and data governance policies for **OKF Agent Memory** implementations.

---

## 1. Core Threat Model & Trust Boundaries

AI agents operating in software repositories, customer coaching platforms, or private workspaces interact with sensitive data. Persistent memory creates a durable record; therefore, strict boundaries must prevent data leaks, credential persistence, and prompt injection attacks.

```
┌────────────────────────────────────────────────────────┐
│                   Untrusted Input                      │
│      (Chat Prompts, Web Scrapes, Raw Stack Traces)     │
└───────────────────────────┬────────────────────────────┘
                            │ Filter & Sanitize
                            ▼
┌────────────────────────────────────────────────────────┐
│                   Agent Reasoning                      │
│           (Transient Context / Scratchpads)            │
└───────────────────────────┬────────────────────────────┘
                            │ Explicit Persistence Gate
                            ▼
┌────────────────────────────────────────────────────────┐
│               Persistent Memory (OKF)                  │
│       (Plaintext Markdown / Git-Audited Corpus)        │
└────────────────────────────────────────────────────────┘
```

---

## 2. What Must NEVER Be Persisted

Agents and tools must enforce an absolute prohibition against persisting the following categories:

| Prohibited Category | Examples | Correct Handling |
| :--- | :--- | :--- |
| **Secrets & Credentials** | API keys, OAuth tokens, private keys (`.pem`), AWS/GCP secrets, database passwords. | Keep in environment variables or secret vaults. Never write to `knowledge/`. |
| **Personally Identifiable Info (PII)** | Social security numbers, credit card details, national IDs, passwords. | Redact or pseudonymize before recording facts. |
| **Transient Session Noise** | Raw chain-of-thought, intermediate tool call logs, speculative agent chatter. | Allow conversation context to expire naturally; do not persist. |
| **Raw Stack Traces & Large Dumps** | Multi-megabyte memory dumps, unparsed server log outputs. | Distill into a high-level root-cause finding concept. |

---

## 3. Provenance & Trust Tiers

OKF v0.2 enforces explicit provenance separation:

### 1. `generated` (Agent Attribution)
When an agent creates or modifies a concept, it must record its identity:
```yaml
generated:
  by: agent/claude-3-7-sonnet
  at: 2026-08-28T10:00:00Z
```

### 2. `verified` (Human Review)
* An AI agent **MUST NEVER** generate a `verified` field asserting human verification.
* Only a verified human review or signed process may record:
```yaml
verified:
  - by: human/skn
    at: 2026-08-28T10:30:00Z
```
* The Go validator (`okf validate --strict`) flags missing provenance or invalid actor schemas.

---

## 4. Privacy & Domain Boundaries

### Software Engineering
* Code architectures, database schemas, API contracts, and decision records are safe for team git repositories.
* Do not commit local developer credentials, private URLs, or production customer payloads into `knowledge/`.

### Coaching & Medical Notes
* In sensitive human domains (e.g. `examples/coaching`), use **pseudonymous client IDs** (e.g. `client-c42`) instead of full legal names.
* Maintain an out-of-band, encrypted client key mapping if necessary.
* Never store personal notes in public Git repositories without local encryption or `.gitignore` shielding.

---

## 5. Defense Against Memory Poisoning (Indirect Prompt Injection)

Malicious third-party data (e.g., untrusted web content or pull requests) could attempt to inject instructions into `knowledge/` concepts to manipulate future agent sessions.

### Protection Rules:
1. **Quarantine External Summaries**: When summarizing external web pages or unvetted documentation, clearly label sources in the frontmatter (`sources:`).
2. **Strict File Scoping**: The Go CLI only accesses paths within the specified bundle directory; it refuses to traverse path escapes (`../..`) outside the designated root.
3. **Audit Trail**: Every change is reflected in `log.md` and standard `git diff`. Changes should be reviewed during standard pull request code reviews.

---

## 6. Data Deletion & Right to Be Forgotten

Because OKF Agent Memory is 100% Git-native:

1. **Concept Archival**: Concepts can have `status: deprecated` or be moved to an archive directory.
2. **Permanent Deletion**: Delete the concept file, remove references from `index.md`, and record the deletion in `log.md`.
3. **Git History Scrubbing**: If sensitive data is accidentally committed, use standard git history rewriting tools (`git filter-repo` or BFG Repo-Cleaner) to purge the commit history.
