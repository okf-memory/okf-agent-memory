# OKF Agent Memory Convention v0.1

**Status:** v0.1 Final  
**Language:** English  
**Target:** Agents and tools maintaining persistent project knowledge  
**Format foundation:** Open Knowledge Format (OKF) v0.2

---

## 1. Purpose

The OKF Agent Memory Convention defines how an AI agent should use an
Open Knowledge Format (OKF) v0.2 knowledge corpus as the persistent
memory of a project.

The convention is intentionally domain-neutral. It can be used for:

- software development
- research
- coaching administration
- reading and book histories
- project management
- personal knowledge projects
- documentation and operations
- other long-lived human or AI-assisted projects

OKF defines the representation format. This convention defines agent
behavior around that format.

The convention does **not** attempt to replace or redefine OKF v0.2.

---

## 2. Design Principles

### 2.1 Persistent knowledge survives conversations

A conversation is temporary. The knowledge corpus is persistent.

An agent MUST assume that a future agent may have no access to the
current conversation.

Information that is important for future work MUST therefore be stored
in the project knowledge corpus.

### 2.2 The corpus is the project's long-term knowledge state

The knowledge corpus represents what the project currently knows,
including relevant history.

It is not a transcript of conversations and it is not an internal
chain-of-thought store.

### 2.3 Domain neutrality

The convention MUST NOT require a fixed domain-specific taxonomy.

A software project, a coaching project and a book-history project may
use completely different concept types.

The project may define its own taxonomy.

### 2.4 Prefer existing knowledge over duplication

Before creating new knowledge, the agent MUST search the existing
corpus for relevant concepts.

If suitable knowledge already exists, the agent SHOULD update or extend
the existing concept rather than creating a duplicate.

### 2.5 Preserve uncertainty

Agents MUST NOT turn assumptions, guesses or inferences into confirmed
facts merely because they are written into the knowledge corpus.

Where the origin or certainty of information matters, provenance and
verification information SHOULD be preserved.

### 2.6 Machine-readable, human-maintainable

Knowledge SHOULD remain understandable and useful to both humans and
agents.

The corpus MUST remain valid OKF v0.2.

---

## 3. Relationship to OKF v0.2

OKF v0.2 is the normative data-format foundation.

This convention adds behavioral rules but does not redefine the OKF
document format.

In particular:

- OKF defines the document structure.
- OKF defines the required `type` field.
- OKF defines supported metadata such as `sources`, `generated`,
  `verified`, `status` and `stale_after`.
- This convention defines when agents should create, update, verify,
  relate or retire knowledge.
- Project-specific conventions may define additional fields or types
  where appropriate.

An implementation MUST remain conformant with OKF v0.2.

Unknown OKF fields MUST NOT be discarded merely because the current
agent does not understand them.

---

## 4. Knowledge Corpus

A project using this convention SHOULD contain a dedicated knowledge
bundle, conventionally located at:

    knowledge/

The bundle follows OKF v0.2.

The corpus MAY contain:

- `index.md`
- `log.md`
- concepts
- project-specific indexes
- supporting resources allowed by the OKF specification

The convention does not require a particular directory layout for
concepts.

Folders are organizational aids, not semantic requirements.

---

## 5. Concepts and Types

A concept is a persistent unit of project knowledge represented
according to OKF v0.2.

The `type` of a concept describes its semantic role.

Projects MAY define their own types.

A project may choose types such as:

- `Fact`
- `Decision`
- `Observation`
- `Research`
- `Person`
- `Book`
- `Review`
- `Client`
- `Goal`
- `Process`
- `Event`
- `Resource`
- `Task`

These are recommendations, not a closed vocabulary.

Agents MUST NOT reject an otherwise valid OKF concept merely because
its type is unfamiliar.

### 5.1 Avoid artificial fragmentation

Agents SHOULD avoid creating many tiny concepts when one coherent
concept would be more useful.

For example, a single architectural decision may contain its rationale,
alternatives and consequences when keeping those elements together
makes the knowledge easier to understand.

Conversely, information SHOULD be split into separate concepts when it
has an independent lifecycle, is frequently referenced independently,
or would otherwise become difficult to maintain.

---

## 6. What Should Be Remembered?

An agent SHOULD persist information when a future human or agent would
reasonably benefit from knowing it.

Typical examples include:

### Facts

Stable or relevant facts about the project or its domain.

### Decisions

Important choices and, where useful, their rationale and consequences.

### Requirements and constraints

Requirements, limitations, policies and project boundaries.

### Discoveries

Non-obvious findings discovered during research, implementation,
analysis or investigation.

### Research

Relevant external information together with appropriate provenance.

### Observations

Useful observations that may affect future decisions or work.

### Relationships

Important connections between concepts.

### History

Relevant events or changes whose historical context matters.

### User-provided knowledge

Information explicitly supplied by the project owner or another trusted
human source.

### Artifacts

Important documents, files, datasets, designs or other project outputs,
when their relationship to project knowledge matters.

---

## 7. What Should Not Be Remembered?

Agents SHOULD NOT store:

- ordinary conversation
- greetings or social interaction
- temporary reasoning
- internal chain-of-thought
- trivial intermediate steps
- redundant copies of existing knowledge
- transient information with no foreseeable future value
- speculative claims presented without appropriate qualification

The goal is a useful knowledge base, not maximum data retention.

---

## 8. Knowledge Lifecycle

Knowledge follows a lifecycle:

    Discover
       ↓
    Evaluate
       ↓
    Search existing corpus
       ↓
    Create or update
       ↓
    Add provenance
       ↓
    Validate
       ↓
    Revisit when needed
       ↓
    Verify / update / retire

### 8.1 Discover

During a task, the agent identifies information that may have lasting
value.

### 8.2 Evaluate

The agent asks:

> Would this be useful to a future human or agent working on this
> project?

If not, it normally does not belong in persistent memory.

### 8.3 Search

Before creating new knowledge, the agent MUST search the existing
knowledge corpus for related concepts.

### 8.4 Create or update

The agent creates a new concept only when suitable existing knowledge
does not exist.

Existing knowledge SHOULD be updated when the new information belongs
to an existing concept.

### 8.5 Provenance

Where the origin of information is material to its reliability or
future interpretation, provenance SHOULD be recorded using OKF
mechanisms.

### 8.6 Validate

After modifying the corpus, the agent SHOULD validate the resulting
bundle.

A dedicated OKF-aware tool SHOULD be preferred for validation and
serialization.

---

## 9. Knowledge Review

After every substantial task, the agent SHOULD perform a knowledge
review.

The review asks:

1. What did I learn?
2. What changed?
3. Did I make an important decision?
4. Did I discover something non-obvious?
5. Did an existing assumption become invalid?
6. Did a relationship between concepts change?
7. Did I create an artifact that future work should know about?
8. Is there information that a future agent would otherwise have to
   rediscover?

If the answer to all questions is no, no knowledge update is required.

The review MUST NOT cause artificial or low-value entries to be created.

---

## 10. Read Before Write

For any substantial task, an agent SHOULD follow this sequence:

    1. Identify the task.
    2. Discover relevant existing knowledge.
    3. Use the knowledge to inform the task.
    4. Perform the task.
    5. Review knowledge changes.
    6. Create or update knowledge.
    7. Validate the corpus.

This prevents the knowledge corpus from becoming a write-only archive.

---

## 11. Provenance and Trust

The reliability of project knowledge depends partly on knowing where it
came from.

Agents SHOULD preserve provenance when meaningful.

Potential origins include:

- human input
- project files
- source code
- external documentation
- research
- imported data
- another knowledge concept
- agent inference

The exact representation MUST use OKF v0.2 mechanisms where applicable.

An agent MUST NOT claim that a statement was human-verified when it
was only generated or inferred by an agent.

`generated` and `verified` have different meanings and MUST remain
semantically distinct.

---

## 12. Inference

Agents may derive useful conclusions from existing information.

Inferred information SHOULD be distinguishable from directly sourced
facts whenever that distinction matters.

An agent SHOULD prefer recording:

> "Based on A and B, the agent infers C."

over silently recording C as an established fact.

If an inference becomes confirmed later, the corresponding knowledge
SHOULD be updated to reflect the new evidence or verification.

---

## 13. Updating Existing Knowledge

When new information conflicts with existing knowledge, the agent MUST
NOT silently overwrite the old meaning.

The agent SHOULD:

1. identify the conflict;
2. determine whether the old information is obsolete, incorrect,
   contextual, or genuinely unresolved;
3. preserve relevant historical information;
4. update the current state;
5. record the reason for a significant change when useful.

Historical information should remain available when it has future value.

---

## 14. Staleness and Lifecycle Metadata

Knowledge may become outdated.

Agents SHOULD use the lifecycle mechanisms provided by OKF v0.2 where
appropriate, including `status` and `stale_after`.

An agent SHOULD NOT automatically delete old knowledge merely because it
is no longer current.

Instead, it should preserve useful history and clearly represent the
current state.

---

## 15. Relationships

Relationships between concepts are part of the project's knowledge.

Agents SHOULD create or preserve relationships when they materially
improve discoverability or understanding.

Examples include:

- a decision affects a component
- a book discusses a topic
- a review refers to a book
- a coaching session belongs to a client
- a requirement constrains a process
- a research result supports a decision

Relationships SHOULD be represented using the mechanisms supported by
the OKF v0.2 corpus and project conventions.

Agents SHOULD avoid creating relationships merely for completeness.

---

## 16. Indexes and Progressive Disclosure

Agents SHOULD use indexes and targeted search to navigate large knowledge corpora.

An agent MUST NOT load the entire corpus or blanket-scan directories when a smaller
set of relevant concepts is sufficient.

A preferred discovery pattern is:

    corpus search / index (lightweight metadata & 1-sentence description)
        ↓
    relevant area / candidate concept
        ↓
    targeted concept inspection (on demand)
        ↓
    referenced concepts (graph traversal only when required)

This reduces context usage while preserving access to detailed
knowledge. Bulk directory scans (`list_dir`, recursive greps across the
corpus) SHOULD be avoided in favor of indexed search tools.

---

## 17. Language

This convention is written in English so that it can be used as a
language-independent agent instruction.

The actual project knowledge MAY be written in any language appropriate
for the project and its users.

Agents MUST preserve the language of existing knowledge when updating
it unless the project explicitly specifies another language.

An agent SHOULD avoid unnecessarily translating existing knowledge.

---

## 18. Agent and Tool Responsibilities

The convention deliberately separates semantic reasoning from format
handling.

### Agent

The agent is responsible for deciding:

- whether information is worth remembering;
- what semantic type it has;
- whether existing knowledge should be updated;
- what relationships matter;
- whether provenance is important.

### Skill / Agent Instructions

A skill or agent instruction SHOULD explain:

- when to use persistent memory;
- how to perform the knowledge review;
- how to search before writing;
- which project-specific types exist;
- which tools are available.

### OKF Tool

A dedicated OKF-aware tool SHOULD be responsible for:

- creating valid OKF documents;
- updating OKF metadata;
- validating syntax;
- managing IDs;
- managing timestamps;
- preserving unknown metadata;
- searching the corpus;
- maintaining indexes where appropriate.

The tool SHOULD be the preferred mechanism for writing OKF whenever it
is available.

---

## 19. Direct File Editing

Agents MAY directly edit OKF files when no suitable OKF-aware tool is
available.

When an OKF-aware tool is available, agents SHOULD use it instead.

Direct editing MUST preserve OKF v0.2 conformance.

An agent MUST NOT invent a new OKF syntax merely because a convenient
tool is unavailable.

---

## 20. Validation

A knowledge corpus MUST remain valid according to OKF v0.2.

Projects SHOULD provide an automated validation command.

A future reference implementation may expose commands such as:

    okf validate
    okf search "query"
    okf show <concept>
    okf create
    okf update
    okf relate

The exact command-line interface is intentionally outside this
convention and will be specified separately.

---

## 21. Failure Handling

If an agent cannot write or validate the knowledge corpus, it MUST NOT
claim that the information was persisted.

It SHOULD report the failed persistence operation and, when possible,
provide enough structured information for another process to complete
the update.

Persistence claims MUST reflect actual tool results.

---

## 22. Human Override

Human instructions have priority over automatically inferred project
knowledge.

If a project owner explicitly corrects a stored fact, the agent SHOULD
update the knowledge corpus accordingly and preserve relevant
provenance.

An agent MUST NOT repeatedly restore information that a human has
explicitly rejected unless new evidence warrants reopening the issue.

---

## 23. Minimal Agent Contract

Any agent implementing this convention MUST understand the following
contract:

    1. Persistent project knowledge belongs in the OKF corpus.
    2. Search before creating.
    3. Do not blanket-scan or dump entire knowledge bundles into context.
    4. Update existing knowledge when appropriate.
    5. Do not store trivial conversation or private chain-of-thought.
    6. Preserve provenance when it matters.
    7. Do not turn inference into fact without qualification.
    8. Preserve meaningful history.
    9. Review knowledge after substantial work.
    10. Validate changes.
    11. Never claim persistence unless persistence actually succeeded.

---

## 24. Compatibility and Evolution

This convention is versioned independently from OKF.

`OKF Agent Memory Convention v0.1` is an initial behavioral convention,
not a replacement for OKF v0.2.

Future versions MAY define:

- a standardized agent skill;
- a CLI contract;
- JSON input/output schemas;
- recommended semantic types;
- relationship conventions;
- synchronization rules;
- conflict resolution rules;
- automated maintenance policies;
- multi-agent coordination.

Such additions SHOULD remain compatible with the domain-neutral nature
of OKF.

---

## 25. Open Questions for v0.2

The following topics are intentionally not finalized:

- exact CLI command structure;
- exact JSON API;
- standard relationship vocabulary;
- recommended metadata extensions;
- automated index maintenance;
- concurrency and multi-agent editing;
- merge/conflict handling;
- knowledge quality scoring;
- verification workflows;
- permissions and sensitive knowledge;
- automatic stale detection;
- import/export workflows;
- corpus migration between OKF versions.

These should be decided before declaring the convention stable.

---

## 26. Summary

The core idea is simple:

> **OKF defines how knowledge is represented.  
> The Agent Memory Convention defines when and why an agent persists
> knowledge.  
> Skills teach the agent how to use the workflow.  
> Tools guarantee correct serialization and validation.  
> The project defines what the knowledge means.**

This separation allows the same persistent-memory approach to work for
software, coaching, books, research and other domains without turning
OKF into a domain-specific system.
