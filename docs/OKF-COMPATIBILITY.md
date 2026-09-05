# OKF v0.2 Compatibility Matrix & Validation Analysis

**Document Status:** Complete  
**Project:** OKF Agent Memory  
**Target Specification:** Open Knowledge Format (OKF) v0.2  
**Normative Reference:** [GoogleCloudPlatform/knowledge-catalog/okf/SPEC.md](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)

---

## 1. Executive Summary

This document evaluates the **OKF Agent Memory Convention v0.1** against the normative requirements of **OKF v0.2**.

The conclusion of this analysis is that the convention is **100% compliant with OKF v0.2**:
- It introduces **no breaking syntax alterations** or competing formats.
- It adheres strictly to the single hard requirement of OKF v0.2 (every concept must carry YAML frontmatter with a non-empty `type` field).
- It separates behavioral guidelines (agent decision-making, search-before-write, review cycles) from format representation (markdown + YAML frontmatter).
- It fully supports all v0.2 metadata families (`sources`, `generated`, `verified`, `status`, `stale_after`, and `Attested Computation`).

---

## 2. Normative OKF v0.2 Requirements vs. Agent Memory Convention

| OKF v0.2 Spec Area | Spec Section | Spec Requirement | Agent Memory Convention Stance | Compatibility Status |
| :--- | :--- | :--- | :--- | :--- |
| **Concept Identity** | § 2, § 3 | Concept ID is file path without `.md` extension. `index.md` and `log.md` are reserved. | Adopts standard concept IDs and preserves reserved file semantics (§ 4, § 5). | **Full Match** |
| **Mandatory Field** | § 4.1, § 11 | `type` is the ONLY required frontmatter field. Must be non-empty string. | Requires valid OKF v0.2 concept creation with `type` (§ 5, § 8.4). | **Full Match** |
| **Extensibility** | § 4.1, § 11 | Consumers MUST tolerate unknown keys; producers MAY add domain keys. | Mandates preserving unknown metadata fields across updates (§ 3, § 18). | **Full Match** |
| **Provenance (`sources`)** | § 5.1 | List of mappings with required `resource`, optional `id`, `title`, `author`, `last_modified`, `usage_count`. | Preserves provenance using OKF mechanisms, keyed footnotes, and URIs (§ 8.5, § 11). | **Full Match** |
| **Actor Convention** | § 7 | Identity formats: `<producer>/<version>`, `human:<id>`, `process:<id>`. | Strictly distinguishes `human:` from agent/process actors in `generated` vs `verified` (§ 11). | **Full Match** |
| **Trust Signals** | § 5.2 | `generated: { by, at }` vs `verified: [{ by, at }]`. Human verification drives trust tier. | Agents MUST NOT mark generated/inferred content as human-verified (§ 11). | **Full Match** |
| **Lifecycle** | § 5.3 | `status: draft\|stable\|deprecated`, `stale_after: YYYY-MM-DD`. | Recommends `status` and `stale_after` without premature deletion of old context (§ 14). | **Full Match** |
| **Links & Graph** | § 6 | Relative or absolute markdown links. Link meaning resides in surrounding prose. | Prefers relative links; treats relationships as part of the knowledge graph (§ 15). | **Full Match** |
| **Index & Navigation** | § 8 | Root `index.md` may declare `okf_version: "0.2"`. Sub-indexes carry no frontmatter. | Uses indexes for progressive disclosure to avoid context bloat (§ 16). | **Full Match** |
| **Change History** | § 9 | `log.md` with ISO 8601 `## YYYY-MM-DD` headings, newest first. | Adopts dated `log.md` for recording significant additions and updates (§ 4). | **Full Match** |
| **Attested Computations**| § 10 | Sanctioned executable computations that agents may invoke but not modify. | Tooling and convention layer support `Attested Computation` concepts (§ 3). | **Full Match** |

---

## 3. Detailed Field & Feature Breakdown

### 3.1 Trust Layer (`generated` vs `verified`)
OKF v0.2 computes consumer trust tiers directly from the `verified` actor list:
1. **Human-reviewed**: Contains at least one `human:<id>` verification.
2. **Machine-confirmed**: Contains at least one `process:<id>` verification.
3. **Unverified**: Lacks `verified` or contains only unverified generation.

*Convention alignment (§ 11)*: An agent running under this convention writes `generated: { by: "<agent-id>", at: "<timestamp>" }` and leaves `verified` empty unless a human or process verification actually took place.

### 3.2 Keyed Footnotes & Citations
v0.2 retired the v0.1 `# Citations` heading in favor of the `sources` frontmatter family and footnote keys `[^id]`.

*Convention alignment (§ 11, § 12)*: Sourced facts are footnoted using source IDs, while inferences are explicitly qualified in prose ("Based on A and B, the agent infers C") to avoid confusing model inference with ground truth.

### 3.3 Link Resolution & Orphan Prevention
While OKF v0.2 allows bundle-absolute links (e.g. `/tables/orders.md`), the convention and reference tooling standardizes on **relative links** (e.g. `../tables/orders.md`) to guarantee offline readability on GitHub, IDEs, and local file systems.

---

## 4. Behavioral Guarantees Added by the Convention

The convention does not restrict the OKF specification; instead, it establishes the operational contract for agents reading and mutating OKF bundles:

1. **Read Before Write (Search before create)**: Eliminates concept duplication and knowledge divergence.
2. **Preserve Uncertainty**: Prevents conversational hallucination from becoming canonical fact.
3. **No Chain-of-Thought Pollution**: Retains only durable project knowledge, discarding transient scratchpads.
4. **Knowledge Review**: Systematically checks after major tasks if decisions or discoveries occurred.
5. **Human Override Supremacy**: Explicit human edits override model inferences with preserved provenance.

---

## 5. Verification Checklist for Implementations

Any tooling (Go library, CLI, or agent skill) implementing this convention must pass:

- [x] OKF v0.2 conformance validation via zero-dependency checker (`okf-validate.mjs`).
- [x] Retention of non-standard metadata keys when parsing and writing.
- [x] Prevention of orphaned concepts (`--strict` mode).
- [x] Validation of actor string formats for `generated.by` and `verified.by`.
- [x] Accurate generation of root and sub-directory `index.md` progressive disclosure trees.
