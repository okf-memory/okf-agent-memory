# OKF Agent Memory — Project Roadmap

**Project:** OKF Agent Memory  
**Current milestone:** Convention v0.1 draft  
**Foundation:** Open Knowledge Format (OKF) v0.2  
**Goal:** Build a domain-neutral persistent project-memory system that
can be used by different AI agents and humans.

---

## 1. Vision

Create a small, open and provider-independent project-memory layer
based on OKF v0.2.

The same system should work for very different projects, for example:

- software development
- coaching administration
- reading and book reviews
- research
- project management
- personal knowledge bases

The project should separate:

    OKF specification
        ↓
    Agent Memory Convention
        ↓
    Agent Skill
        ↓
    OKF CLI / library
        ↓
    Project knowledge corpus

The LLM should reason about knowledge; deterministic tooling should
handle format correctness.

---

## 2. Current Status

### Done

- [x] Identified OKF v0.2 as the format foundation.
- [x] Decided against forcing a domain-specific taxonomy.
- [x] Defined the principle "search before write".
- [x] Defined a persistent-knowledge lifecycle.
- [x] Separated agent behavior from OKF syntax.
- [x] Decided that a dedicated Go tool is a strong candidate for
      deterministic serialization and validation.
- [x] Drafted `CONVENTION.md` v0.1.
- [x] Verified every convention rule against the exact OKF v0.2
      specification and reference implementation (`OKF-COMPATIBILITY.md`).
- [x] Bootstrapped the project's own persistent knowledge base (`knowledge/`
      OKF v0.2 bundle).
- [x] Finalized the Convention v0.1 (`docs/CONVENTION.md`).
- [x] Designed and implemented the standardized agent skill (`.agents/skills/okf-memory/`).
- [x] Designed and implemented the Go OKF core library (`pkg/okf/`).
- [x] Implemented deterministic OKF v0.2 validation engine with strict & drift modes.
- [x] Implemented fast in-memory BM25 search & progressive disclosure.
- [x] Implemented memory create/update operations with automated bookkeeping.
- [x] Defined and implemented relationship handling (`okf relate`).
- [x] Built the standalone CLI tool (`cmd/okf/`) with JSON mode for agents.
- [x] Implemented embedded Model Context Protocol (MCP) server (`okf mcp`).
- [x] Created cross-domain example corpora (`examples/software`, `examples/coaching`, `examples/books`).
- [x] Tested multi-agent integration & scenario benchmarks (TC-01 through TC-07, `docs/AGENT_TESTING.md`).
- [x] Documented installation, CLI reference, security policies, and release playbook (`docs/`).

### In Progress / Next Milestones

- [ ] Community feedback and broader ecosystem adoption.
- [ ] Automated migration tools for legacy ad-hoc markdown files.
- [ ] Extended MCP client ecosystem recipes.

---

# 3. Phase 1 — Specification

## 3.1 Validate the foundation

Read and compare:

- OKF v0.2 specification
- OKF v0.2 README
- Google reference implementation
- relevant examples/tests

Questions to answer:

- Which fields are normative?
- What exactly constitutes a valid Concept?
- How are IDs represented?
- How are `sources` represented?
- How do `generated` and `verified` work?
- How should `status` and `stale_after` be interpreted?
- How are links/relationships represented?
- What are the exact rules for `index.md` and `log.md`?
- What must a compliant consumer preserve?

**Deliverable:** `OKF-COMPATIBILITY.md`

---

## 3.2 Finalize Convention v0.1

Review:

- knowledge lifecycle
- provenance
- verification
- conflict handling
- language
- stale knowledge
- relationships
- indexing
- human overrides
- failure handling

Avoid adding features that belong in the CLI or skill.

**Deliverable:** stable `CONVENTION.md`

---

# 4. Phase 2 — Agent Skill

Create a provider-neutral skill.

Suggested structure:

    skill/
    ├── SKILL.md
    ├── discovery.md
    ├── remember.md
    ├── update.md
    ├── relationships.md
    └── examples.md

The skill should teach the agent:

1. when to search;
2. when to remember;
3. when to update;
4. when not to remember;
5. how to use the OKF tool;
6. how to handle uncertainty;
7. how to perform a knowledge review.

The skill should contain only enough OKF syntax knowledge for the agent
to use the tools correctly.

It should NOT reproduce the entire OKF specification.

**Deliverable:** `SKILL.md` and supporting skill documents.

---

# 5. Phase 3 — Go Library

Build the reusable core first.

Suggested package responsibilities:

    okf/
    ├── parser
    ├── document
    ├── bundle
    ├── provenance
    ├── validation
    ├── search
    └── relationship

The library should:

- parse OKF v0.2;
- preserve unknown fields;
- create concepts;
- update concepts;
- validate documents;
- validate bundles;
- inspect indexes;
- resolve references where possible.

The library should not contain agent-specific decision logic.

**Deliverable:** reusable Go package.

---

# 6. Phase 4 — CLI

Build the CLI on top of the Go library.

Initial commands should probably be:

    okf init
    okf validate
    okf search <query>
    okf show <id>
    okf create
    okf update
    okf relate

Potential future commands:

    okf archive
    okf verify
    okf explain
    okf index
    okf doctor
    okf export

The CLI should have a machine-friendly mode.

For example:

    okf search --json
    okf create --json
    okf update --json

The LLM should be able to provide semantic information while the CLI
handles serialization.

**Deliverable:** `okf` executable.

---

# 7. Phase 5 — Agent Interface

Decide how agents access the CLI.

### Option A — Shell/CLI

Simplest and lowest overhead.

    Agent → shell → okf

### Option B — MCP

Useful where an agent environment strongly prefers MCP.

    Agent → MCP → Go library → OKF

### Option C — Both

Recommended.

The CLI remains the canonical interface and an optional MCP adapter
uses the same Go library.

Do NOT implement OKF twice.

---

# 8. Phase 6 — Examples

Create complete example projects.

## Software

Demonstrate:

- architecture decision
- API research
- implementation discovery
- historical decisions

## Coaching

Demonstrate:

- clients
- sessions
- goals
- processes
- observations
- records

Be especially careful to define privacy/security boundaries before using
real personal data.

## Books

Demonstrate:

- books
- authors
- reading history
- reviews
- topics
- relationships

Example query:

    Which books have I read about cognitive biases?

The answer should be discoverable from the OKF corpus rather than from
chat history.

**Deliverable:** `examples/`

---

# 9. Phase 7 — Agent Testing

Test with different agent environments.

At minimum:

- Claude Code
- Codex
- Gemini CLI
- one local-LLM agent

Test scenarios:

### New project

Can the agent build useful initial knowledge?

### Existing project

Can the agent discover existing knowledge before acting?

### Repeated task

Does it update instead of duplicate?

### Contradiction

Does it detect conflicting information?

### Human correction

Does it respect and preserve the correction?

### Long-running project

Does knowledge remain useful after many sessions?

### Large corpus

Does progressive discovery prevent unnecessary context loading?

---

# 10. Phase 8 — Quality Tests

We need automated tests for:

- valid OKF parsing
- invalid frontmatter
- missing required fields
- unknown fields
- provenance
- generated/verified metadata
- stale knowledge
- relationships
- index handling
- updates
- duplicate prevention
- corrupted corpus recovery

We should also create fixture corpora and run them through every CLI
operation.

---

# 11. Phase 9 — Security and Privacy

This phase is essential before recommending the system for real
coaching or personal projects.

Define:

- what should never be automatically persisted;
- handling of sensitive information;
- local-only mode;
- file permissions;
- redaction;
- provenance;
- audit history;
- deletion;
- backup behavior.

The convention should explicitly distinguish persistent project
knowledge from information that should remain transient.

---

# 12. Phase 10 — Documentation

Create:

    README.md
    GETTING_STARTED.md
    CONVENTION.md
    CLI.md
    SKILL.md
    ARCHITECTURE.md
    OKF-COMPATIBILITY.md
    CONTRIBUTING.md

The README should make the concept immediately understandable:

    "Give your AI agent a persistent, portable project memory."

---

# 13. Phase 11 — Release

Before v1.0:

- freeze the convention;
- freeze the CLI contract;
- document compatibility with OKF v0.2;
- test multiple agents;
- test non-coding use cases;
- document migration;
- publish source;
- publish releases/binaries;
- provide example projects.

---

# 14. Important Architectural Rules

These decisions should remain visible during development.

### Rule 1

Do not fork or redefine OKF.

### Rule 2

Do not make the system coding-specific.

### Rule 3

Do not force a universal domain taxonomy.

### Rule 4

Do not make the LLM responsible for syntax correctness when
deterministic tooling can do it.

### Rule 5

Do not require the LLM to read the complete OKF specification for every
operation.

### Rule 6

Do not store chain-of-thought as project memory.

### Rule 7

Do not claim persistence unless the write actually succeeded.

### Rule 8

Search before write.

### Rule 9

Prefer updating existing knowledge over duplication.

### Rule 10

Keep the core provider-independent.

---

# 15. Proposed Repository

    okf-agent-memory/
    ├── README.md
    ├── AGENTS.md
    ├── LICENSE
    │
    ├── convention/
    │   ├── CONVENTION.md
    │   └── OKF-COMPATIBILITY.md
    │
    ├── skill/
    │   ├── SKILL.md
    │   ├── discovery.md
    │   ├── remember.md
    │   ├── update.md
    │   ├── relationships.md
    │   └── examples.md
    │
    ├── cmd/
    │   └── okf/
    │
    ├── internal/
    │   ├── parser/
    │   ├── validator/
    │   ├── search/
    │   └── index/
    │
    ├── pkg/
    │   └── okf/
    │
    ├── examples/
    │   ├── software/
    │   ├── coaching/
    │   └── books/
    │
    └── testdata/

---

# 16. Definition of Done

The project is ready for a first stable release when a new agent can
enter an existing project and reliably do the following:

    1. Discover the project knowledge.
    2. Understand relevant existing concepts.
    3. Work on a task.
    4. Detect new persistent knowledge.
    5. Search for existing concepts.
    6. Create or update the correct concepts.
    7. Preserve provenance.
    8. Preserve meaningful history.
    9. Validate the resulting corpus.
    10. Continue the work in a later session without the original
        conversation.

The ultimate test is:

> Can a completely new agent continue the project effectively using the
> repository and its OKF knowledge corpus alone?

If yes, the project has achieved its primary goal.
